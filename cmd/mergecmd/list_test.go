package mergecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/cmd/mergecmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	"github.com/themakeos/lobe/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
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
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get merge requests posts: error"))
		})

		It("should sort merge requests posts by latest", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				&plumbing2.Post{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &mergecmd.MergeRequestListArgs{
				StdErr: out, StdOut: out,
				Format: "%H",
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(Equal([]string{hash2, hash1}))
		})

		It("should reverse merge requests posts when Reverse=true", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				&plumbing2.Post{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &mergecmd.MergeRequestListArgs{
				StdErr: out, StdOut: out,
				Format:  "%H",
				Reverse: true,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(Equal([]string{hash1, hash2}))
		})

		It("should limit returned merge requests posts when Limit=1", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []plumbing2.PostEntry{
				&plumbing2.Post{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				&plumbing2.Post{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.PostBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &mergecmd.MergeRequestListArgs{
				StdErr: out, StdOut: out,
				Format: "%H",
				Limit:  1,
				PostGetter: func(types.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := mergecmd.MergeRequestListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(HaveLen(1))
			Expect(strings.Fields(out.String())).To(Equal([]string{hash2}))
		})
	})
})
