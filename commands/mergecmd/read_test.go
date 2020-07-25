package mergecmd_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/commands/mergecmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var _ = Describe("MergeRequestRead", func() {
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

	Describe(".MergeRequestReadCmd", func() {
		It("should return err when unable to find the merge request", func() {
			args := &mergecmd.MergeRequestReadArgs{
				Reference: plumbing2.MakeMergeRequestReference(1),
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			err := mergecmd.MergeRequestReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to find merge request: error"))
		})

		It("should return err when merge request was not found", func() {
			args := &mergecmd.MergeRequestReadArgs{
				Reference: plumbing2.MakeMergeRequestReference(1),
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return plumbing2.Posts{}, nil
				},
			}
			err := mergecmd.MergeRequestReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("merge request not found"))
		})

		It("should return err when unable to check `close` status of merge request post", func() {
			mrPath := plumbing2.MakeMergeRequestReference(1)
			args := &mergecmd.MergeRequestReadArgs{
				Reference: mrPath,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					post := mocks.NewMockPostEntry(ctrl)
					post.EXPECT().IsClosed().Return(false, fmt.Errorf("error"))
					return plumbing2.Posts{post}, nil
				},
			}
			err := mergecmd.MergeRequestReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check close status: error"))
		})

		It("should return err when unable to get comments", func() {
			mrPath := plumbing2.MakeMergeRequestReference(1)
			args := &mergecmd.MergeRequestReadArgs{
				Reference: mrPath,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					post := mocks.NewMockPostEntry(ctrl)
					post.EXPECT().IsClosed().Return(false, nil)
					post.EXPECT().GetComments().Return(nil, fmt.Errorf("error"))
					return plumbing2.Posts{post}, nil
				},
			}
			err := mergecmd.MergeRequestReadCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get comments: error"))
		})
	})
})
