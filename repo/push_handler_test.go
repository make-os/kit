package repo

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("PushHandler", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo types.BareRepo
	var mockMgr *mocks.MockRepoManager
	var mgr *Manager
	var handler *PushHandler
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var pubKey, pubKey2 string
	var gpgKeyID, gpgKeyID2 string
	var repoName string
	var mockMempool *mocks.MockMempool

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
	})

	BeforeEach(func() {
		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockLogic = mocks.NewMockLogic(ctrl)
		mockDHT := mocks.NewMockDHT(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mgr = NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool)

		mockMgr = mocks.NewMockRepoManager(ctrl)

		mockMgr.EXPECT().Log().Return(cfg.G().Log)
		handler = newPushHandler(repo, mockMgr)

		gpgKeyID = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey, err = crypto.GetGPGPublicKeyStr(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		Expect(err).To(BeNil())
		gpgKeyID2 = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey2, err = crypto.GetGPGPublicKeyStr(gpgKeyID2, testutil.GPGProgramPath, cfg.DataDir())
		Expect(err).To(BeNil())
		GitEnv = append(GitEnv, "GNUPGHOME="+cfg.DataDir())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".HandleStream", func() {
		When("unable to get repo old state", func() {
			BeforeEach(func() {
				mockMgr.EXPECT().GetRepoState(repo).Return(nil, fmt.Errorf("error"))
				err = handler.HandleStream(nil, nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})

		When("packfile is invalid", func() {
			BeforeEach(func() {
				oldState := &State{}
				mockMgr.EXPECT().GetRepoState(repo).Return(oldState, nil)
				err = handler.HandleStream(strings.NewReader("invalid"), nil)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to read pushed update: invalid pkt-len found"))
			})
		})
	})

	Describe(".HandleValidateAndRevert", func() {
		When("old state is not set", func() {
			BeforeEach(func() {
				_, _, err = handler.HandleValidateAndRevert()
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push-handler: expected old state to have been captured"))
			})
		})

		When("txline was not set", func() {
			var err error
			BeforeEach(func() {
				handler.rMgr = mgr

				oldState := getRepoState(repo)
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				newState := getRepoState(repo)
				var packfile io.ReadSeeker
				packfile, err = makePackfile(repo, oldState, newState)

				Expect(err).To(BeNil())
				handler.oldState = oldState
				err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
				Expect(err).To(BeNil())

				_, _, err = handler.HandleValidateAndRevert()
			})

			It("should return err='validation error.*txline was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("validation error.*txline was not set"))
			})
		})

		When("txline is set and valid", func() {
			var err error
			BeforeEach(func() {
				handler.rMgr = mgr

				oldState := getRepoState(repo)
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)

				newState := getRepoState(repo)
				var packfile io.ReadSeeker
				packfile, err = makePackfile(repo, oldState, newState)

				Expect(err).To(BeNil())
				handler.oldState = oldState

				gpgPubKeyKeeper := mocks.NewMockGPGPubKeyKeeper(ctrl)
				gpgPubKeyKeeper.EXPECT().GetGPGPubKey(pkID).Return(&types.GPGPubKey{PubKey: pubKey})
				mockLogic.EXPECT().GPGPubKeyKeeper().Return(gpgPubKeyKeeper)

				err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
				Expect(err).To(BeNil())
				_, _, err = handler.HandleValidateAndRevert()
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("with two references", func() {
			When("txline for both references are set and valid but pkIDs are different", func() {
				var err error
				BeforeEach(func() {
					handler.rMgr = mgr

					oldState := getRepoState(repo)
					pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
					pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
					txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
					appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)

					createCheckoutBranch(path, "branch2")
					pkEntity, _ = crypto.PGPEntityFromPubKey(pubKey2)
					pkID2 := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
					txLine = fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID2)
					appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID2)

					newState := getRepoState(repo)
					var packfile io.ReadSeeker
					packfile, err = makePackfile(repo, oldState, newState)

					Expect(err).To(BeNil())
					handler.oldState = oldState

					gpgPubKeyKeeper := mocks.NewMockGPGPubKeyKeeper(ctrl)
					gpgPubKeyKeeper.EXPECT().GetGPGPubKey(pkID).Return(&types.GPGPubKey{PubKey: pubKey})
					gpgPubKeyKeeper.EXPECT().GetGPGPubKey(pkID2).Return(&types.GPGPubKey{PubKey: pubKey2})
					mockLogic.EXPECT().GPGPubKeyKeeper().Return(gpgPubKeyKeeper).Times(2)

					err = handler.HandleStream(packfile, &WriteCloser{Buffer: bytes.NewBuffer(nil)})
					Expect(err).To(BeNil())
					_, _, err = handler.HandleValidateAndRevert()
				})

				It("should return err='rejected because the pushed references were signed with multiple pgp keys'", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("rejected because the pushed references were signed with multiple pgp keys"))
				})
			})
		})

	})

})
