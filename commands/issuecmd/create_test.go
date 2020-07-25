package issuecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/commands/issuecmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	io2 "github.com/themakeos/lobe/util/io"
)

func testPostCommentCreator(isNewIssue bool, issueReference string, err error) func(targetRepo types.LocalRepo,
	args *plumbing.CreatePostCommitArgs) (bool, string, error) {
	return func(targetRepo types.LocalRepo, args *plumbing.CreatePostCommitArgs) (bool, string, error) {
		return isNewIssue, issueReference, err
	}
}

var noopPostCommentCreator = testPostCommentCreator(false, "", nil)
var errorPostCommentCreator = testPostCommentCreator(false, "", fmt.Errorf("error"))

var _ = Describe("IssueCreate", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var inpReaderCallCount int

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		inpReaderCallCount = 0
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
				args := &issuecmd.IssueCreateArgs{Labels: &[]string{"*la&bel"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("label (*la&bel) is not valid"))
			})

			It("should return error when a assignee is not valid", func() {
				args := &issuecmd.IssueCreateArgs{Assignees: &[]string{"*assign&ee"}}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("assignee (*assign&ee) is not a valid push key address"))
			})

			It("should return error when reply hash is set but issue number is not set", func() {
				args := &issuecmd.IssueCreateArgs{ReplyHash: "02we"}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue number is required when adding a comment"))
			})

			When("title is not set AND reply hash is not set AND issues did not previously exist", func() {
				It("should read title and body from stdIn", func() {
					mockStdOut := mocks.NewMockFileWriter(ctrl)
					mockStdOut.EXPECT().Write(gomock.Any()).AnyTimes()

					args := &issuecmd.IssueCreateArgs{
						StdOut:             mockStdOut,
						PostCommentCreator: noopPostCommentCreator,
						InputReader: func(title string, args *io2.InputReaderArgs) string {
							return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "my body")
						},
					}
					err := issuecmd.IssueCreateCmd(mockRepo, args)
					Expect(err).To(BeNil())
					Expect(args.Title).To(Equal("my title"))
					Expect(args.Body).To(Equal("my body"))
				})
			})

			It("should return error when title is not provided from stdin", func() {
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil),
					InputReader: func(title string, args *io2.InputReaderArgs) string { return "" }}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(common.ErrTitleRequired))
			})

			It("should return error when body is not provided from stdin", func() {
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil),
					InputReader: func(title string, args *io2.InputReaderArgs) string {
						return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
					},
				}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(common.ErrBodyRequired))
			})

			It("should return error when body is not provided from stdin even when NoBody is true", func() {
				args := &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), NoBody: true,
					InputReader: func(title string, args *io2.InputReaderArgs) string {
						return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
					},
				}
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(common.ErrBodyRequired))
			})

			When("custom editor is requested", func() {
				var args *issuecmd.IssueCreateArgs
				BeforeEach(func() {
					args = &issuecmd.IssueCreateArgs{StdOut: bytes.NewBuffer(nil), UseEditor: true,
						InputReader: func(title string, args *io2.InputReaderArgs) string {
							return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
						},
					}
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
					Expect(err).To(Equal(common.ErrBodyRequired))
				})
			})

		})

		When("issue number is set (new issue creation or comment)", func() {
			It("should return error when issue does not exist and reply hash is set", func() {
				args := &issuecmd.IssueCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("", plumbing.ErrRefNotFound)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("issue (1) was not found"))
			})

			It("should return error when unable to get issue reference", func() {
				args := &issuecmd.IssueCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when unable to count number of comments in reference", func() {
				args := &issuecmd.IssueCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeIssueReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(0, fmt.Errorf("error"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to count comments in issue: error"))
			})

			It("should return error when issue has commits and title is provided", func() {
				args := &issuecmd.IssueCreateArgs{ID: 1, Title: "Some Title"}
				ref := plumbing.MakeIssueReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("title not required when adding a comment to an issue"))
			})

			It("should return error when reply hash does not exist in issue branch", func() {
				args := &issuecmd.IssueCreateArgs{ID: 1, ReplyHash: "reply_hash"}
				ref := plumbing.MakeIssueReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("reply_hash", "xyz").Return(fmt.Errorf("bad"))
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target comment hash (reply_hash) is unknown"))
			})

			It("should not return ErrBodyRequired when NoBody=true and intent is a reply", func() {
				issueNumber := 1
				ref := plumbing.MakeIssueReference(issueNumber)
				args := &issuecmd.IssueCreateArgs{ID: issueNumber, ReplyHash: "comment_hash", NoBody: true,
					StdOut:             bytes.NewBuffer(nil),
					PostCommentCreator: testPostCommentCreator(true, ref, nil)}

				mockRepo.EXPECT().RefGet(ref).Return("ref_hash", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("comment_hash", "ref_hash").Return(nil)
				err := issuecmd.IssueCreateCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should return error when unable to create issue/comment", func() {
			args := &issuecmd.IssueCreateArgs{
				StdOut:             bytes.NewBuffer(nil),
				PostCommentCreator: errorPostCommentCreator,
				InputReader: func(title string, args *io2.InputReaderArgs) string {
					return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "my body")
				},
			}
			err := issuecmd.IssueCreateCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(args.Title).To(Equal("my title"))
			Expect(args.Body).To(Equal("my body"))
			Expect(err).To(MatchError("failed to create or add new comment to issue: error"))
		})
	})
})
