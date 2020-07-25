package mergecmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/commands/mergecmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	io2 "github.com/themakeos/lobe/util/io"
)

func testPostCommentCreator(isNewPost bool, reference string, err error) func(targetRepo types.LocalRepo,
	args *plumbing.CreatePostCommitArgs) (bool, string, error) {
	return func(targetRepo types.LocalRepo, args *plumbing.CreatePostCommitArgs) (bool, string, error) {
		return isNewPost, reference, err
	}
}

var noopPostCommentCreator = testPostCommentCreator(false, "", nil)
var errorPostCommentCreator = testPostCommentCreator(false, "", fmt.Errorf("error"))

var _ = Describe("MergeRequestCreate", func() {
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

	Describe(".MergeRequestCreateCmd", func() {
		When("merge request number is unset (new merge request creation)", func() {
			It("should return error when a reaction is unknown", func() {
				args := &mergecmd.MergeRequestCreateArgs{Reactions: []string{":unknown:"}}
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("reaction (:unknown:) is not supported"))
			})

			It("should return error when reply hash is set but merge request number is not set", func() {
				args := &mergecmd.MergeRequestCreateArgs{ReplyHash: "02we"}
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge request number is required when adding a comment"))
			})

			When("title is not set AND reply hash is not set AND merge request did not previously exist", func() {
				It("should read title and body from stdIn", func() {
					mockStdOut := mocks.NewMockFileWriter(ctrl)
					mockStdOut.EXPECT().Write(gomock.Any()).AnyTimes()
					args := &mergecmd.MergeRequestCreateArgs{
						StdOut:             mockStdOut,
						PostCommentCreator: noopPostCommentCreator,
						InputReader: func(title string, args *io2.InputReaderArgs) string {
							return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "my body")
						},
					}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).To(BeNil())
					Expect(args.Title).To(Equal("my title"))
					Expect(args.Body).To(Equal("my body"))
				})
			})

			It("should return error when title is not provided from stdin", func() {
				args := &mergecmd.MergeRequestCreateArgs{StdOut: bytes.NewBuffer(nil),
					InputReader: func(title string, args *io2.InputReaderArgs) string { return "" }}
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(common.ErrTitleRequired))
			})

			It("should return error when body is not provided from stdin", func() {
				args := &mergecmd.MergeRequestCreateArgs{StdOut: bytes.NewBuffer(nil),
					InputReader: func(title string, args *io2.InputReaderArgs) string {
						return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
					},
				}
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(common.ErrBodyRequired))
			})

			It("should return error when body is not provided from stdin even when NoBody is true", func() {
				args := &mergecmd.MergeRequestCreateArgs{StdOut: bytes.NewBuffer(nil), NoBody: true,
					InputReader: func(title string, args *io2.InputReaderArgs) string {
						return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
					},
				}
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(args.Title).To(Equal("my title"))
				Expect(err).To(Equal(common.ErrBodyRequired))
			})

			When("custom editor is requested", func() {
				var args *mergecmd.MergeRequestCreateArgs
				BeforeEach(func() {
					args = &mergecmd.MergeRequestCreateArgs{StdOut: bytes.NewBuffer(nil), UseEditor: true,
						InputReader: func(title string, args *io2.InputReaderArgs) string {
							return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "")
						},
					}
				})

				It("should request fetch core.editor from git config", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					mergecmd.MergeRequestCreateCmd(mockRepo, args)
				})

				It("should use custom editor program is EditorPath is set", func() {
					args.EditorPath = "myeditor"
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						Expect(editor).To(Equal(args.EditorPath))
						return "", nil
					}
					mergecmd.MergeRequestCreateCmd(mockRepo, args)
				})

				It("should return error if reading from editor failed", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
						return "", fmt.Errorf("error")
					}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("failed read body from editor: error"))
				})

				It("should return error when body is unset through editor", func() {
					mockRepo.EXPECT().GetConfig("core.editor")
					args.EditorReader = func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) { return "", nil }
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(Equal(common.ErrBodyRequired))
				})
			})
		})

		When("merge request number is set (new merge request creation or comment)", func() {
			It("should return error when merge request does not exist and reply hash is set", func() {
				args := &mergecmd.MergeRequestCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeMergeRequestReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("", plumbing.ErrRefNotFound)
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge request (1) was not found"))
			})

			It("should return error when unable to get merge request reference", func() {
				args := &mergecmd.MergeRequestCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeMergeRequestReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error"))
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when unable to count number of comments in reference", func() {
				args := &mergecmd.MergeRequestCreateArgs{ID: 1, ReplyHash: "xyz"}
				ref := plumbing.MakeMergeRequestReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(0, fmt.Errorf("error"))
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to count comments in merge request: error"))
			})

			It("should return error when merge request has comments and title is provided", func() {
				args := &mergecmd.MergeRequestCreateArgs{ID: 1, Title: "Some Title"}
				ref := plumbing.MakeMergeRequestReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("title not required when adding a comment to a merge request"))
			})

			It("should return error when reply hash does not exist in merge request reference", func() {
				args := &mergecmd.MergeRequestCreateArgs{ID: 1, ReplyHash: "reply_hash"}
				ref := plumbing.MakeMergeRequestReference(args.ID)
				mockRepo.EXPECT().RefGet(ref).Return("xyz", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("reply_hash", "xyz").Return(fmt.Errorf("bad"))
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target comment hash (reply_hash) is unknown"))
			})

			When("base, baseHash, target, targetHash are unset and merge request reference is new", func() {
				var args *mergecmd.MergeRequestCreateArgs
				BeforeEach(func() {
					mockRepo.EXPECT().RefGet(gomock.Any()).Return("xyz", plumbing.ErrRefNotFound)
					mockRepo.EXPECT().NumCommits(gomock.Any(), false).Return(0, nil)
				})

				It("should return error when base branch is unset in a new merge request post", func() {
					args = &mergecmd.MergeRequestCreateArgs{ID: 1}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("base branch name is required"))
				})

				It("should return error when base branch hash is unset in a new merge request post", func() {
					args = &mergecmd.MergeRequestCreateArgs{ID: 1, Base: "master"}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("base branch hash is required"))
				})

				It("should return error when target branch is unset in a new merge request post", func() {
					args = &mergecmd.MergeRequestCreateArgs{ID: 1, Base: "master", BaseHash: "hash1"}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("target branch name is required"))
				})

				It("should return error when target branch hash is unset in a new merge request post", func() {
					args = &mergecmd.MergeRequestCreateArgs{ID: 1, Base: "master", BaseHash: "hash1", Target: "dev"}
					err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("target branch hash is required"))
				})
			})

			It("should not return ErrBodyRequired when NoBody=true and intent is a reply", func() {
				mergeReqNumber := 1
				ref := plumbing.MakeMergeRequestReference(mergeReqNumber)
				args := &mergecmd.MergeRequestCreateArgs{ID: mergeReqNumber, ReplyHash: "comment_hash", NoBody: true,
					StdOut:             bytes.NewBuffer(nil),
					PostCommentCreator: testPostCommentCreator(true, ref, nil)}

				mockRepo.EXPECT().RefGet(ref).Return("ref_hash", nil)
				mockRepo.EXPECT().NumCommits(ref, false).Return(1, nil)
				mockRepo.EXPECT().IsAncestor("comment_hash", "ref_hash").Return(nil)
				err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should return error when unable to create comment", func() {
			args := &mergecmd.MergeRequestCreateArgs{
				StdOut:             bytes.NewBuffer(nil),
				PostCommentCreator: errorPostCommentCreator,
				InputReader: func(title string, args *io2.InputReaderArgs) string {
					return testutil.ReturnStringOnCallCount(&inpReaderCallCount, "my title", "my body")
				},
			}
			err := mergecmd.MergeRequestCreateCmd(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(args.Title).To(Equal("my title"))
			Expect(args.Body).To(Equal("my body"))
			Expect(err).To(MatchError("failed to create or add new comment to merge request request: error"))
		})
	})
})
