package issuecmd_test

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/issuecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IssueReadCmd", func() {
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

	Describe(".IssueReadCmd", func() {
		It("should return err when unable to find the issue", func() {
			args := &issuecmd.IssueReadArgs{
				Reference: plumbing2.MakeIssueReference(1),
				PostGetter: func(plumbing2.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			_, err := issuecmd.IssueReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to find issue: error"))
		})

		It("should return err when issue was not found", func() {
			args := &issuecmd.IssueReadArgs{
				Reference: plumbing2.MakeIssueReference(1),
				PostGetter: func(plumbing2.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return plumbing2.Posts{}, nil
				},
			}
			_, err := issuecmd.IssueReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue not found"))
		})

		It("should return err when unable to check `close` status of issue", func() {
			issuePath := plumbing2.MakeIssueReference(1)
			args := &issuecmd.IssueReadArgs{
				Reference: issuePath,
				PostGetter: func(plumbing2.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					post := mocks.NewMockPostEntry(ctrl)
					post.EXPECT().IsClosed().Return(false, fmt.Errorf("error"))
					return plumbing2.Posts{post}, nil
				},
			}
			_, err := issuecmd.IssueReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check close status: error"))
		})

		It("should return err when unable to get comments", func() {
			issuePath := plumbing2.MakeIssueReference(1)
			args := &issuecmd.IssueReadArgs{
				Reference: issuePath,
				PostGetter: func(plumbing2.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					post := mocks.NewMockPostEntry(ctrl)
					post.EXPECT().IsClosed().Return(false, nil)
					post.EXPECT().GetComments().Return(nil, fmt.Errorf("error"))
					return plumbing2.Posts{post}, nil
				},
			}
			_, err := issuecmd.IssueReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get comments: error"))
		})
	})
})
