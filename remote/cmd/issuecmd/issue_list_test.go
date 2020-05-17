package issuecmd_test

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
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/cmd/issuecmd"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
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
				PostGetter: func(core.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return nil, fmt.Errorf("error")
				},
			}
			err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get issue posts: error"))
		})

		It("should sort issue posts by latest", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []*plumbing2.Post{
				{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &issuecmd.IssueListArgs{
				StdErr: out, StdOut: out,
				Format: "%H%",
				PostGetter: func(core.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(Equal([]string{hash2, hash1}))
		})

		It("should reverse issue when Reverse=true", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []*plumbing2.Post{
				{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &issuecmd.IssueListArgs{
				StdErr: out, StdOut: out,
				Format:  "%H%",
				Reverse: true,
				PostGetter: func(core.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(Equal([]string{hash1, hash2}))
		})

		It("should limit issue when Limit=1", func() {
			hash1 := util.RandString(40)
			hash2 := util.RandString(40)
			posts := []*plumbing2.Post{
				{
					Title: "How to open a file",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-10 * time.Second),
						Hash:    hash1,
					},
				},
				{
					Title: "Remove examples",
					First: &plumbing2.Comment{
						Body:    &plumbing2.IssueBody{},
						Created: time.Now().Add(-5 * time.Second),
						Hash:    hash2,
					},
				},
			}
			out := bytes.NewBuffer(nil)
			args := &issuecmd.IssueListArgs{
				StdErr: out, StdOut: out,
				Format: "%H%",
				Limit:  1,
				PostGetter: func(core.LocalRepo, func(ref plumbing.ReferenceName) bool) (plumbing2.Posts, error) {
					return posts, nil
				},
				PagerWrite: func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
					out.ReadFrom(content)
					Expect(pagerCmd).To(Equal("pager_program"))
				},
			}
			mockRepo.EXPECT().Var("GIT_PAGER").Return("pager_program", nil)
			err := issuecmd.IssueListCmd(mockRepo, args)
			Expect(err).To(BeNil())
			Expect(strings.Fields(out.String())).To(HaveLen(1))
			Expect(strings.Fields(out.String())).To(Equal([]string{hash2}))
		})
	})
})
