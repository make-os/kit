package validation_test

import (
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var testPushKeyGetter = func(pubKey *crypto.PubKey, err error) func(pushKeyID string) (crypto.PublicKey, error) {
	return func(pushKeyID string) (crypto.PublicKey, error) {
		if pubKey == nil {
			return crypto.EmptyPublicKey, err
		}
		return pubKey.ToPublicKey(), err
	}
}

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var testRepo types.LocalRepo
	var path string
	var pubKey *crypto.PubKey
	var privKey *crypto.Key
	var ctrl *gomock.Controller
	var baseTxDetail *types.TxDetail

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		privKey = crypto.NewKeyFromIntSeed(1)
		baseTxDetail = &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
		pubKey = privKey.PubKey()

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		testRepo, err = repo.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckCommit", func() {
		var commit *object.Commit
		var err error

		When("commit was not signed", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) was not signed"))
			})
		})

		When("commit is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(nil, fmt.Errorf("not found")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get push key (.*): not found"))
			})
		})

		When("commit has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("signature is malformed"))
			})
		})

		When("commit signature header could not be decoded", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{
					Bytes:   []byte{1, 2, 3},
					Headers: map[string]string{"nonce": "invalid"},
					Type:    "SIGNATURE"}))
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to decode PEM header: nonce must be numeric"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{
					Bytes:   []byte{1, 2, 3},
					Headers: txDetail.GetPEMHeader(),
					Type:    "SIGNATURE"}))
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("object (.*) signature is invalid"))
			})
		})

		When("commit has a valid signature but its decoded header did not match the request transaction info", func() {
			var err error
			var sig []byte
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(commitHash))
				sigMsg := validation.GetCommitOrTagSigMsg(commit)

				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				pemHeader := txDetail.GetPEMHeader()

				sig, err = privKey.PrivKey().Sign(append([]byte(sigMsg), txDetail.BytesNoSig()...))
				Expect(err).To(BeNil())

				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: pemHeader, Type: "SIGNATURE"}))
				err = validation.CheckCommit(commit, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrSigHeaderAndReqParamsMismatch))
			})
		})

		When("commit has a valid signature and the decoded signature header matches the request transaction info", func() {
			var err error
			var sig []byte
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(commitHash))
				sigMsg := validation.GetCommitOrTagSigMsg(commit)

				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				pemHeader := txDetail.GetPEMHeader()

				sig, err = privKey.PrivKey().Sign(append([]byte(sigMsg), txDetail.BytesNoSig()...))
				Expect(err).To(BeNil())

				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: pemHeader, Type: "SIGNATURE"}))
				err = validation.CheckCommit(commit, txDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("tag is not signed", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())
				err = validation.CheckAnnotatedTag(tob, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='txDetail was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("tag (.*) is unsigned. Sign the tag with your push key"))
			})
		})

		When("tag is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				err = validation.CheckAnnotatedTag(tob, baseTxDetail, testPushKeyGetter(nil, fmt.Errorf("bad error")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get pusher key(.*) to verify commit .*"))
			})
		})

		When("tag has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				err = validation.CheckAnnotatedTag(tob, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("signature is malformed"))
			})
		})

		When("tag has a signature but the signature is invalid", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())

				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				sig := pem.EncodeToMemory(&pem.Block{Bytes: []byte("invalid sig"), Headers: txDetail.GetPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(sig)

				err = validation.CheckAnnotatedTag(tob, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("object (.*) signature is invalid"))
			})
		})

		When("tag has a valid signature but the signature header does not match the request transaction info", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())

				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				msg := validation.GetCommitOrTagSigMsg(tob)
				sig, _ := privKey.PrivKey().Sign(append([]byte(msg), txDetail.BytesNoSig()...))
				pemData := pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: txDetail.GetPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(pemData)

				err = validation.CheckAnnotatedTag(tob, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrSigHeaderAndReqParamsMismatch))
			})
		})

		When("tag has signature and header are valid but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())

				txDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				msg := validation.GetCommitOrTagSigMsg(tob)
				sig, _ := privKey.PrivKey().Sign(append([]byte(msg), txDetail.BytesNoSig()...))
				pemData := pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: txDetail.GetPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(pemData)

				err = validation.CheckAnnotatedTag(tob, txDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) was not signed"))
			})
		})
	})

	Describe(".CheckNote", func() {
		var err error

		When("unable to get note", func() {
			BeforeEach(func() {
				detail := &types.TxDetail{Reference: "refs/notes/note1"}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().RefGet(detail.Reference).Return("", fmt.Errorf("bad error"))
				err = validation.CheckNote(mockRepo, detail)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to get note: bad error"))
			})
		})

		When("current note hash is different from tx detail hash", func() {
			BeforeEach(func() {
				hash := util.RandString(40)
				detail := &types.TxDetail{Reference: "refs/notes/note1", Head: hash}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				noteHash := util.RandString(40)
				mockRepo.EXPECT().RefGet(detail.Reference).Return(noteHash, nil)
				err = validation.CheckNote(mockRepo, detail)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("current note hash differs from signed note hash"))
			})
		})
	})

	Describe(".validation.ValidateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/others/name", Data: "stuff"}}
				err = validation.ValidateChange(testRepo, "", change, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				err = validation.ValidateChange(testRepo, "", change, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				err = validation.ValidateChange(testRepo, "", change, baseTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='unable to get tag object: tag not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get tag object: tag not found"))
			})
		})
	})

	Describe(".IsBlockedByScope", func() {
		It("should return true when scopes has r/repo1 and tx repo=repo2 and namespace=''", func() {
			scopes := []string{"r/repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has r/repo1 and tx repo=repo1 and namespace=''", func() {
			scopes := []string{"r/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has ns1/repo1 and tx repo=repo1 and namespace=ns2", func() {
			scopes := []string{"ns1/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns2"}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has ns1/repo1 and tx repo=repo1 and namespace=ns1", func() {
			scopes := []string{"ns1/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has ns1/ and tx repo=repo1 and namespace=ns2", func() {
			scopes := []string{"ns1/"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns2"}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has ns1/ and tx repo=repo1 and namespace=ns1", func() {
			scopes := []string{"ns1/"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return false when scopes has repo1 and tx repo=repo1 and namespace=''", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has repo1 and tx repo=repo2 and namespace=''", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return true when scopes has repo1 and tx repo=repo2 and "+
			"namespace='ns1' "+
			"but ns1/repo2 does not point to repo1", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			ns.Domains["repo2"] = "repo100"
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has repo1 and tx repo=repo2 and "+
			"namespace='ns1' "+
			"but ns1/repo2 does not point to r/repo1", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			ns.Domains["repo2"] = "r/repo1"
			Expect(validation.IsBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})
	})
})
