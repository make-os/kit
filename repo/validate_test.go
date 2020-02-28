package repo

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/dht/types"
	types5 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/mosdef/mocks"

	"github.com/bitfield/script"
	"golang.org/x/crypto/openpgp"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var repo core.BareRepo
	var path string
	var gpgKeyID string
	var pubKey string
	var privKey *openpgp.Entity
	var privKey2 *openpgp.Entity
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockTickMgr *mocks.MockTicketManager
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockGPGKeeper *mocks.MockGPGPubKeyKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockTxLogic *mocks.MockTxLogic

	var gpgPubKeyGetter = func(gpgID string) (string, error) {
		return pubKey, nil
	}

	var gpgInvalidPubKeyGetter = func(gpgID string) (string, error) {
		return "invalid key", nil
	}

	var gpgPubKeyGetterWithErr = func(err error) func(gpgID string) (string, error) {
		return func(gpgID string) (string, error) {
			return "", err
		}
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		gpgKeyID = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		gpgKeyID2 := testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey, err = crypto.GetGPGPublicKeyStr(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey, err = crypto.GetGPGPrivateKey(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey2, err = crypto.GetGPGPrivateKey(gpgKeyID2, testutil.GPGProgramPath, cfg.DataDir())
		Expect(err).To(BeNil())

		GitEnv = append(GitEnv, "GNUPGHOME="+cfg.DataDir())
		mockObjs := testutil.MockLogic(ctrl)
		mockLogic = mockObjs.Logic
		mockTickMgr = mockObjs.TicketManager
		mockRepoKeeper = mockObjs.RepoKeeper
		mockGPGKeeper = mockObjs.GPGPubKeyKeeper
		mockAcctKeeper = mockObjs.AccountKeeper
		mockSysKeeper = mockObjs.SysKeeper
		mockTxLogic = mockObjs.Tx

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".checkCommit", func() {
		var cob *object.Commit
		var err error

		When("txparams is not set", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='txparams was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txparams was not set"))
			})
		})

		When("txparams.gpgID is not valid", func() {
			BeforeEach(func() {
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", "invalid_pk_id")
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("commit is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the commit with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the commit with your gpg key"))
			})
		})

		When("commit is signed but unable to get public key using the gpgID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("commit has a signature and the signature is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				appendMakeSignableCommit(path, "file.txt", "line 1", txParams, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("txparams is not set", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='txparams was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txparams was not set"))
			})
		})

		When("txparams.gpgID is not valid", func() {
			BeforeEach(func() {
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", "invalid_pk_id")
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("tag is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the tag with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the tag with your gpg key"))
			})
		})

		When("tag is signed but unable to get public key using the gpgID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("tag has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("tag has a valid signature but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txParams, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err=''", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*referenced commit.* is unsigned.*"))
			})
		})

		When("tag has a valid signature and the referenced commit is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createMakeSignableCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txParams, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkNote", func() {
		var err error

		When("target note does not exist", func() {
			BeforeEach(func() {
				_, err = checkNote(repo, "unknown", gpgPubKeyGetter)
			})

			It("should return err='unable to fetch note entries (unknown)'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to fetch note entries (unknown)"))
			})
		})

		When("a note does not have a tx blob object", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='unacceptable note. it does not have a signed transaction object'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unacceptable note. it does not have a signed transaction object"))
			})
		})

		When("a notes tx blob has invalid signature format", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=xyz")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='note (refs/notes/note1): field:sig, msg: signature format is not valid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1): field:sig, msg: signature format is not valid"))
			})
		})

		When("a notes tx blob has an unknown public key id", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetterWithErr(fmt.Errorf("error finding pub key")))
			})

			It("should return err='unable to verify note (refs/notes/note1). public key was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key was not found"))
			})
		})

		When("a notes tx blob includes a public key id to an invalid public key", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgInvalidPubKeyGetter)
			})

			It("should return err='unable to verify note (refs/notes/note1). public key .. was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key is not valid"))
			})
		})

		When("a note's signature is not signed with an expected private key", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := MakeNoteSigMsg("0", "0", "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", commitHash, false)
				sig, err = crypto.GPGSign(privKey2, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='invalid signature: RSA verification failure'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: RSA verification failure"))
			})
		})

		When("a note's signature message content/format is not expected", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000001" + commitHash)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='...invalid signature: hash tag doesn't match'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: hash tag doesn't match"))
			})
		})

		When("a note's signature is valid", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := MakeNoteSigMsg("0", "0", "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", commitHash, false)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 gpgID=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkMergeCompliance", func() {
		When("pushed reference is not a branch", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				change := &core.ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: pushed reference must be a branch"))
			})
		})

		When("target merge proposal does not exist", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().State().Return(state.BareRepository())
				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: merge proposal (0001) not found"))
			})
		})

		When("signer did not create the proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Creator = "address_of_creator"
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{Address: "address_xyz"})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: signer must be the creator of the merge proposal (0001)"))
			})
		})

		When("unable to check whether proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, fmt.Errorf("error"))

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: error"))
			})
		})

		When("target merge proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(true, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: merge proposal (0001) is already closed"))
			})
		})

		When("target merge proposal's base branch name does not match the pushed branch name", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base": "release",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: pushed branch name and merge proposal base branch name must match"))
			})
		})

		When("target merge proposal outcome is not 'accepted'", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base": "master",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: merge proposal (0001) has not been accepted"))
			})
		})

		When("unable to get merger initiator commit", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base": "master",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(nil, fmt.Errorf("error"))

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: error"))
			})
		})

		When("unable to get merger initiator commit has more than 1 parents", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base": "master",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(2)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: multiple targets not allowed"))
			})
		})

		When("merger commit modified worktree history of parent", func() {
			When("tree hash is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
						"base": "master",
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge compliance error: merger commit cannot modify history as seen from target commit"))
				})
			})

			When("author is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
						"base": "master",
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					mergerCommit.EXPECT().GetAuthor().Return(author)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author2@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge compliance error: merger commit cannot modify history as seen from target commit"))
				})
			})

			When("committer is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
						"base": "master",
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					mergerCommit.EXPECT().GetAuthor().Return(author)
					committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
					mergerCommit.EXPECT().GetCommitter().Return(committer)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)
					committer = &object.Signature{Name: "committer1", Email: "committer2@email.com"}
					targetCommit.EXPECT().GetCommitter().Return(committer)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge compliance error: merger commit cannot modify history as seen from target commit"))
				})
			})
		})

		When("old pushed branch hash is different from old branch hash described in the merge proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base":     "master",
					"baseHash": "xyz",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "abc"}

				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(1)
				treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				mergerCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				mergerCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: merge proposal base branch hash is stale or invalid"))
			})
		})

		When("merge proposal target hash does not match the expected target hash", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData[core.ProposalActionDataMergeRequest] = map[string]interface{}{
					"base":       "master",
					"baseHash":   "abc",
					"targetHash": "target_xyz",
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockGPGKeeper.EXPECT().GetGPGPubKey("gpg_key_id").Return(&state.GPGPubKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "abc"}

				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(1)
				treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				mergerCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				mergerCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				targetHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("target_abc"))
				targetCommit.EXPECT().GetHash().Return(targetHash)
				treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "gpg_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge compliance error: target commit " +
					"hash and the merge proposal target hash must match"))
			})
		})
	})

	Describe(".validateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get tag object: tag not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get tag object: tag not found"))
			})
		})

		When("branch item points to a valid commit", func() {
			var cob *object.Commit
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				appendMakeSignableCommit(path, "file.txt", "line 1", txParams, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: cob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("annotated tag item points to a valid commit", func() {
			var tob *object.Tag
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createMakeSignableCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txParams, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				change := &core.ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("lightweight tag item points to a valid commit", func() {
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				gpgID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", "0", "0", gpgID)
				createMakeSignableCommitAndLightWeightTag(path, "file.txt", "first file", txParams, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")

				change := &core.ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tagRef.Target().String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckPushOK", func() {
		It("should return error when push note id is not set", func() {
			err := CheckPushOK(&core.PushOK{}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.pushNoteID, msg:push note id is required"))
		})

		It("should return error when public key is not valid", func() {
			err := CheckPushOK(&core.PushOK{
				PushNoteID:   util.StrToBytes32("id"),
				SenderPubKey: util.EmptyBytes32,
			}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key is required"))
		})
	})

	Describe(".CheckPushOKConsistency", func() {
		When("unable to fetch top hosts", func() {
			BeforeEach(func() {
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: util.EmptyBytes32,
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("sender is not a host", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*types5.SelectedTicket{}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key does not belong to an active host"))
			})
		})

		When("unable to decode host's BLS public key", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*types5.SelectedTicket{
					{
						Ticket: &types5.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      util.RandBytes(128),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("unable to verify signature", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*types5.SelectedTicket{
					{
						Ticket: &types5.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:endorsements.sig, msg:signature could not be verified"))
			})
		})

		When("noSigCheck is true", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*types5.SelectedTicket{
					{
						Ticket: &types5.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, true, -1)
			})

			It("should not check signature", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkPushNoteSyntax", func() {
		key := crypto.NewKeyFromIntSeed(1)
		okTx := &core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().MustBytes32()}
		bz, _ := key.PrivKey().Sign(okTx.Bytes())
		okTx.NodeSig = bz

		var cases = [][]interface{}{
			{&core.PushNote{}, "field:repoName, msg:repo name is required"},
			{&core.PushNote{RepoName: "repo"}, "field:pusherKeyId, msg:pusher gpg key id is required"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: []byte("xyz")}, "field:pusherKeyId, msg:pusher gpg key is not valid"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: 0}, "field:timestamp, msg:timestamp is required"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: 2000000000}, "field:timestamp, msg:timestamp cannot be a future time"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix()}, "field:accountNonce, msg:account nonce must be greater than zero"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), AccountNonce: 1, Fee: ""}, "field:fee, msg:fee is required"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), AccountNonce: 1, Fee: "one"}, "field:fee, msg:fee must be numeric"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), AccountNonce: 1, Fee: "1"}, "field:nodePubKey, msg:push node public key is required"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), AccountNonce: 1, Fee: "1", NodePubKey: key.PubKey().MustBytes32()}, "field:nodeSig, msg:push node signature is required"},
			{&core.PushNote{RepoName: "repo", PusherKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), AccountNonce: 1, Fee: "1", NodePubKey: key.PubKey().MustBytes32(), NodeSig: []byte("invalid signature")}, "field:nodeSig, msg:failed to verify signature"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{}}}, "index:0, field:references.name, msg:name is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1"}}}, "index:0, field:references.oldHash, msg:old hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: "invalid"}}}, "index:0, field:references.oldHash, msg:old hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40)}}}, "index:0, field:references.newHash, msg:new hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: "invalid"}}}, "index:0, field:references.newHash, msg:new hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40)}}}, "index:0, field:references.nonce, msg:reference nonce must be greater than zero"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Objects: []string{"invalid object"}}}}, "index:0, field:references.objects.0, msg:object hash is not valid"},
		}

		It("should check cases", func() {
			for _, c := range cases {
				_c := c
				if _c[1] != nil {
					Expect(CheckPushNoteSyntax(_c[0].(*core.PushNote)).Error()).To(Equal(_c[1]))
				} else {
					Expect(CheckPushNoteSyntax(_c[0].(*core.PushNote))).To(BeNil())
				}
			}
		})
	})

	Describe(".checkPushedReference", func() {
		var mockKeepers *mocks.MockKeepers
		var mockRepo *mocks.MockBareRepo
		var oldHash = fmt.Sprintf("%x", util.Hash20(util.RandBytes(16)))

		BeforeEach(func() {
			mockKeepers = mocks.NewMockKeepers(ctrl)
			mockRepo = mocks.NewMockBareRepo(ctrl)
		})

		When("old hash is non-zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*core.PushedReference{
					{Name: "refs/heads/master", OldHash: oldHash},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{}),
				}
				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash is zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*core.PushedReference{
					{Name: "refs/heads/master", OldHash: strings.Repeat("0", 40)},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{}),
				}
				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should not return error about unknown reference", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:0, field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash of reference is different from the local hash of same reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("old hash of reference is non-zero and the local equivalent reference is not accessible", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(nil, plumbing.ErrReferenceNotFound)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and nil repo passed", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}

				err = checkPushedReference(nil, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:0, field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and it is different from the hash of the equivalent local reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("pushed reference nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*core.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: newHash,
						Objects: []string{newHash},
						Nonce:   2,
					},
				}

				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' has nonce '2', expecting '1'"))
			})
		})

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*core.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: newHash,
						Objects: []string{newHash},
						Nonce:   1,
					},
				}

				repository := &state.Repository{
					References: state.References(map[string]interface{}{
						refName: &state.Reference{Nonce: 0},
					}),
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckPushNoteConsistency", func() {

		When("no repository with matching name exist", func() {
			BeforeEach(func() {
				tx := &core.PushNote{RepoName: "unknown"}
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(state.BareRepository())
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoName, msg:repository named 'unknown' is unknown"))
			})
		})

		When("pusher public key id is unknown", func() {
			BeforeEach(func() {
				tx := &core.PushNote{RepoName: "repo1", PusherKeyID: util.RandBytes(20)}
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&state.Repository{Balance: "10"})
				mockGPGKeeper.EXPECT().GetGPGPubKey(util.MustToRSAPubKeyID(tx.PusherKeyID)).Return(state.BareGPGPubKey())
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:pusherKeyId, msg:pusher's public key id '.*' is unknown"))
			})
		})

		When("gpg owner address not the same as the pusher address", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:      "repo1",
					PusherKeyID:   util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&state.Repository{Balance: "10"})

				gpgKey := state.BareGPGPubKey()
				gpgKey.Address = util.String("address2")
				mockGPGKeeper.EXPECT().GetGPGPubKey(util.MustToRSAPubKeyID(tx.PusherKeyID)).Return(gpgKey)
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:gpg key is not associated with the pusher address"))
			})
		})

		When("unable to find pusher account", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:      "repo1",
					PusherKeyID:   util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&state.Repository{Balance: "10"})

				gpgKey := state.BareGPGPubKey()
				gpgKey.Address = util.String("address1")
				mockGPGKeeper.EXPECT().GetGPGPubKey(util.MustToRSAPubKeyID(tx.PusherKeyID)).Return(gpgKey)

				mockAcctKeeper.EXPECT().GetAccount(tx.PusherAddress).Return(state.BareAccount())

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:pusher account not found"))
			})
		})

		When("push note account nonce not correct", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:      "repo1",
					PusherKeyID:   util.RandBytes(20),
					PusherAddress: "address1",
					AccountNonce:  3,
				}
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&state.Repository{Balance: "10"})

				gpgKey := state.BareGPGPubKey()
				gpgKey.Address = util.String("address1")
				mockGPGKeeper.EXPECT().GetGPGPubKey(util.MustToRSAPubKeyID(tx.PusherKeyID)).Return(gpgKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().GetAccount(tx.PusherAddress).Return(acct)

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:wrong account nonce '3', expecting '2'"))
			})
		})

		When("pusher account balance not sufficient to pay fee", func() {
			BeforeEach(func() {

				tx := &core.PushNote{
					RepoName:      "repo1",
					PusherKeyID:   util.RandBytes(20),
					PusherAddress: "address1",
					AccountNonce:  2,
					Fee:           "10",
				}

				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(&state.Repository{Balance: "10"})

				gpgKey := state.BareGPGPubKey()
				gpgKey.Address = util.String("address1")
				mockGPGKeeper.EXPECT().GetGPGPubKey(util.MustToRSAPubKeyID(tx.PusherKeyID)).Return(gpgKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().GetAccount(tx.PusherAddress).Return(acct)

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mockTxLogic.EXPECT().
					CanExecCoinTransfer(tx.PusherAddress, util.String("0"), tx.Fee, uint64(2), uint64(1)).
					Return(fmt.Errorf("insufficient"))

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("insufficient"))
			})
		})
	})

	Describe(".fetchAndCheckReferenceObjects", func() {
		When("object does not exist in the dht", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(nil, fmt.Errorf("object not found"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch object 'obj_hash': object not found"))
			})
		})

		When("object exist in the dht but failed to write to repository", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(fmt.Errorf("something bad"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write fetched object 'obj_hash' to disk: something bad"))
			})
		})

		When("object exist in the dht and successfully written to disk", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 7}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("object exist in the dht, successfully written to disk and object size is different from actual size", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 10}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &types.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:size, msg:invalid size (10 bytes). actual object size (7 bytes) is different"))
			})
		})
	})

	Describe(".checkPushNoteAgainstTxParamss", func() {
		When("pusher key in push note is different from txparamss pusher key", func() {
			BeforeEach(func() {
				pn := &core.PushNote{PusherKeyID: util.MustDecodeRSAPubKeyID("gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {PubKeyID: util.MustToRSAPubKeyID(util.RandBytes(20))},
				}
				err = checkPushNoteAgainstTxParamss(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note pusher public key id does not match txparamss pusher public key id"))
			})
		})

		When("fee do not match", func() {
			BeforeEach(func() {
				gpgID := util.RandBytes(20)
				pn := &core.PushNote{PusherKeyID: gpgID, Fee: "9"}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {
						PubKeyID: util.MustToRSAPubKeyID(gpgID),
						Fee:      "10",
					},
				}
				err = checkPushNoteAgainstTxParamss(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note fees does not match total txparamss fees"))
			})
		})

		When("push note has unexpected pushed reference", func() {
			BeforeEach(func() {
				gpgID := util.RandBytes(20)
				pn := &core.PushNote{
					PusherKeyID: gpgID,
					Fee:         "10",
					References: []*core.PushedReference{
						{Name: "refs/heads/dev"},
					},
				}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {
						PubKeyID: util.MustToRSAPubKeyID(gpgID),
						Fee:      "10",
					},
				}
				err = checkPushNoteAgainstTxParamss(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note has unexpected pushed reference (refs/heads/dev)"))
			})
		})
	})
})
