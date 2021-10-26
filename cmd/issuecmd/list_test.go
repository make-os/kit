package issuecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/issuecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IssueList", func() {
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

	Describe(".IssueListCmd", func() {
		It("should return err when unable to fetch issues", func() {
			args := &issuecmd.IssueListArgs{
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			_, err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get issue posts: error"))
		})

		It("should sort issue posts by latest", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      hash1,
					},
				},
				&plumbing2.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      hash2,
					},
				},
			}
			args := &issuecmd.IssueListArgs{
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
			}
			res, err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res[0].GetName()).To(Equal("b"))
			Expect(res[1].GetName()).To(Equal("a"))
		})

		It("should reverse issue when Reverse=true", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      hash1,
					},
				},
				&plumbing2.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      hash2,
					},
				},
			}
			args := &issuecmd.IssueListArgs{
				Reverse: true,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
			}
			res, err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res[0].GetName()).To(Equal("a"))
			Expect(res[1].GetName()).To(Equal("b"))
		})

		It("should limit issue when Limit=1", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      hash1,
					},
				},
				&plumbing2.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      hash2,
					},
				},
			}
			args := &issuecmd.IssueListArgs{
				Limit: 1,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
			}
			res, err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(1))
			Expect(res[0].GetName()).To(Equal("b"))
		})
	})

	Describe(".FormatAndPrintIssueList", func() {
		It("should write to output", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Title: "How to open a file",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      hash1,
					},
				},
				&plumbing2.Post{
					Title: "Remove examples",
					Comment: &plumbing2.Comment{
						Body:      plumbing2.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      hash2,
					},
				},
			}

			out := bytes.NewBuffer(nil)
			args := &issuecmd.IssueListArgs{
				StdErr: out, StdOut: out,
				Format: "%H",
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					_, _ = out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)

			err = issuecmd.FormatAndPrintIssueList(mockRepo, args, posts)
			Expect(err).To(BeNil())
			Expect(out.Len()).To(BeNumerically(">", 0))
		})
	})
})
