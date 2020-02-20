package repo

import (
	"fmt"
	"gitlab.com/makeos/mosdef/repo/types/core"
	"gitlab.com/makeos/mosdef/repo/types/msgs"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/phayes/freeport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/mocks"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Manager", func() {
	var err error
	var cfg *config.AppConfig
	var repoMgr *Manager
	var path, repoName string
	var repo core.BareRepo
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
	})

	BeforeEach(func() {
		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = GetRepo(path)
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
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("two branches with 1 commit each exist", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 2 refs", func() {
				st, err := repoMgr.GetRepoState(repo)
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(2)))
			})
		})

		When("match=refs/heads", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				createCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag1")
			})

			Specify("that the repo has ref refs/heads/master", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/master"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref refs/heads/dev", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("branch master and dev exist and match=refs/heads/dev", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				appendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 1 ref", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/heads/dev"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})
		})

		When("match=refs/tags", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				createCheckoutBranch(path, "dev")
				createCommitAndAnnotatedTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag")
				createCommitAndLightWeightTag(path, "file.txt", "some text for tag", "commit msg for tag", "tag2")
			})

			Specify("that the repo has ref=refs/tags/tag", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/tags/tag"))
				Expect(err).To(BeNil())
				Expect(st.GetReferences().Len()).To(Equal(int64(1)))
			})

			Specify("that the repo has ref=refs/tags/tag2", func() {
				st, err := repoMgr.GetRepoState(repo, matchOpt("refs/tags/tag2"))
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

	Describe(".cachePushNoteSender", func() {
		It("should add to cache", func() {
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(0))
			repoMgr.cachePushNoteSender("sender", "txID")
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isPushNoteSender", func() {
		It("should return true if sender + txID is cached", func() {
			repoMgr.cachePushNoteSender("sender", "txID")
			Expect(repoMgr.pushNoteSenders.Len()).To(Equal(1))
			isSender := repoMgr.isPushNoteSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := repoMgr.isPushNoteSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".cachePushOkSender", func() {
		It("should add to cache", func() {
			Expect(repoMgr.pushOKSenders.Len()).To(Equal(0))
			repoMgr.cachePushOkSender("sender", "txID")
			Expect(repoMgr.pushOKSenders.Len()).To(Equal(1))
		})
	})

	Describe(".isPushOKSender", func() {
		It("should return true if sender + txID is cached", func() {
			repoMgr.cachePushOkSender("sender", "txID")
			Expect(repoMgr.pushOKSenders.Len()).To(Equal(1))
			isSender := repoMgr.isPushOKSender("sender", "txID")
			Expect(isSender).To(BeTrue())
		})

		It("should return false if sender + txID is not cached", func() {
			isSender := repoMgr.isPushOKSender("sender", "txID")
			Expect(isSender).To(BeFalse())
		})
	})

	Describe(".addPushNoteEndorsement", func() {
		When("1 PushOK for id=abc is added", func() {
			BeforeEach(func() {
				pushOK := &msgs.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))}
				repoMgr.addPushNoteEndorsement("abc", pushOK)
			})

			Specify("that id=abc has 1 PushOK", func() {
				Expect(repoMgr.pushNoteEndorsements.Len()).To(Equal(1))
				pushOKList := repoMgr.pushNoteEndorsements.Get("abc")
				Expect(pushOKList).To(HaveLen(1))
			})
		})

		When("2 PushOKs for id=abc are added", func() {
			BeforeEach(func() {
				pushOK := &msgs.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))}
				pushOK2 := &msgs.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))}
				repoMgr.addPushNoteEndorsement("abc", pushOK)
				repoMgr.addPushNoteEndorsement("abc", pushOK2)
			})

			Specify("that id=abc has 2 PushOK", func() {
				Expect(repoMgr.pushNoteEndorsements.Len()).To(Equal(1))
				pushOKList := repoMgr.pushNoteEndorsements.Get("abc")
				Expect(pushOKList).To(HaveLen(2))
			})
		})
	})
})
