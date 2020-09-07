package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht/announcer"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/push/types"
	rr "github.com/make-os/lobe/remote/repo"
	testutil2 "github.com/make-os/lobe/remote/testutil"
	types2 "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
)

var _ = Describe("Server", func() {
	var err error
	var cfg *config.AppConfig
	var svr *Server
	var path, repoName string
	var repo types2.LocalRepo
	var ctrl *gomock.Controller
	var mockLogic *testutil.MockObjects
	var mockDHT *mocks.MockDHT
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter
	var mockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		mockLogic = testutil.MockLogic(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockRepoSyncInfoKeeper = mockLogic.RepoSyncInfoKeeper

		mockDHT = mocks.NewMockDHT(ctrl)
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeRepoName, gomock.Any())
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeGit, gomock.Any())

		port, _ := freeport.GetFreePort()
		svr = New(cfg, fmt.Sprintf(":%d", port), mockLogic.Logic, mockDHT, mockMempool, mockBlockGetter)

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = rr.Get(path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		svr.Stop()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetRepoState", func() {
		When("no objects exist", func() {
			It("should return empty state", func() {
				st, err := svr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.IsEmpty()).To(BeTrue())
			})
		})

		When("a commit exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := svr.GetRepoState(repo)
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
				st, err := svr.GetRepoState(repo)
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
				st, err := svr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/master"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref refs/heads/dev", func() {
				st, err := svr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/dev"))
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
				st, err := svr.GetRepoState(repo, plumbing.MatchOpt("refs/heads/dev"))
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
				st, err := svr.GetRepoState(repo, plumbing.MatchOpt("refs/tags/tag"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref=refs/tags/tag2", func() {
				st, err := svr.GetRepoState(repo, plumbing.MatchOpt("refs/tags/tag2"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})
	})

	Describe(".registerNoteSender", func() {
		It("should add to cache", func() {
			Expect(svr.noteSenders.Len()).To(Equal(0))
			svr.registerNoteSender("sender", "txID")
			Expect(svr.noteSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isNoteSender", func() {
		It("should return true if sender + txID is cached", func() {
			svr.registerNoteSender("sender", "txID")
			Expect(svr.noteSenders.Len()).To(Equal(1))
			isSender := svr.isNoteSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := svr.isNoteSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".registerEndorsementSender", func() {
		It("should add to cache", func() {
			Expect(svr.endorsementSenders.Len()).To(Equal(0))
			svr.registerEndorsementSender("sender", "txID")
			Expect(svr.endorsementSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isEndorsementSender", func() {
		It("should return true if sender + txID is cached", func() {
			svr.registerEndorsementSender("sender", "txID")
			Expect(svr.endorsementSenders.Len()).To(Equal(1))
			isSender := svr.isEndorsementSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := svr.isEndorsementSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".registerNoteEndorsement", func() {
		When("1 Endorsement for id=abc is added", func() {
			BeforeEach(func() {
				pushEnd := &types.PushEndorsement{SigBLS: util.RandBytes(5)}
				svr.registerNoteEndorsement("abc", pushEnd)
			})

			Specify("that id=abc has 1 Endorsement", func() {
				Expect(svr.endorsements.Len()).To(Equal(1))
				pushEndList := svr.endorsements.Get("abc")
				Expect(pushEndList).To(HaveLen(1))
			})
		})

		When("2 endorsements for id=abc are added", func() {
			BeforeEach(func() {
				pushEnd := &types.PushEndorsement{SigBLS: util.RandBytes(5)}
				pushEnd2 := &types.PushEndorsement{SigBLS: util.RandBytes(5)}
				svr.registerNoteEndorsement("abc", pushEnd)
				svr.registerNoteEndorsement("abc", pushEnd2)
			})

			Specify("that id=abc has 2 Endorsement", func() {
				Expect(svr.endorsements.Len()).To(Equal(1))
				pushEndList := svr.endorsements.Get("abc")
				Expect(pushEndList).To(HaveLen(2))
			})
		})
	})

	Describe(".checkRepo", func() {
		It("should return false if error checking repo's existence", func() {
			Expect(svr.checkRepo("", []byte("repo"))).To(BeFalse())
		})

		It("should return true if repo exists", func() {
			Expect(svr.checkRepo("", []byte(repoName))).To(BeTrue())
		})
	})

	Describe(".checkRepoObject", func() {
		It("should return true if object exists", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			recentHash := testutil2.GetRecentCommitHash(path, "master")
			recentHashHex, _ := util.FromHex(recentHash)
			Expect(svr.checkRepoObject(repoName, recentHashHex)).To(BeTrue())
		})

		It("should return false if object does not exists", func() {
			recentHashHex, _ := util.FromHex("b4952909ef739a347d6d323e0d8700bf0cc346e1")
			Expect(svr.checkRepoObject(repoName, recentHashHex)).To(BeFalse())
		})
	})

	Describe(".applyRepoTrackingConfig", func() {
		It("should track repos provided in Config.Repo.Track", func() {
			cfg.Repo.Track = []string{"repo1", "repo2"}
			mockRepoSyncInfoKeeper.EXPECT().Track("repo1")
			mockRepoSyncInfoKeeper.EXPECT().Track("repo2")
			svr.applyRepoTrackingConfig()
		})

		It("should track repos provided in Config.Repo.UnTrack", func() {
			cfg.Repo.Untrack = []string{"repo1", "repo2"}
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo1")
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo2")
			svr.applyRepoTrackingConfig()
		})

		It("should track repos provided in Config.Repo.Track", func() {
			cfg.Repo.UntrackAll = true
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {},
				"repo2": {},
			})
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo1")
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo2")
			svr.applyRepoTrackingConfig()
		})
	})
})
