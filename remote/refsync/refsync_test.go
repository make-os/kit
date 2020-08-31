package refsync_test

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/remote/push"
	"github.com/make-os/lobe/remote/push/types"
	types3 "github.com/make-os/lobe/remote/refsync/types"
	repo3 "github.com/make-os/lobe/remote/repo"
	testutil2 "github.com/make-os/lobe/remote/testutil"
	types2 "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	. "github.com/onsi/ginkgo"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/make-os/lobe/remote/refsync"
	. "github.com/onsi/gomega"
)

var _ = Describe("RefSync", func() {
	var err error
	var cfg *config.AppConfig
	var rs types3.RefSync
	var ctrl *gomock.Controller
	var mockFetcher *mocks.MockObjectFetcher
	var repoName string
	var oldHash = "5b9ba1de20344b12cce76256b67cff9bb31e77b2"
	var newHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"
	var path string
	var mockKeepers *mocks.MockKeepers
	var mockTrackedRepoKeeper *mocks.MockTrackedRepoKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockFetcher = mocks.NewMockObjectFetcher(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockTrackedRepoKeeper = mocks.NewMockTrackedRepoKeeper(ctrl)
		mockKeepers.EXPECT().TrackedRepoKeeper().Return(mockTrackedRepoKeeper).AnyTimes()
		rs = New(cfg, mockFetcher, mockKeepers)

		repoName = util.RandString(5)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
	})

	Describe(".Start", func() {
		It("should panic if called twice", func() {
			rs.Start()
			Expect(func() { rs.Start() }).To(Panic())
		})
	})

	Describe(".IsRunning", func() {
		It("should return false if not running", func() {
			Expect(rs.IsRunning()).To(BeFalse())
		})

		It("should return true if running", func() {
			rs.Start()
			Expect(rs.IsRunning()).To(BeTrue())
		})
	})

	Describe(".Stop", func() {
		It("should set running state to false", func() {
			rs.Start()
			Expect(rs.IsRunning()).To(BeTrue())
			rs.Stop()
			Expect(rs.IsRunning()).To(BeFalse())
		})
	})

	Describe(".HasTask", func() {
		It("should return false when task queue is empty", func() {
			Expect(rs.QueueSize()).To(BeZero())
			Expect(rs.HasTask()).To(BeFalse())
		})
	})

	Describe(".CanSync", func() {
		It("should return nil if no repository is being tracked", func() {
			mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{})
			Expect(rs.CanSync("", "repo1")).To(BeNil())
		})

		It("should return ErrUntracked if target repo is not part of the tracked repositories", func() {
			mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
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
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				tx := &txns.TxPush{Note: &types.Note{Namespace: "ns1", RepoName: "domain"}}
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash(tx.Note.GetNamespace())).Return(&state.Namespace{})
				err := rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("namespace not found"))
			})

			It("should return error if namespace's domain does not exist", func() {
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
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
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
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
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
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

		When("target repo is untracked", func() {
			It("should not add new task to queue", func() {
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
					"repo2": {},
				})
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{RepoName: "repo1"}}, 1)
				Expect(rs.HasTask()).To(BeFalse())
				Expect(rs.QueueSize()).To(Equal(0))
			})
		})

		When("there are  no tracked repositories", func() {
			BeforeEach(func() {
				mockTrackedRepoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{}).AnyTimes()
			})

			It("should add new task to queue", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{
					References: []*types.PushedReference{{Name: "master", Nonce: 1}},
				}}, 1)
				Expect(rs.HasTask()).To(BeTrue())
				Expect(rs.QueueSize()).To(Equal(1))
			})

			It("should not add new task to queue when task with matching ID already exist in queue", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, 1)
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}}, 1)
				Expect(rs.HasTask()).To(BeTrue())
				Expect(rs.QueueSize()).To(Equal(1))
			})

			It("should not add task if reference new hash is zero-hash", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{
					{Name: "master", Nonce: 1, NewHash: plumbing.ZeroHash.String()},
				}}}, 1)
				Expect(rs.HasTask()).To(BeFalse())
				Expect(rs.QueueSize()).To(Equal(0))
			})

			It("should add two tasks if push transaction contains 2 different references", func() {
				rs.OnNewTx(&txns.TxPush{Note: &types.Note{
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1},
						{Name: "refs/heads/dev", Nonce: 1},
					},
				}}, 1)
				Expect(rs.HasTask()).To(BeTrue())
				Expect(rs.QueueSize()).To(Equal(2))
			})
		})
	})

	Describe(".Start", func() {
		It("should set the status to start", func() {
			rs.Start()
			Expect(rs.IsRunning()).To(BeTrue())
			rs.Stop()
		})
	})

	Describe(".Do", func() {
		It("should append task back to queue when another task with matching reference name is being finalized", func() {
			task := &types3.RefTask{Ref: &types.PushedReference{Name: "refs/heads/master"}}
			rs.(*RefSync).FinalizingRefs[task.Ref.Name] = struct{}{}
			err := rs.Do(task, 0)
			Expect(err).To(BeNil())
			Expect(rs.QueueSize()).To(Equal(1))
		})

		It("should return error when repo does not exist locally", func() {
			task := &types3.RefTask{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			err := rs.Do(task, 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get target repo: repository does not exist"))
		})

		It("should return error when unable to get reference from local repo", func() {
			task := &types3.RefTask{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
			mockRepo.EXPECT().RefGet(task.Ref.Name).Return("", fmt.Errorf("error"))
			err := rs.Do(task, 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get reference from target repo: error"))
		})

		When("local reference hash and task reference old hash do not match", func() {
			It("should set task old hash to the local hash", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(nil)
				err := rs.Do(task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
				Expect(task.Ref.OldHash).To(Equal(oldHash))
			})
		})

		When("local reference hash and task reference old hash match", func() {
			It("should attempt to fetch objects and update repo", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(nil)
				err := rs.Do(task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})

			It("should not attempt to update repo and return error if fetch attempt failed", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(fmt.Errorf("error"))
				})
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(nil)
				err := rs.Do(task, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
				Expect(updated).To(BeFalse())
			})

			It("should update repo without fetching objects if node is the creator of the push note", func() {
				key, _ := cfg.G().PrivVal.GetKey()
				task := &types3.RefTask{
					RepoName:    "repo1",
					Ref:         &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash},
					NoteCreator: key.PubKey().MustBytes32(),
				}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(nil)
				err := rs.Do(task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})

			It("should update repo without fetching objects if node is an endorser of the push note", func() {
				key, _ := cfg.G().PrivVal.GetKey()
				task := &types3.RefTask{
					RepoName: "repo1",
					Ref:      &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash},
					Endorsements: []*types.PushEndorsement{
						{EndorserPubKey: key.PubKey().MustBytes32()},
					},
				}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(nil)
				err := rs.Do(task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})
		})

		When("target repo is tracked", func() {
			It("should return error when unable to update tracked repo update height", func() {
				task := &types3.RefTask{RepoName: "repo1", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}, Height: 10}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.MakeReferenceUpdateRequestPackFunc, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				mockTrackedRepoKeeper.EXPECT().Get(task.RepoName).Return(&core.TrackedRepo{})
				mockTrackedRepoKeeper.EXPECT().Add(task.RepoName, uint64(task.Height)).Return(fmt.Errorf("error"))
				err := rs.Do(task, 0)
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
			Expect(err.Error()).To(MatchRegexp("failed to start git-receive-pack command"))
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
})
