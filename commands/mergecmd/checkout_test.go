package mergecmd_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/commands/mergecmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("MergeReqCheckoutCmd", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MergeReqCheckoutCmd", func() {
		It("should return error when unable to get merge request reference", func() {
			args := &mergecmd.MergeReqCheckoutArgs{Reference: plumbing.MakeMergeRequestReference(1)}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(nil, fmt.Errorf("error"))
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when merge request reference does not exist", func() {
			args := &mergecmd.MergeReqCheckoutArgs{Reference: plumbing.MakeMergeRequestReference(1)}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(nil, plumbing.ErrRefNotFound)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("merge request not found"))
		})

		It("should return error when unable to read a commit in a merge request reference", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return nil, nil, fmt.Errorf("error")
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read commit (4caa628): error"))
		})

		It("should return error when merge request has no target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target branch was not set in merge request"))
		})

		It("should return error when merge request has no target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{
						MergeRequestFields: types.MergeRequestFields{
							TargetBranch: "target",
						},
					}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target branch hash was not set in merge request"))
		})

		It("should return error when unable to fetch target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin", RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(fmt.Errorf("error"))
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to fetch target branch: error"))
		})

		It("should return error when unable to get fetched branch in repo", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin", RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(nil)
			mockRepo.EXPECT().RefGet("target").Return("", fmt.Errorf("error"))
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should checkout target branch if merge request target and local target branch have same tip hash", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin", RemoteRef: "target",
				LocalRef: "target", Force: false, Verbose: true}).Return(nil)
			mockRepo.EXPECT().RefGet("target").Return("hash", nil)
			mockRepo.EXPECT().Checkout("target", false, args.ForceCheckout)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).To(BeNil())
		})

		When("Base=true", func() {
			It("should checkout target base branch if merge request base target and local base branch have same tip hash", func() {
				mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
				args := &mergecmd.MergeReqCheckoutArgs{
					Base:      true,
					Reference: plumbing.MakeMergeRequestReference(1),
					ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
						Expect(hash).To(Equal(mergeReqCommits[0]))
						return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{BaseBranch: "base", BaseBranchHash: "hash"}}, nil, nil
					},
				}
				mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
				mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin",
					RemoteRef: "base", LocalRef: "base", Force: false, Verbose: true}).Return(nil)
				mockRepo.EXPECT().RefGet("base").Return("hash", nil)
				mockRepo.EXPECT().Checkout("base", false, args.ForceCheckout)
				err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should prompt user to confirm checkout, if merge request target and local target branch have different tip hash", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqCheckoutArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
				ConfirmInput: func(title string, def bool) bool {
					return true
				},
				StdOut: ioutil.Discard,
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin",
				RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(nil)
			mockRepo.EXPECT().RefGet("target").Return("hash2", nil)
			mockRepo.EXPECT().Checkout("target", false, args.ForceCheckout)
			err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
			Expect(err).To(BeNil())
		})

		When("checkout confirmation returns false", func() {
			It("should return error and no checkout happens", func() {
				mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
				args := &mergecmd.MergeReqCheckoutArgs{
					Reference: plumbing.MakeMergeRequestReference(1),
					ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
						Expect(hash).To(Equal(mergeReqCommits[0]))
						return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
					},
					ConfirmInput: func(title string, def bool) bool { return false },
					StdOut:       ioutil.Discard,
				}
				mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
				mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin",
					RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(nil)
				mockRepo.EXPECT().RefGet("target").Return("hash2", nil)
				err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("aborted"))
			})
		})

		When("YesCheckoutDiffTarget=true and merge request target and local target have different hash", func() {
			It("should skip to checkout and not ask for confirmation", func() {
				mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
				args := &mergecmd.MergeReqCheckoutArgs{
					YesCheckoutDiffTarget: true,
					Reference:             plumbing.MakeMergeRequestReference(1),
					ReadPostBody: func(repo types.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
						Expect(hash).To(Equal(mergeReqCommits[0]))
						return &plumbing.PostBody{MergeRequestFields: types.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
					},
				}
				mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
				mockRepo.EXPECT().RefFetch(types.RefFetchArgs{Remote: "origin", RemoteRef: "target",
					LocalRef: "target", Force: false, Verbose: true}).Return(nil)
				mockRepo.EXPECT().RefGet("target").Return("hash2", nil)
				mockRepo.EXPECT().Checkout("target", false, args.ForceCheckout)
				err = mergecmd.MergeReqCheckoutCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})
})
