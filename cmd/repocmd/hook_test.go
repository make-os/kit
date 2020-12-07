package repocmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/signcmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe(".HookCmd", func() {
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

	Describe(".HookCmd", func() {
		It("should return error when unable to read from stdin", func() {
			in := testutil.Reader{Err: fmt.Errorf("error")}
			args := &HookArgs{Stdin: in}
			err := HookCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("branch reference was received from stdin", func() {
			It("should return error if commit signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
					return fmt.Errorf("commit sign error")
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("commit sign error"))
			})

			It("should return no error if commit signer succeeded", func() {
				in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})

			Context("with two references", func() {
				It("should return no error if commit signer succeeded", func() {
					timesCalled := 0
					in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
					in.Write([]byte("refs/heads/dev fbbefce3f78361968fcce78cc44b5a6dbebe4952 refs/heads/dev 0000000000000000000000000000000000000000\n"))
					args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
					args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
						timesCalled++
						return nil
					}
					err := HookCmd(cfg, mockRepo, args)
					Expect(err).To(BeNil())
					Expect(timesCalled).To(Equal(2))
				})
			})
		})

		When("reference is a merge or issue branch", func() {
			It("should set AmendCommit argument to true (merge request branch)", func() {
				refName := plumbing.MakeMergeRequestReference("1")
				in := bytes.NewBuffer([]byte(refName + " 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 " + refName + " 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
					Expect(args.AmendCommit).To(BeTrue())
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})

			It("should set AmendCommit argument to true (issue branch)", func() {
				refName := plumbing.MakeIssueReference("1")
				in := bytes.NewBuffer([]byte(refName + " 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 " + refName + " 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
					Expect(args.AmendCommit).To(BeTrue())
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("tag reference was received from stdin", func() {
			It("should return error if tag signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/tags/v1.2 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/tag/v1.2 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.TagSigner = func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *signcmd.SignTagArgs) error {
					Expect(gitArgs[0]).To(Equal("v1.2"))
					return fmt.Errorf("tag sign error")
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("tag sign error"))
			})

			It("should return no error if commit signer succeeded", func() {
				in := bytes.NewBuffer([]byte("refs/tags/v1.2 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/tag/v1.2 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.TagSigner = func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *signcmd.SignTagArgs) error {
					Expect(gitArgs[0]).To(Equal("v1.2"))
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("notes reference was received from stdin", func() {
			It("should return error if notes signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/notes/note1 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/notes/note1 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.NoteSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignNoteArgs) error {
					Expect(args.Name).To(Equal("note1"))
					return fmt.Errorf("notes sign error")
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("notes sign error"))
			})

			It("should return no error if commit signer succeeded", func() {
				in := bytes.NewBuffer([]byte("refs/notes/note1 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/notes/note1 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.NoteSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignNoteArgs) error {
					Expect(args.Name).To(Equal("note1"))
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})
})
