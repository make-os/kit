package repocmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/cmd/signcmd"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	config2 "gopkg.in/src-d/go-git.v4/config"
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

		It("should return error when unable to get repo config", func() {
			in := bytes.NewBuffer([]byte("refs/notes/mynote3 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/notes/mynote3 0000000000000000000000000000000000000000\n"))
			args := &HookArgs{Stdin: in}
			mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error"))
			err := HookCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to set 'hook.curRemote' option", func() {
			in := testutil.Reader{Data: []byte{}}
			args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
			repoCfg := config2.NewConfig()
			mockRepo.EXPECT().Config().Return(repoCfg, nil)
			mockRepo.EXPECT().SetConfig(repoCfg).Return(fmt.Errorf("error"))
			err := HookCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to set `hook.curRemote` value: error"))
		})

		When("the target remote already has tokens field set", func() {
			It("should return nil", func() {
				in := testutil.Reader{Data: []byte{}}
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				repoCfg.Raw.Section("remote").Subsection("remote_name").SetOption("tokens", "abc")
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("no reference was received from stdin", func() {
			It("should return nil and set `hook.curRemote` config option", func() {
				in := testutil.Reader{Data: []byte{}}
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
				Expect(repoCfg.Raw.Section("hook").Option("curRemote")).To(Equal("remote_name"))
			})
		})

		When("branch reference was received from stdin", func() {
			It("should return error if commit signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *signcmd.SignCommitArgs) error {
					return fmt.Errorf("commit sign error")
				}
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
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
				repoCfg := config2.NewConfig()
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".HandleAskPass", func() {
		It("should return nil and output nothing if 'Password' is requested", func() {
			out := bytes.NewBuffer(nil)
			err := HandleAskPass(out, ioutil.Discard, mockRepo, []string{"Password"})
			Expect(err).To(BeNil())
			Expect(out.String()).To(Equal(""))
		})

		When("Username is requested", func() {
			var repoCfg *config2.Config

			BeforeEach(func() {
				repoCfg = config2.NewConfig()
			})

			It("should return err when unable to get config", func() {
				out := bytes.NewBuffer(nil)
				mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error"))
				err := HandleAskPass(out, ioutil.Discard, mockRepo, []string{"Username"})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to get repo config: error"))
			})

			It("should return err when token was not set in 'hook.curRemote'", func() {
				out := bytes.NewBuffer(nil)
				mockRepo.EXPECT().Config().Return(repoCfg, nil)
				err := HandleAskPass(out, ioutil.Discard, mockRepo, []string{"Username"})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("push token was not found"))
			})

			When("finished with no error", func() {
				var out *bytes.Buffer
				BeforeEach(func() {
					out = bytes.NewBuffer(nil)
					mockRepo.EXPECT().Config().Return(repoCfg, nil)
					repoCfg.Raw.Section("hook").SetOption("curRemote", "origin")
					mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
					repoCfg.Raw.Section("remote").Subsection("origin").SetOption("tokens", "abc")
				})

				It("should return no error and print token on success", func() {
					err := HandleAskPass(out, ioutil.Discard, mockRepo, []string{"Username"})
					Expect(err).To(BeNil())
					Expect(out.String()).To(Equal("abc"))
				})

				It("should remove config fields", func() {
					repoCfg.Raw.Section("sign").SetOption("mergeID", "12")
					err := HandleAskPass(out, ioutil.Discard, mockRepo, []string{"Username"})
					Expect(err).To(BeNil())
					Expect(repoCfg.Raw.Section("hook").Options).To(BeNil())
					Expect(repoCfg.Raw.Section("remote").Subsection("origin").Options).To(BeEmpty())
					Expect(repoCfg.Raw.Section("sign").Option("mergeID")).To(BeEmpty())
				})
			})
		})
	})
})
