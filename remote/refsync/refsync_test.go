package refsync

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/push"
	"github.com/make-os/kit/remote/push/types"
	types3 "github.com/make-os/kit/remote/refsync/types"
	repo3 "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	types2 "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	. "github.com/onsi/ginkgo"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/onsi/gomega"
)

func TestRefsync(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Refsync Suite")
}

var _ = Describe("RefSync", func() {
	var err error
	var cfg *config.AppConfig
	var rs *RefSync
	var ctrl *gomock.Controller
	var mockFetcher *mocks.MockObjectFetcher
	var repoName string
	var oldHash = "5b9ba1de20344b12cce76256b67cff9bb31e77b2"
	var newHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"
	var localHash = "0b6509c59b7bc8e6f4e2d57612aedcb66a3b72c7"
	var path string
	var mockKeepers *mocks.MockKeepers
	var mockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper
	var mockPushPool *mocks.MockPushPool

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockFetcher = mocks.NewMockObjectFetcher(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockRepoSyncInfoKeeper = mocks.NewMockRepoSyncInfoKeeper(ctrl)
		mockKeepers.EXPECT().RepoSyncInfoKeeper().Return(mockRepoSyncInfoKeeper).AnyTimes()
		mockPushPool = mocks.NewMockPushPool(ctrl)
		rs = New(cfg, mockPushPool, mockFetcher, nil, mockKeepers)

		repoName = util.RandString(5)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
	})

	Describe(".CanSync", func() {
		It("should return nil if no repository is being tracked", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{})
			Expect(rs.CanSync("", "repo1")).To(BeNil())
		})

		It("should return ErrUntracked if target repo is not part of the tracked repositories", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo2": {},
			})
			err := rs.CanSync("", "repo1")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrUntracked))
		})

		When("tx targets a repo using a namespace URI", func() {
			var mockNSKeeper *mocks.MockNamespaceKeeper

			BeforeEach(func() {
				mockNSKeeper = mocks.NewMockNamespaceKeeper(ctrl)
				mockKeepers.EXPECT().NamespaceKeeper().Return(mockNSKeeper)
			})

			It("should return error if namespace does not exist", func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				tx := &txns.TxPush{Note: &types.Note{Namespace: "ns1", RepoName: "domain"}}
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash(tx.Note.GetNamespace())).Return(&state.Namespace{})
				err := rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("namespace not found"))
			})

			It("should return error if namespace's domain does not exist", func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				tx := &txns.TxPush{Note: &types.Note{Namespace: "ns1", RepoName: "domain"}}
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash(tx.Note.GetNamespace())).Return(&state.Namespace{
					Domains: map[string]string{"domain2": "target"},
				})
				err := rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("namespace's domain not found"))
			})

			It("should return ErrUntracked if namespace domain target is not a tracked repository", func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				tx := &txns.TxPush{Note: &types.Note{Namespace: "ns1", RepoName: "domain"}}
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash(tx.Note.GetNamespace())).Return(&state.Namespace{
					Domains: map[string]string{"domain": "r/repo1"},
				})
				err := rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName())
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrUntracked))
			})

			It("should return nil if namespace domain target is a tracked repository", func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				tx := &txns.TxPush{Note: &types.Note{Namespace: "ns1", RepoName: "domain"}}
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash(tx.Note.GetNamespace())).Return(&state.Namespace{
					Domains: map[string]string{"domain": "r/repo2"},
				})
				err := rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName())
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".OnNewTx", func() {

		It("should not add tx to queue if tx has all ready been seen", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo2": {},
			})
			tx := &txns.TxPush{Note: &types.Note{RepoName: "repo1"}}
			rs.queued.Add(tx.GetID(), struct{}{})
			rs.OnNewTx(tx, "", 0, 1, nil)
			Expect(len(rs.queues)).To(Equal(0))
		})

		When("target repo is untracked", func() {
			It("should not add new task to queue", func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{"repo2": {}})
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{RepoName: "repo1"}}, "", 0, 1, nil)
				Expect(len(rs.queues)).To(Equal(0))
			})
		})

		When("there are no tracked repositories", func() {
			BeforeEach(func() {
				mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{}).AnyTimes()
				rs.removeRefQueueOnEmpty = false
			})

			It("should add only one queue per reference", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, "", 0, 1, nil)
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 2}}}}, "", 1, 1, nil)
				time.Sleep(1 * time.Millisecond)
				Expect(len(rs.queues)).To(Equal(1))
				Expect(rs.queues).To(HaveKey("master"))
			})

			It("should not add new task to queue when task with matching ID already exist in queue", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, "", 0, 1, nil)
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, "", 0, 1, nil)
				time.Sleep(1 * time.Millisecond)
				Expect(len(rs.queues)).To(Equal(1))
				Expect(rs.queues).To(HaveKey("master"))
			})

			It("should not add task if reference new hash is zero-hash", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1, NewHash: plumbing.ZeroHash.String()}}}}, "", 0, 1, nil)
				time.Sleep(1 * time.Millisecond)
				Expect(len(rs.queues)).To(Equal(0))
			})

			It("should add two tasks if push transaction contains 2 different references", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "refs/heads/master", Nonce: 1}, {Name: "refs/heads/dev", Nonce: 1}}}}, "", 0, 1, nil)
				time.Sleep(1 * time.Millisecond)
				Expect(len(rs.queues)).To(Equal(2))
			})

			It("should remove reference queue when it becomes empty", func() {
				rs.removeRefQueueOnEmpty = true
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, "", 0, 1, nil)
				time.Sleep(1 * time.Millisecond)
				Expect(rs.queues).ToNot(HaveKey("master"))
			})
		})
	})

	Describe(".Do", func() {

		It("should return error when repo does not exist locally", func() {
			task := &types3.RefTask{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			err := rs.do(task)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get target repo: repository does not exist"))
		})

		It("should return error when unable to get reference from local repo", func() {
			task := &types3.RefTask{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
			mockRepo.EXPECT().RefGet(task.Ref.Name).Return("", fmt.Errorf("error"))
			err := rs.do(task)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get reference from target repo: error"))
		})

		When("local reference hash is unset", func() {
			It("should set task old hash to zero hash", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return("", nil)
				updated := false
				rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockPushPool.EXPECT().HasSeen(task.ID).Return(false)
				mockFetcher.EXPECT().OnPackReceived(gomock.Any())
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				mockRepoSyncInfoKeeper.EXPECT().UpdateRefLastSyncHeight(task.RepoName, task.Ref.Name, uint64(task.Height)).Return(nil)
				err := rs.do(task)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
				Expect(task.Ref.OldHash).To(Equal(plumbing.ZeroHash.String()))
			})
		})

		When("local reference hash and task reference new hash match", func() {
			It("should not process task and return nil error", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(newHash, nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				err := rs.do(task)
				Expect(err).To(BeNil())
			})
		})

		When("task reference new hash is an ancestor to the local reference hash", func() {
			It("should not process task and return nil error", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(localHash, nil)
				mockRepo.EXPECT().IsAncestor(newHash, localHash).Return(nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				err := rs.do(task)
				Expect(err).To(BeNil())
			})
		})

		When("local reference hash and task reference old hash do not match", func() {
			It("should set task old hash to the local hash", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(localHash, nil)
				mockRepo.EXPECT().IsAncestor(task.Ref.NewHash, localHash).Return(fmt.Errorf("some error"))
				updated := false
				rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockPushPool.EXPECT().HasSeen(task.ID).Return(false)
				mockFetcher.EXPECT().OnPackReceived(gomock.Any())
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				mockRepoSyncInfoKeeper.EXPECT().UpdateRefLastSyncHeight(task.RepoName, task.Ref.Name, uint64(task.Height)).Return(nil)
				err := rs.do(task)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
				Expect(task.Ref.OldHash).To(Equal(localHash))
			})
		})

		When("local reference hash and task reference old hash match", func() {
			It("should attempt to fetch objects and update repo", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				mockRepo.EXPECT().IsAncestor(newHash, oldHash).Return(fmt.Errorf("not ancestor"))
				updated := false
				rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockPushPool.EXPECT().HasSeen(task.ID).Return(false)
				mockFetcher.EXPECT().OnPackReceived(gomock.Any())
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				mockRepoSyncInfoKeeper.EXPECT().UpdateRefLastSyncHeight(task.RepoName, task.Ref.Name, uint64(task.Height)).Return(nil)
				err := rs.do(task)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})

			It("should not attempt to update repo and return error if fetch attempt failed", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				mockRepo.EXPECT().IsAncestor(newHash, oldHash).Return(fmt.Errorf("not ancestor"))
				updated := false
				rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(fmt.Errorf("error"))
				})
				mockPushPool.EXPECT().HasSeen(task.ID).Return(false)
				mockFetcher.EXPECT().OnPackReceived(gomock.Any())
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
				err := rs.do(task)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
				Expect(updated).To(BeFalse())
			})

			When("push note was recently added to the push pool", func() {
				It("should update repo without fetching objects", func() {
					key, _ := cfg.G().PrivVal.GetKey()
					task := &types3.RefTask{
						RepoName:    "repo1",
						Ref:         &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash},
						NoteCreator: key.PubKey().MustBytes32(),
					}
					mockRepo := mocks.NewMockLocalRepo(ctrl)
					rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
					mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
					mockRepo.EXPECT().IsAncestor(newHash, oldHash).Return(fmt.Errorf("not ancestor"))
					updated := false
					rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
						updated = true
						return nil
					}
					mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
					mockRepoSyncInfoKeeper.EXPECT().UpdateRefLastSyncHeight(task.RepoName, task.Ref.Name, uint64(task.Height)).Return(nil)
					mockPushPool.EXPECT().HasSeen(task.ID).Return(true)
					err := rs.do(task)
					Expect(err).To(BeNil())
					Expect(updated).To(BeTrue())
				})
			})
		})

		When("target repo is tracked", func() {
			It("should return error when unable to update tracked repo update height", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}, Height: 10}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				mockRepo.EXPECT().IsAncestor(newHash, oldHash).Return(fmt.Errorf("not ancestor"))
				updated := false
				rs.UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockPushPool.EXPECT().HasSeen(task.ID).Return(false)
				mockFetcher.EXPECT().OnPackReceived(gomock.Any())
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, uint64(task.Height)).Return(fmt.Errorf("error"))
				err := rs.do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to update tracked repo info: error"))
				Expect(updated).To(BeTrue())
			})
		})
	})

	Describe(".UpdateRepoUsingNote", func() {
		var repo types2.LocalRepo

		BeforeEach(func() {
			repo, err = repo3.GetWithLiteGit(cfg.Node.GitBinPath, path)
			Expect(err).To(BeNil())
		})

		It("should return error if unable to create packfile from note", func() {
			note := &types.Note{}
			err := UpdateRepoUsingNote(cfg.Node.GitBinPath, func(tx types.PushNote) (io.ReadSeeker, error) {
				return nil, fmt.Errorf("error")
			}, note)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to create packfile from push note: error"))
		})

		It("should return failed to run git-receive if repo path is invalid", func() {
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().GetPath().Return("invalid/path/to/repo")
			note := &types.Note{TargetRepo: mockRepo}
			buf := strings.NewReader("invalid")
			err := UpdateRepoUsingNote(cfg.Node.GitBinPath, func(tx types.PushNote) (io.ReadSeeker, error) {
				return buf, nil
			}, note)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("git-receive-pack failed to start"))
		})

		It("should return error when generated packfile is invalid", func() {
			note := &types.Note{TargetRepo: repo}
			buf := strings.NewReader("invalid")
			err := UpdateRepoUsingNote(cfg.Node.GitBinPath, func(tx types.PushNote) (io.ReadSeeker, error) {
				return buf, nil
			}, note)
			Expect(err).ToNot(BeNil())
		})

		It("should return nil when packfile is valid", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			note := &types.Note{
				TargetRepo: repo,
				References: []*types.PushedReference{
					{Name: "refs/heads/master", NewHash: commitHash, OldHash: plumbing.ZeroHash.String()},
				},
			}
			packfile, err := push.MakeReferenceUpdateRequestPack(note)
			Expect(err).To(BeNil())
			err = UpdateRepoUsingNote(cfg.Node.GitBinPath, func(tx types.PushNote) (io.ReadSeeker, error) {
				return packfile, nil
			}, note)
			Expect(err).To(BeNil())
		})
	})

	Describe(".Watch", func() {
		It("should add a watcher task", func() {
			rs.Watch("repo1", "refs/heads/master", 10, 100)
			Expect(rs.watcher.QueueSize()).To(Equal(1))
		})
	})
})
