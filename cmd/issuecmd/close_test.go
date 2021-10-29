package issuecmd_test

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/issuecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IssueCreate", func() {
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

	Describe(".IssueCloseCmd", func() {
		It("should return error when unable to get reference", func() {
			ref := plumbing.MakeIssueReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{Reference: ref})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when issue reference does not exist", func() {
			ref := plumbing.MakeIssueReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", plumbing.ErrRefNotFound)
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{Reference: ref})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue not found"))
		})

		It("should return error when unable to read recent commit post body", func() {
			ref := plumbing.MakeIssueReference(1)
			hash := "e31992a88829f3cb70ab5f5e964597a6c8f17047"
			mockRepo.EXPECT().RefGet(ref).Return(hash, nil)
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{
				Reference: ref,
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					return nil, nil, fmt.Errorf("error")
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read recent comment: error"))
		})

		It("should return error when recent commit post body includes a closed=true directive", func() {
			ref := plumbing.MakeIssueReference(1)
			hash := "e31992a88829f3cb70ab5f5e964597a6c8f17047"
			mockRepo.EXPECT().RefGet(ref).Return(hash, nil)
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{
				Reference: ref,
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					closed := true
					return &plumbing.PostBody{Close: &closed}, nil, nil
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("already closed"))
		})

		Specify("that the correct body was created", func() {
			ref := plumbing.MakeIssueReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{
				Reference: ref,
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					return plumbing.NewEmptyPostBody(), nil, nil
				},
				PostCommentCreator: func(r plumbing.LocalRepo, args *plumbing.CreatePostCommitArgs) (isNew bool, reference string, err error) {
					Expect(args.Body).To(Equal("---\nclose: true\n---\n"))
					return false, "", nil
				},
			})
			Expect(err).To(BeNil())
		})

		It("should return error when unable to post comment", func() {
			ref := plumbing.MakeIssueReference(1)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			_, err := issuecmd.IssueCloseCmd(mockRepo, &issuecmd.IssueCloseArgs{
				Reference: ref,
				ReadPostBody: func(repo plumbing.LocalRepo, hash string) (*plumbing.PostBody, *object.Commit, error) {
					return plumbing.NewEmptyPostBody(), nil, nil
				},
				PostCommentCreator: func(r plumbing.LocalRepo, args *plumbing.CreatePostCommitArgs) (isNew bool, reference string, err error) {
					return false, "", fmt.Errorf("error")
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to create or add new close comment: error"))
		})
	})
})
