package issuecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/cmd/issuecmd"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
)

func testIssueCommentCreator(isNewIssue bool, issueReference string, err error) func(targetRepo core.BareRepo,
	issueID int, issueBody string, isComment bool) (bool, string, error) {
	return func(targetRepo core.BareRepo, issueID int, issueBody string, isComment bool) (bool, string, error) {
		return isNewIssue, issueReference, err
	}
}

var noopIssueCommentCreator = testIssueCommentCreator(false, "", nil)
var errorIssueCommentCreator = testIssueCommentCreator(false, "", fmt.Errorf("error"))

func mockReadFunc(data []byte, err error) func(b []byte) (int, error) {
	return func(b []byte) (int, error) {
		copy(b, data[:])
		return len(data), err
	}
}

var _ = Describe("IssueCreate", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockBareRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockBareRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".IssueCreateCmd", func() {
		When("issue number is unset (new issue creation)", func() {
			It("should return error when a reaction is unknown", func() {
				args := &issuecmd.IssueCreateArgs{Reactions: []string{":unknown:"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("reaction (:unknown:) is not supported"))
			})

			It("should return error when a label is not valid", func() {
				args := &issuecmd.IssueCreateArgs{Labels: []string{"*la&bel"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("label (*la&bel) is not valid"))
			})

			It("should return error when a assignee is not valid", func() {
				args := &issuecmd.IssueCreateArgs{Assignees: []string{"*assign&ee"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("assignee (*assign&ee) is not a valid push key address"))
			})

			It("should return error when a fixer is not valid", func() {
				args := &issuecmd.IssueCreateArgs{Fixers: []string{"*fix&er"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("fixer (*fix&er) is not a valid push key address"))
			})

			It("should return error when reply hash is set but issue number is not set", func() {
				args := &issuecmd.IssueCreateArgs{ReplyHash: "02we"}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue number is required when adding a comment"))
			})

			When("title is not set AND reply hash is not set AND issues did not previously exist", func() {
				It("should read title and body from stdIn", func() {
					mockStdIn := mocks.NewMockReadCloser(ctrl)
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my body"), io.EOF))
					args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, IssueCommentCreator: noopIssueCommentCreator}
					err := issuecmd.IssueCreateCmd(mockRepo, args)
					Expect(err).To(BeNil())
					Expect(args.Title).To(Equal("my title"))
					Expect(args.Body).To(Equal("my body"))
				})
			})

			It("should return error when title is not provided from stdin", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, nil))
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, io.EOF))
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(issuecmd.ErrTitleRequired))
			})

			It("should return error when body is not provided from stdin", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, io.EOF))
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(issuecmd.ErrBodyRequired))
			})

			It("should return error when body is not provided from stdin even is NoBody is true", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, NoBody: true}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(issuecmd.ErrBodyRequired))
			})

			When("custom editor is requested", func() {
				var args *issuecmd.IssueCreateArgs
				BeforeEach(func() {
					mockStdIn := mocks.NewMockReadCloser(ctrl)
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
					args = &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, UseEditor: true}
				})

				It("should request fetch core.editor from git config", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					issuecmd.IssueCreateCmd(mockRepo, args)
				})

				It("should use custom editor program is EditorPath is set", func() {
					args.EditorPath = "myeditor"
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						Expect(editor).To(Equal(args.EditorPath))
						return "", nil
					}
					issuecmd.IssueCreateCmd(mockRepo, args)
				})

				It("should return error if reading from editor failed", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						return "", fmt.Errorf("error")
					}
					err := issuecmd.IssueCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("failed read body from editor: error"))
				})

				It("should return error when body is unset through editor", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					err := issuecmd.IssueCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(Equal(issuecmd.ErrBodyRequired))
				})
			})

		})

		When("issue number is set (new issue creation or comment)", func() {
			It("should return error when issue does not exist and reply hash is set", func() {
				args := &issuecmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("", repo.ErrRefNotFound)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue (1) was not found"))
			})

			It("should return error when unable to get issue reference", func() {
				args := &issuecmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when unable to count number of comments in reference", func() {
				args := &issuecmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(0, fmt.Errorf("error"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to count comments in issue: error"))
			})

			It("should return error when issue has commits and title is provided", func() {
				args := &issuecmd.IssueCreateArgs{IssueNumber: 1, Title: "Some Title"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("title not required when adding a comment to an issue"))
			})

			It("should return error when reply hash does not exist in issue branch", func() {
				args := &issuecmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "reply_hash"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("reply_hash", "xyz").Return(fmt.Errorf("bad"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target comment hash (reply_hash) is unknown"))
			})

			It("should not return ErrBodyRequired when NoBody=true and intent is a reply", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)

				issueNumber := 1
				ref := plumbing.MakeIssueReference(issueNumber)
				args := &issuecmd.IssueCreateArgs{IssueNumber: issueNumber, ReplyHash: "comment_hash", NoBody: true,
					StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn,
					IssueCommentCreator: testIssueCommentCreator(true, ref, nil)}

				mockRepo.EXPECT().RefGet(ref).Return("ref_hash", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("comment_hash", "ref_hash").Return(nil)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should return error when unable to create issue/comment", func() {
			mockStdIn := mocks.NewMockReadCloser(ctrl)
			mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
			mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my body"), io.EOF))
			args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, IssueCommentCreator: errorIssueCommentCreator}
			err := issuecmd.IssueCreateCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(args.Title).To(Equal("my title"))
			Expect(args.Body).To(Equal("my body"))
			Expect(err).To(MatchError("failed to add issue or comment: error"))
		})
	})
})
