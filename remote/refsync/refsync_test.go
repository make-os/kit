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
	repo3 "github.com/make-os/lobe/remote/repo"
	testutil2 "github.com/make-os/lobe/remote/testutil"
	types2 "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/make-os/lobe/remote/refsync"
	. "github.com/onsi/gomega"
)

var _ = Describe("RefSync", func() {
	var err error
	var cfg *config.AppConfig
	var rs RefSyncer
	var ctrl *gomock.Controller
	var mockFetcher *mocks.MockObjectFetcher
	var repoName string
	var oldHash = "5b9ba1de20344b12cce76256b67cff9bb31e77b2"
	var newHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"
	var path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockFetcher = mocks.NewMockObjectFetcher(ctrl)
		rs = New(cfg, mockFetcher, 1)

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

	Describe(".HasTax", func() {
		It("should return false when task queue is empty", func() {
			Expect(rs.QueueSize()).To(BeZero())
			Expect(rs.HasTask()).To(BeFalse())
		})
	})

	Describe(".OnNewTx", func() {
		It("should add new task to queue", func() {
			rs.OnNewTx(&txns.TxPush{Note: &types.Note{
				References: []*types.PushedReference{{Name: "master", Nonce: 1}},
			}})
			Expect(rs.HasTask()).To(BeTrue())
			Expect(rs.QueueSize()).To(Equal(1))
		})

		It("should not add new task to queue when task with matching ID already exist in queue", func() {
			rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}})
			rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{{Name: "master", Nonce: 1}}}})
			Expect(rs.HasTask()).To(BeTrue())
			Expect(rs.QueueSize()).To(Equal(1))
		})

		It("should not add task if reference new hash is zero-hash", func() {
			rs.OnNewTx(&txns.TxPush{Note: &types.Note{References: []*types.PushedReference{
				{Name: "master", Nonce: 1, NewHash: plumbing.ZeroHash.String()},
			}}})
			Expect(rs.HasTask()).To(BeFalse())
			Expect(rs.QueueSize()).To(Equal(0))
		})

		It("should add two tasks if push transaction contains 2 different references", func() {
			rs.OnNewTx(&txns.TxPush{Note: &types.Note{
				References: []*types.PushedReference{
					{Name: "refs/heads/master", Nonce: 1},
					{Name: "refs/heads/dev", Nonce: 1},
				},
			}})
			Expect(rs.HasTask()).To(BeTrue())
			Expect(rs.QueueSize()).To(Equal(2))
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
			task := &Task{Ref: &types.PushedReference{Name: "refs/heads/master"}}
			rs.(*RefSync).FinalizingRefs[task.Ref.Name] = struct{}{}
			err := Do(rs.(*RefSync), task, 0)
			Expect(err).To(BeNil())
			Expect(rs.QueueSize()).To(Equal(1))
		})

		It("should return error when repo does not exist locally", func() {
			task := &Task{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			err := Do(rs.(*RefSync), task, 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get target repo: repository does not exist"))
		})

		It("should return error when unable to get reference from local repo", func() {
			task := &Task{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master"}}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
			mockRepo.EXPECT().RefGet(task.Ref.Name).Return("", fmt.Errorf("error"))
			err := Do(rs.(*RefSync), task, 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get reference from target repo: error"))
		})

		When("local reference hash and task reference old hash do not match", func() {
			It("should set task old hash to the local hash", func() {
				task := &Task{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.ReferenceUpdateRequestPackMaker, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				err := Do(rs.(*RefSync), task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
				Expect(task.Ref.OldHash).To(Equal(oldHash))
			})
		})

		When("local reference hash and task reference old hash match", func() {
			It("should attempt to fetch objects and update repo", func() {
				task := &Task{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.ReferenceUpdateRequestPackMaker, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(nil)
				})
				err := Do(rs.(*RefSync), task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})

			It("should not attempt to update repo and return error if fetch attempt failed", func() {
				task := &Task{RepoName: "unknown", Ref: &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.ReferenceUpdateRequestPackMaker, types.PushNote) error {
					updated = true
					return nil
				}
				mockFetcher.EXPECT().FetchAsync(gomock.Any(), gomock.Any()).Do(func(note types.PushNote, cb func(err error)) {
					Expect(note).ToNot(BeNil())
					cb(fmt.Errorf("error"))
				})
				err := Do(rs.(*RefSync), task, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
				Expect(updated).To(BeFalse())
			})

			It("should update repo without fetching objects if node is the creator of the push note", func() {
				key, _ := cfg.G().PrivVal.GetKey()
				task := &Task{
					RepoName:    "unknown",
					Ref:         &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash},
					NoteCreator: key.PubKey().MustBytes32(),
				}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.ReferenceUpdateRequestPackMaker, types.PushNote) error {
					updated = true
					return nil
				}
				err := Do(rs.(*RefSync), task, 0)
				Expect(err).To(BeNil())
				Expect(updated).To(BeTrue())
			})

			It("should update repo without fetching objects if node is an endorser of the push note", func() {
				key, _ := cfg.G().PrivVal.GetKey()
				task := &Task{
					RepoName: "unknown",
					Ref:      &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash},
					Endorsements: []*types.PushEndorsement{
						{EndorserPubKey: key.PubKey().MustBytes32()},
					},
				}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				rs.(*RefSync).RepoGetter = func(gitBinPath, path string) (types2.LocalRepo, error) { return mockRepo, nil }
				mockRepo.EXPECT().RefGet(task.Ref.Name).Return(oldHash, nil)
				updated := false
				rs.(*RefSync).UpdateRepoUsingNote = func(string, push.ReferenceUpdateRequestPackMaker, types.PushNote) error {
					updated = true
					return nil
				}
				err := Do(rs.(*RefSync), task, 0)
				Expect(err).To(BeNil())
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
