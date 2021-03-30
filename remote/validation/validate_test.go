package validation_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testPushKeyGetter = func(pubKey *ed25519.PubKey, err error) func(pushKeyID string) (ed25519.PublicKey, error) {
	return func(pushKeyID string) (ed25519.PublicKey, error) {
		if pubKey == nil {
			return ed25519.EmptyPublicKey, err
		}
		return pubKey.ToPublicKey(), err
	}
}

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var testRepo types.LocalRepo
	var path string
	var pubKey *ed25519.PubKey
	var privKey *ed25519.Key
	var ctrl *gomock.Controller
	var testTxDetail *types.TxDetail
	var mockKeepers *mocks.MockKeepers

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		privKey = ed25519.NewKeyFromIntSeed(1)
		testTxDetail = &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
		pubKey = privKey.PubKey()

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		testRepo, err = repo.GetWithGitModule(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
		mockKeepers = mocks.NewMockKeepers(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckCommit", func() {
		var commit *object.Commit

		When("commit's hash and the signed head did not match", func() {
			var err error
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(commitHash))
				testTxDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String()}
				Expect(err).To(BeNil())
				err = validation.CheckCommit(commit, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrPushedAndSignedHeadMismatch))
			})
		})

		When("commit's hash and the signed head match", func() {
			var err error
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := testRepo.GetRecentCommitHash()
				commit, _ = testRepo.CommitObject(plumbing.NewHash(commitHash))
				testTxDetail := &types.TxDetail{Fee: "0", PushKeyID: pubKey.PushAddr().String(), Head: commitHash}
				err = validation.CheckCommit(commit, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should not return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("tag's hash and the signed head did not match", func() {
			BeforeEach(func() {
				testTxDetail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Head: "hash1"}
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())
				err = validation.CheckAnnotatedTag(tob, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(validation.ErrPushedAndSignedHeadMismatch))
			})
		})

		When("tag's hash and the signed head match", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := testRepo.Tag("v1")
				tob, _ = testRepo.TagObject(tagRef.Hash())
				testTxDetail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Head: tagRef.Hash().String()}
				err = validation.CheckAnnotatedTag(tob, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
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
				Expect(err.Error()).To(Equal("pushed object hash differs from signed reference hash"))
			})
		})
	})

	Describe(".validation.ValidateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/others/name", Data: "stuff"}}
				err = validation.ValidateChange(mockKeepers, testRepo, "", change, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				err = validation.ValidateChange(mockKeepers, testRepo, "", change, testTxDetail, testPushKeyGetter(pubKey, nil))
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &types.ItemChange{Item: &plumbing2.Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				err = validation.ValidateChange(mockKeepers, testRepo, "", change, testTxDetail, testPushKeyGetter(pubKey, nil))
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
