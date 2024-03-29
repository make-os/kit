package mergecmd_test

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/mergecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeReqFetchCmd", func() {
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

	Describe(".MergeReqFetchCmd", func() {
		It("should return error when unable to get merge request reference", func() {
			args := &mergecmd.MergeReqFetchArgs{Reference: plumbing.MakeMergeRequestReference(1)}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(nil, fmt.Errorf("error"))
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when merge request reference does not exist", func() {
			args := &mergecmd.MergeReqFetchArgs{Reference: plumbing.MakeMergeRequestReference(1)}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(nil, plumbing.ErrRefNotFound)
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("merge request not found"))
		})

		It("should return error when unable to read a commit in a merge request reference", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqFetchArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return nil, nil, fmt.Errorf("error")
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read commit (4caa628): error"))
		})

		It("should return error when merge request has no target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqFetchArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return plumbing.NewEmptyPostBody(), nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target branch was not set in merge request"))
		})

		It("should return error when merge request has no target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqFetchArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{
						MergeRequestFields: &plumbing.MergeRequestFields{
							TargetBranch: "target",
						},
					}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target branch hash was not set in merge request"))
		})

		It("should return error when unable to fetch target branch", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqFetchArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: &plumbing.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(plumbing.RefFetchArgs{Remote: "origin", RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(fmt.Errorf("error"))
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to fetch target branch: error"))
		})

		It("should return no error fetch succeeds", func() {
			mergeReqCommits := []string{"4caa628d799954fc0bbcf667322719120e2a56ec"}
			args := &mergecmd.MergeReqFetchArgs{
				Reference: plumbing.MakeMergeRequestReference(1),
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					Expect(hash).To(Equal(mergeReqCommits[0]))
					return &plumbing.PostBody{MergeRequestFields: &plumbing.MergeRequestFields{TargetBranch: "target", TargetBranchHash: "hash"}}, nil, nil
				},
			}
			mockRepo.EXPECT().GetRefCommits(args.Reference, true).Return(mergeReqCommits, nil)
			mockRepo.EXPECT().RefFetch(plumbing.RefFetchArgs{Remote: "origin", RemoteRef: "target", LocalRef: "target", Force: false, Verbose: true}).Return(nil)
			err = mergecmd.MergeReqFetchCmd(mockRepo, args)
			Expect(err).To(BeNil())
		})
	})
})
