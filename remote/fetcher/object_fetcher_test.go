package fetcher_test

import (
	"bytes"
	"context"
	"fmt"
	io2 "io"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	types2 "github.com/make-os/lobe/dht/streamer/types"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/remote/fetcher"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/push/types"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util/io"
	. "github.com/onsi/ginkgo"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/gomega"
)

var _ = Describe("ObjectFetcher", func() {
	var err error
	var cfg *config.AppConfig
	var f *fetcher.BasicObjectFetcher
	var mockDHT *mocks.MockDHT
	var ctrl *gomock.Controller
	var mockObjStreamer *mocks.MockObjectStreamer

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockDHT = mocks.NewMockDHT(ctrl)
		mockObjStreamer = mocks.NewMockObjectStreamer(ctrl)
		f = fetcher.NewFetcher(mockDHT, 1, cfg)
	})

	AfterEach(func() {
		f.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".FetchAsync", func() {
		It("should add task", func() {
			f.FetchAsync(&types.Note{}, func(err error) {})
			Expect(f.IsQueueEmpty()).To(BeFalse())
		})
	})

	Describe(".Start", func() {
		It("should panic if start is called twice", func() {
			f.Start()
			Expect(func() {
				f.Start()
			}).To(Panic())
		})
	})

	Describe(".Operation", func() {
		var oldHash = "5b9ba1de20344b12cce76256b67cff9bb31e77b2"
		var newHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"

		It("should return nil when there are no pushed reference in the task", func() {
			mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
			note := &types.Note{References: []*types.PushedReference{}}
			task := fetcher.NewTask(note, func(err error) {})
			err := f.Operation(task)
			Expect(err).To(BeNil())
		})

		When("pushed reference is a branch", func() {
			It("should return error when unable to get object from the object streamer", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}},
				}

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockF := mockObjStreamer.EXPECT().GetCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.RepoName).To(Equal(note.RepoName))
					Expect(args.StartHash).To(Equal(plumbing.HashToBytes(newHash)))
					Expect(args.ExcludeEndCommit).To(BeTrue())
					Expect(args.GitBinPath).To(Equal(cfg.Node.GitBinPath))
					Expect(args.ReposDir).To(Equal(cfg.GetRepoRoot()))
					Expect(args.EndHash).To(Equal(plumbing.HashToBytes(oldHash)))
					return nil, fmt.Errorf("error")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when the object streamer result callback failed to "+
				"write packfile to repo due to receiving a bad packfile", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error {
					return fmt.Errorf("bad packfile")
				}

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockF := mockObjStreamer.EXPECT().GetCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.RepoName).To(Equal(note.RepoName))
					Expect(args.StartHash).To(Equal(plumbing.HashToBytes(newHash)))
					Expect(args.ExcludeEndCommit).To(BeTrue())
					Expect(args.GitBinPath).To(Equal(cfg.Node.GitBinPath))
					Expect(args.ReposDir).To(Equal(cfg.GetRepoRoot()))
					Expect(args.EndHash).To(Equal(plumbing.HashToBytes(oldHash)))
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad packfile"))
			})

			It("should call fetcher 'fetched' callback when streamer result callback successfully wrote packfile to repo", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/heads/master", OldHash: oldHash, NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error { return nil }

				fetchedCalled := false
				f.OnPackReceived(func(s string, seeker io2.ReadSeeker) { fetchedCalled = true })

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockF := mockObjStreamer.EXPECT().GetCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.RepoName).To(Equal(note.RepoName))
					Expect(args.StartHash).To(Equal(plumbing.HashToBytes(newHash)))
					Expect(args.ExcludeEndCommit).To(BeTrue())
					Expect(args.GitBinPath).To(Equal(cfg.Node.GitBinPath))
					Expect(args.ReposDir).To(Equal(cfg.GetRepoRoot()))
					Expect(args.EndHash).To(Equal(plumbing.HashToBytes(oldHash)))
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).To(BeNil())
				Expect(fetchedCalled).To(BeTrue())
			})

			It("should not set EndHash when pushed reference old hash is zero value", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/heads/master", OldHash: plumbing2.ZeroHash.String(), NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error { return nil }

				fetchedCalled := false
				f.OnPackReceived(func(s string, seeker io2.ReadSeeker) { fetchedCalled = true })

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockF := mockObjStreamer.EXPECT().GetCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.EndHash).To(BeEmpty())
					Expect(args.RepoName).To(Equal(note.RepoName))
					Expect(args.StartHash).To(Equal(plumbing.HashToBytes(newHash)))
					Expect(args.ExcludeEndCommit).To(BeTrue())
					Expect(args.GitBinPath).To(Equal(cfg.Node.GitBinPath))
					Expect(args.ReposDir).To(Equal(cfg.GetRepoRoot()))
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).To(BeNil())
				Expect(fetchedCalled).To(BeTrue())
			})
		})

		When("pushed reference is a tag", func() {
			It("should return error when unable to get old hash of reference from the local repo", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
				}

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(nil, fmt.Errorf("error"))
				note.SetTargetRepo(mockRepo)

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			When("end tag target type is not commit or tag", func() {
				It("should return not set EndHash argument to object streamer", func() {
					note := &types.Note{
						RepoName:   "repo1",
						References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
					}

					mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
					mockRepo := mocks.NewMockLocalRepo(ctrl)
					endTag := &object.Tag{TargetType: plumbing2.BlobObject}
					mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(endTag, nil)
					note.SetTargetRepo(mockRepo)
					mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
					mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
						Expect(args.RepoName).To(Equal(note.RepoName))
						Expect(args.StartHash).To(Equal(plumbing.HashToBytes(newHash)))
						Expect(args.ExcludeEndCommit).To(BeTrue())
						Expect(args.GitBinPath).To(Equal(cfg.Node.GitBinPath))
						Expect(args.ReposDir).To(Equal(cfg.GetRepoRoot()))
						Expect(args.EndHash).To(BeEmpty())
						return nil, nil
					})

					task := fetcher.NewTask(note, func(err error) {})
					err := f.Operation(task)
					Expect(err).To(BeNil())
				})
			})

			When("end tag (A) target type is a tag (B) and (B) points to (C - a commit)", func() {
				It("should fetch the target tag (B) and attempt to use its target (C) as the EndHash", func() {
					note := &types.Note{
						RepoName:   "repo1",
						References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
					}

					mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
					mockRepo := mocks.NewMockLocalRepo(ctrl)

					targetHash := plumbing2.NewHash("1bd66e9881639ea2fd73e6a76a7101151a3dd80c")
					commitHash := plumbing2.NewHash("910476d3115b7ef8ca69b060760db7dd6fb7bd74")
					endTag := &object.Tag{TargetType: plumbing2.TagObject, Target: targetHash}
					endTagParent := &object.Tag{TargetType: plumbing2.CommitObject, Target: commitHash}

					mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(endTag, nil)
					mockRepo.EXPECT().TagObject(targetHash).Return(endTagParent, nil)
					note.SetTargetRepo(mockRepo)
					mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
					mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
						Expect(args.EndHash).To(Equal(commitHash[:]))
						return nil, nil
					})

					task := fetcher.NewTask(note, func(err error) {})
					err := f.Operation(task)
					Expect(err).To(BeNil())
				})
			})

			It("should return error when unable to get object from the object streamer", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
				}

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				commitHash := plumbing2.NewHash("910476d3115b7ef8ca69b060760db7dd6fb7bd74")
				endTag := &object.Tag{TargetType: plumbing2.CommitObject, Target: commitHash}
				mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(endTag, nil)
				note.SetTargetRepo(mockRepo)

				mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.EndHash).To(Equal(commitHash[:]))
					return nil, fmt.Errorf("error")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when the object streamer result callback failed to "+
				"write packfile to repo due to receiving a bad packfile", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error {
					return fmt.Errorf("bad packfile")
				}

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				commitHash := plumbing2.NewHash("910476d3115b7ef8ca69b060760db7dd6fb7bd74")
				endTag := &object.Tag{TargetType: plumbing2.CommitObject, Target: commitHash}
				mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(endTag, nil)
				note.SetTargetRepo(mockRepo)

				mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.EndHash).To(Equal(commitHash[:]))
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad packfile"))
			})

			It("should call fetcher 'fetched' callback when streamer result callback successfully wrote packfile to repo", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: oldHash, NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error { return nil }

				fetchedCalled := false
				f.OnPackReceived(func(s string, seeker io2.ReadSeeker) { fetchedCalled = true })

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				commitHash := plumbing2.NewHash("910476d3115b7ef8ca69b060760db7dd6fb7bd74")
				endTag := &object.Tag{TargetType: plumbing2.CommitObject, Target: commitHash}
				mockRepo.EXPECT().TagObject(plumbing2.NewHash(oldHash)).Return(endTag, nil)
				note.SetTargetRepo(mockRepo)

				mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.EndHash).To(Equal(commitHash[:]))
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).To(BeNil())
				Expect(fetchedCalled).To(BeTrue())
			})

			It("should not set EndHash when pushed reference old hash is zero value", func() {
				note := &types.Note{
					RepoName:   "repo1",
					References: []*types.PushedReference{{Name: "refs/tags/v1.0", OldHash: plumbing2.ZeroHash.String(), NewHash: newHash}},
				}

				f.PackToRepoUnpacker = func(repo remotetypes.LocalRepo, pack io.ReadSeekerCloser) error { return nil }

				fetchedCalled := false
				f.OnPackReceived(func(s string, seeker io2.ReadSeeker) { fetchedCalled = true })

				mockDHT.EXPECT().ObjectStreamer().Return(mockObjStreamer)
				mockF := mockObjStreamer.EXPECT().GetTaggedCommitWithAncestors(gomock.Any(), gomock.Any())
				mockF.DoAndReturn(func(ctx context.Context, args types2.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
					Expect(args.EndHash).To(BeEmpty())
					return nil, args.ResultCB(testutil.WrapReadSeekerCloser{Rdr: bytes.NewBuffer(nil)}, "")
				})

				task := fetcher.NewTask(note, func(err error) {})
				err := f.Operation(task)
				Expect(err).To(BeNil())
				Expect(fetchedCalled).To(BeTrue())
			})
		})
	})
})
