package mergecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/mergecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	plumbing3 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeRequestList", func() {
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

	Describe(".MergeRequestListCmd", func() {
		It("should return err when unable to fetch merge requests", func() {
			args := &mergecmd.MergeRequestListArgs{
				PostGetter: func(plumbing3.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing3.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			_, err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get merge requests posts: error"))
		})

		It("should sort merge requests posts by latest", func() {
			posts := []plumbing3.PostEntry{
				&plumbing3.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      util.RandString(40),
					},
				},
				&plumbing3.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      util.RandString(40),
					},
				},
			}
			args := &mergecmd.MergeRequestListArgs{
				PostGetter: func(plumbing3.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing3.Posts, error) {
					return posts, nil
				},
			}
			res, err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(2))
			Expect(res[0].GetName()).To(Equal("b"))
			Expect(res[1].GetName()).To(Equal("a"))
		})

		It("should reverse merge requests posts when Reverse=true", func() {
			posts := []plumbing3.PostEntry{
				&plumbing3.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      util.RandString(40),
					},
				},
				&plumbing3.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      util.RandString(40),
					},
				},
			}
			args := &mergecmd.MergeRequestListArgs{
				Reverse: true,
				PostGetter: func(plumbing3.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing3.Posts, error) {
					return posts, nil
				},
			}
			res, err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(2))
			Expect(res[0].GetName()).To(Equal("a"))
			Expect(res[1].GetName()).To(Equal("b"))
		})

		It("should limit returned merge requests posts when Limit=1", func() {
			posts := []plumbing3.PostEntry{
				&plumbing3.Post{
					Name:  "a",
					Title: "How to open a file",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      util.RandString(40),
					},
				},
				&plumbing3.Post{
					Name:  "b",
					Title: "Remove examples",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      util.RandString(40),
					},
				},
			}
			args := &mergecmd.MergeRequestListArgs{
				Limit: 1,
				PostGetter: func(plumbing3.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing3.Posts, error) {
					return posts, nil
				},
			}
			res, err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(1))
			Expect(res[0].GetName()).To(Equal("b"))
		})
	})

	Describe(".FormatAndPrintMergeRequestList", func() {
		It("should write to output", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing3.PostEntry{
				&plumbing3.Post{
					Title: "How to open a file",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-10 * time.Second),
						Hash:      hash1,
					},
				},
				&plumbing3.Post{
					Title: "Remove examples",
					Comment: &plumbing3.Comment{
						Body:      plumbing3.NewEmptyPostBody(),
						CreatedAt: time.Now().Add(-5 * time.Second),
						Hash:      hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &mergecmd.MergeRequestListArgs{
				StdErr: out, StdOut: out,
				Format: "%H",
				PostGetter: func(plumbing3.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing3.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					_, _ = out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err = mergecmd.FormatAndPrintMergeRequestList(mockRepo, args, posts)
			Expect(err).To(BeNil())
			Expect(out.Len()).To(BeNumerically(">", 0))
		})
	})
})
