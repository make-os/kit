package cmd_test

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
	"gitlab.com/makeos/mosdef/remote/cmd"
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

var _ = Describe("Issue", func() {
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
				args := &cmd.IssueCreateArgs{Reactions: []string{":unknown:"}}
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("reaction (:unknown:) is not supported"))
			})

			It("should return error when reply hash is set but issue number is not set", func() {
				args := &cmd.IssueCreateArgs{ReplyHash: "02we"}
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue number is required when adding a comment"))
			})

			When("title is not set AND reply hash is not set AND issues did not previously exist", func() {
				It("should read title and body from stdIn", func() {
					mockStdIn := mocks.NewMockReadCloser(ctrl)
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my body"), io.EOF))
					args := &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, IssueCommentCreator: noopIssueCommentCreator}
					err := cmd.IssueCreateCmd(mockRepo, args)
					Expect(err).To(BeNil())
					Expect(args.Title).To(Equal("my title"))
					Expect(args.Body).To(Equal("my body"))
				})
			})

			It("should return error when title is not provided from stdin", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, nil))
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, io.EOF))
				args := &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn}
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(cmd.ErrTitleRequired))
			})

			It("should return error when body is not provided from stdin", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc(nil, io.EOF))
				args := &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn}
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(cmd.ErrBodyRequired))
			})

			It("should return error when body is not provided from stdin even is NoBody is true", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)
				mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
				args := &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, NoBody: true}
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(cmd.ErrBodyRequired))
			})

			When("custom editor is requested", func() {
				var args *cmd.IssueCreateArgs
				BeforeEach(func() {
					mockStdIn := mocks.NewMockReadCloser(ctrl)
					mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
					args = &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, UseEditor: true}
				})

				It("should request fetch core.editor from git config", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					cmd.IssueCreateCmd(mockRepo, args)
				})

				It("should use custom editor program is EditorPath is set", func() {
					args.EditorPath = "myeditor"
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						Expect(editor).To(Equal(args.EditorPath))
						return "", nil
					}
					cmd.IssueCreateCmd(mockRepo, args)
				})

				It("should return error if reading from editor failed", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						return "", fmt.Errorf("error")
					}
					err := cmd.IssueCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("failed read body from editor: error"))
				})

				It("should return error when body is unset through editor", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					err := cmd.IssueCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(Equal(cmd.ErrBodyRequired))
				})
			})

		})

		When("issue number is set (new issue creation or comment)", func() {
			It("should return error when issue does not exist and reply hash is set", func() {
				args := &cmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("", repo.ErrRefNotFound)
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue (1) was not found"))
			})

			It("should return error when unable to get issue reference", func() {
				args := &cmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when unable to count number of comments in reference", func() {
				args := &cmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(0, fmt.Errorf("error"))
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to count comments in issue: error"))
			})

			It("should return error when issue has commits and title is provided", func() {
				args := &cmd.IssueCreateArgs{IssueNumber: 1, Title: "Some Title"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("title not required when adding a comment to an issue"))
			})

			It("should return error when reply hash does not exist in issue branch", func() {
				args := &cmd.IssueCreateArgs{IssueNumber: 1, ReplyHash: "reply_hash"}
				ref := plumbing.MakeIssueReference(args.IssueNumber)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("reply_hash", "xyz").Return(fmt.Errorf("bad"))
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target comment hash (reply_hash) is unknown"))
			})

			It("should not return ErrBodyRequired when NoBody=true and intent is a reply", func() {
				mockStdIn := mocks.NewMockReadCloser(ctrl)

				issueNumber := 1
				ref := plumbing.MakeIssueReference(issueNumber)
				args := &cmd.IssueCreateArgs{IssueNumber: issueNumber, ReplyHash: "comment_hash", NoBody: true,
					StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn,
					IssueCommentCreator: testIssueCommentCreator(true, ref, nil)}

				mockRepo.EXPECT().RefGet(ref).Return("ref_hash", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("comment_hash", "ref_hash").Return(nil)
				err := cmd.IssueCreateCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should return error when unable to create issue/comment", func() {
			mockStdIn := mocks.NewMockReadCloser(ctrl)
			mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my title\n"), nil))
			mockStdIn.EXPECT().Read(gomock.Any()).DoAndReturn(mockReadFunc([]byte("my body"), io.EOF))
			args := &cmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), StdIn: mockStdIn, IssueCommentCreator: errorIssueCommentCreator}
			err := cmd.IssueCreateCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(args.Title).To(Equal("my title"))
			Expect(args.Body).To(Equal("my body"))
			Expect(err).To(MatchError("failed to add issue or comment: error"))
		})
	})
})
