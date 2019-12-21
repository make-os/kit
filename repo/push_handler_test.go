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
	"gopkg.in/src-d/go-git.v4/plumbing"

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
		dht := mocks.NewMockDHT(ctrl)
		mgr = NewManager(cfg, ":9000", mockLogic, dht)

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

	Describe(".removeObjRelatedToOnlyRef", func() {
		var pr *PushReader
		var err error
		var hash1, hash2 string

		When("object is related to 1 reference 'master' only and target ref is master", func() {
			BeforeEach(func() {
				pr, err = newPushReader(&WriteCloser{Buffer: bytes.NewBuffer(nil)}, repo)
				Expect(err).To(BeNil())

				hash1 = createBlob(path, "abc")
				Expect(repo.ObjectExist(hash1)).To(BeTrue())

				pr.objects = []*packObject{
					{Type: 1, Hash: plumbing.NewHash(hash1)},
				}
				pr.objectsRefs[hash1] = []string{"master"}

				mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash1).Return(false)

				errs := removeObjRelatedToOnlyRef(repo, pr, mockMgr, "master")
				Expect(errs).To(HaveLen(0))
			})

			It("should remove the reference 'master' from the object's reference list", func() {
				Expect(pr.objectsRefs[hash1]).ToNot(ContainElement("master"))
				Expect(pr.objectsRefs[hash1]).To(BeEmpty())
			})

			It("should remove the object from disk", func() {
				Expect(repo.ObjectExist(hash1)).To(BeFalse())
			})
		})

		When("object is related to 1 reference 'master' only and target ref is master", func() {
			When("object is also included in the unfinalized object cache", func() {
				BeforeEach(func() {
					pr, err = newPushReader(&WriteCloser{Buffer: bytes.NewBuffer(nil)}, repo)
					Expect(err).To(BeNil())

					hash1 = createBlob(path, "abc")
					Expect(repo.ObjectExist(hash1)).To(BeTrue())

					pr.objects = []*packObject{
						{Type: 1, Hash: plumbing.NewHash(hash1)},
					}
					pr.objectsRefs[hash1] = []string{"master"}

					mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash1).Return(true)

					errs := removeObjRelatedToOnlyRef(repo, pr, mockMgr, "master")
					Expect(errs).To(HaveLen(0))
				})

				It("should not remove the reference 'master' from the object's reference list", func() {
					Expect(pr.objectsRefs[hash1]).To(ContainElement("master"))
					Expect(pr.objectsRefs[hash1]).ToNot(BeEmpty())
				})

				It("should not remove the object from disk", func() {
					Expect(repo.ObjectExist(hash1)).To(BeTrue())
				})
			})
		})

		When("object is related to 2 reference 'master' and 'dev' and target ref is master", func() {
			BeforeEach(func() {
				pr, err = newPushReader(&WriteCloser{Buffer: bytes.NewBuffer(nil)}, repo)
				Expect(err).To(BeNil())

				hash1 = createBlob(path, "abc")
				Expect(repo.ObjectExist(hash1)).To(BeTrue())

				pr.objects = []*packObject{
					{Type: 1, Hash: plumbing.NewHash(hash1)},
				}

				pr.objectsRefs[hash1] = []string{"master", "dev"}

				mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash1).Return(false)

				errs := removeObjRelatedToOnlyRef(repo, pr, mockMgr, "master")
				Expect(errs).To(HaveLen(0))
			})

			It("should remove the reference 'master' from the object's reference list", func() {
				Expect(pr.objectsRefs[hash1]).ToNot(ContainElement("master"))
				Expect(pr.objectsRefs[hash1]).To(HaveLen(1))
			})

			It("should not remove the object from disk", func() {
				Expect(repo.ObjectExist(hash1)).To(BeTrue())
			})
		})

		When("2 objects are related to reference 'master' and target ref is master", func() {
			BeforeEach(func() {
				pr, err = newPushReader(&WriteCloser{Buffer: bytes.NewBuffer(nil)}, repo)
				Expect(err).To(BeNil())

				hash1 = createBlob(path, "abc")
				hash2 = createBlob(path, "xyz")
				Expect(repo.ObjectExist(hash1)).To(BeTrue())
				Expect(repo.ObjectExist(hash2)).To(BeTrue())

				pr.objects = []*packObject{
					{Type: 1, Hash: plumbing.NewHash(hash1)},
					{Type: 1, Hash: plumbing.NewHash(hash2)},
				}

				pr.objectsRefs[hash1] = []string{"master"}
				pr.objectsRefs[hash2] = []string{"master"}

				mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash1).Return(false)
				mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash2).Return(false)

				errs := removeObjRelatedToOnlyRef(repo, pr, mockMgr, "master")
				Expect(errs).To(HaveLen(0))
			})

			It("should remove the reference 'master' from the objects' reference list", func() {
				Expect(pr.objectsRefs[hash1]).ToNot(ContainElement("master"))
				Expect(pr.objectsRefs[hash1]).To(HaveLen(0))
				Expect(pr.objectsRefs[hash2]).ToNot(ContainElement("master"))
				Expect(pr.objectsRefs[hash2]).To(HaveLen(0))
			})

			It("should not remove both objects from disk", func() {
				Expect(repo.ObjectExist(hash1)).To(BeFalse())
				Expect(repo.ObjectExist(hash2)).To(BeFalse())
			})
		})
	})

	Describe(".removeObjsRelatedToRefs", func() {
		var pr *PushReader
		var err error
		var hash1 string

		When("object is related to 1 reference 'master' only", func() {
			BeforeEach(func() {
				pr, err = newPushReader(&WriteCloser{Buffer: bytes.NewBuffer(nil)}, repo)
				Expect(err).To(BeNil())

				hash1 = createBlob(path, "abc")
				Expect(repo.ObjectExist(hash1)).To(BeTrue())

				pr.objects = []*packObject{
					{Type: 1, Hash: plumbing.NewHash(hash1)},
				}
				pr.objectsRefs[hash1] = []string{"master"}

				mockMgr.EXPECT().IsUnfinalizedObject(repo.GetName(), hash1).Return(false)

				errs := removeObjsRelatedToRefs(repo, pr, mockMgr, []string{"master"})
				Expect(errs).To(HaveLen(0))
			})

			It("should remove the reference 'master' from the object's reference list", func() {
				Expect(pr.objectsRefs[hash1]).ToNot(ContainElement("master"))
				Expect(pr.objectsRefs[hash1]).To(BeEmpty())
			})

			It("should remove the object from disk", func() {
				Expect(repo.ObjectExist(hash1)).To(BeFalse())
			})
		})
	})
})
