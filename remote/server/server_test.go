package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/push/types"
	rr "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	types2 "gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Server", func() {
	var err error
	var cfg *config.AppConfig
	var repoMgr *Server
	var path, repoName string
	var repo types2.LocalRepo
	var ctrl *gomock.Controller
	var mockLogic *testutil.MockObjects
	var mockDHT *mocks.MockDHT
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		port, _ := freeport.GetFreePort()
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
		mockDHT = mocks.NewMockDHT(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port), mockLogic.Logic,
			mockDHT, mockMempool, mockBlockGetter)

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = rr.Get(path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetRepoState", func() {
		When("no objects exist", func() {
			It("should return empty state", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.IsEmpty()).To(BeTrue())
			})
		})

		When("a commit exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("two branches with 1 commit each exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 2 refs", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(2)))
			})
		})

		When("match=refs/heads", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag1")
			})

			Specify("that the repo has ref refs/heads/master", func() {
				st, err := repoMgr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/master"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref refs/heads/dev", func() {
				st, err := repoMgr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("branch master and dev exist and match=refs/heads/dev", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("match=refs/tags", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag")
				testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag2")
			})

			Specify("that the repo has ref=refs/tags/tag", func() {
				st, err := repoMgr.GetRepoState(repo, plumbing.MatchOpt("refs/tags/tag"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref=refs/tags/tag2", func() {
				st, err := repoMgr.GetRepoState(repo, plumbing.MatchOpt("refs/tags/tag2"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})
	})

	Describe(".FindObject", func() {
		When("key is not valid", func() {
			It("should return err", func() {
				_, err := repoMgr.FindObject([]byte("invalid"))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid repo object key"))
			})
		})

		When("key includes an object hash with unexpected length", func() {
			It("should return err", func() {
				_, err := repoMgr.FindObject([]byte("repo/object_hash"))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid object hash"))
			})
		})

		When("target repo does not exist", func() {
			It("should return err", func() {
				_, err := repoMgr.FindObject([]byte("repo/" + strings.Repeat("0", 40)))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("repository does not exist"))
			})
		})

		When("target object does not exist", func() {
			It("should return err", func() {
				_, err := repoMgr.FindObject([]byte(repoName + "/" + strings.Repeat("0", 40)))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("object not found"))
			})
		})
	})

	Describe(".cacheNoteSender", func() {
		It("should add to cache", func() {
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(0))
			repoMgr.cacheNoteSender("sender", "txID")
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isPushNoteSender", func() {
		It("should return true if sender + txID is cached", func() {
			repoMgr.cacheNoteSender("sender", "txID")
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(1))
			isSender := repoMgr.isPushNoteSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := repoMgr.isPushNoteSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".cachePushEndSender", func() {
		It("should add to cache", func() {
			Expect(repoMgr.pushEndSenders.Len()).To(Equal(0))
			repoMgr.cachePushEndSender("sender", "txID")
			Expect(repoMgr.pushEndSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isPushEndSender", func() {
		It("should return true if sender + txID is cached", func() {
			repoMgr.cachePushEndSender("sender", "txID")
			Expect(repoMgr.pushEndSenders.Len()).To(Equal(1))
			isSender := repoMgr.isPushEndSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := repoMgr.isPushEndSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".addPushNoteEndorsement", func() {
		When("1 PushEndorsement for id=abc is added", func() {
			BeforeEach(func() {
				pushEnd := &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))}
				repoMgr.addPushNoteEndorsement("abc", pushEnd)
			})

			Specify("that id=abc has 1 PushEndorsement", func() {
				Expect(repoMgr.pushEndorsements.Len()).To(Equal(1))
				pushEndList := repoMgr.pushEndorsements.Get("abc")
				Expect(pushEndList).To(HaveLen(1))
			})
		})

		When("2 PushEnds for id=abc are added", func() {
			BeforeEach(func() {
				pushEnd := &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))}
				pushEnd2 := &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))}
				repoMgr.addPushNoteEndorsement("abc", pushEnd)
				repoMgr.addPushNoteEndorsement("abc", pushEnd2)
			})

			Specify("that id=abc has 2 PushEndorsement", func() {
				Expect(repoMgr.pushEndorsements.Len()).To(Equal(1))
				pushEndList := repoMgr.pushEndorsements.Get("abc")
				Expect(pushEndList).To(HaveLen(2))
			})
		})
	})
})
