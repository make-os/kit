package repocmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	gogitcfg "github.com/go-git/go-git/v5/config"
	fmtcfg "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/golang/mock/gomock"
	types2 "github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
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

		When("args.PostCommit is true", func() {
			It("should attempt to get HEAD reference and return error on failure", func() {
				in := bytes.NewBuffer(nil)
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}, PostCommit: true}
				mockRepo.EXPECT().Head().Return("", fmt.Errorf("error"))
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to get HEAD: error"))
			})
		})

		When("args.PostCommit is true and current HEAD is a branch reference", func() {
			It("should attempt to sign the HEAD reference", func() {
				in := bytes.NewBuffer(nil)
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}, PostCommit: true}
				mockRepo.EXPECT().Head().Return("refs/heads/branch", nil)
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignCommitArgs) error {
					Expect(args.Head).To(Equal("refs/heads/branch"))
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("branch reference was received from stdin", func() {
			It("should return error if commit signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignCommitArgs) error {
					return fmt.Errorf("commit sign error")
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("commit sign error"))
			})

			It("should return no error if commit signer succeeded", func() {
				in := bytes.NewBuffer([]byte("refs/heads/master 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/heads/master 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignCommitArgs) error {
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
					args.CommitSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignCommitArgs) error {
						timesCalled++
						return nil
					}
					err := HookCmd(cfg, mockRepo, args)
					Expect(err).To(BeNil())
					Expect(timesCalled).To(Equal(2))
				})
			})
		})

		When("tag reference was received from stdin", func() {
			It("should return error if tag signer failed", func() {
				in := bytes.NewBuffer([]byte("refs/tags/v1.2 03f6ce13b4c2b8ff230d474dc058af1edff0deb9 refs/tag/v1.2 0000000000000000000000000000000000000000\n"))
				args := &HookArgs{Stdin: in, Args: []string{"remote_name"}}
				args.TagSigner = func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *types2.SignTagArgs) error {
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
				args.TagSigner = func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *types2.SignTagArgs) error {
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
				args.NoteSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignNoteArgs) error {
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
				args.NoteSigner = func(cfg *config.AppConfig, repo types.LocalRepo, args *types2.SignNoteArgs) error {
					Expect(args.Name).To(Equal("note1"))
					return nil
				}
				err := HookCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".AskPassCmd", func() {
		It("should return - for Password request", func() {
			buf := bytes.NewBuffer(nil)
			err := AskPassCmd(mockRepo, []string{"", "Password for 'http://127.0.0.1:8002':"}, buf)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(Equal("-"))
		})

		When("git is requesting for Username", func() {
			It("should return error if request url is invalid", func() {
				buf := bytes.NewBuffer(nil)
				err := AskPassCmd(mockRepo, []string{"", "Username for 'htt*:://127**.0/s1:8002':"}, buf)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad remote url"))
			})

			It("should return error when unable to get repo config", func() {
				mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error"))
				buf := bytes.NewBuffer(nil)
				err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unable to read git config: error"))
			})

			It("should return error when no remotes with matching hostname url(s) exist", func() {
				gitCfg := &gogitcfg.Config{}
				mockRepo.EXPECT().Config().Return(gitCfg, nil)
				buf := bytes.NewBuffer(nil)
				err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("no push token(s) found"))
			})

			When("a remote has a url that matches the requested url's hostname", func() {
				It("should return error when unable to get repocfg", func() {
					gitCfg := &gogitcfg.Config{Raw: &fmtcfg.Config{
						Sections: fmtcfg.Sections{
							{Name: "remote", Subsections: []*fmtcfg.Subsection{
								{Name: "origin", Options: []*fmtcfg.Option{
									{Key: "url", Value: "http://127.0.0.1:8002/r/repo"},
								}},
							}},
						},
					}}
					mockRepo.EXPECT().Config().Return(gitCfg, nil)
					mockRepo.EXPECT().GetRepoConfig().Return(nil, fmt.Errorf("error"))
					buf := bytes.NewBuffer(nil)
					err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("unable to read repocfg file: error"))
				})

				It("should return error when no push tokens where found for the selected origin", func() {
					gitCfg := &gogitcfg.Config{Raw: &fmtcfg.Config{
						Sections: fmtcfg.Sections{
							{Name: "remote", Subsections: []*fmtcfg.Subsection{
								{Name: "origin", Options: []*fmtcfg.Option{
									{Key: "url", Value: "http://127.0.0.1:8002/r/repo"},
								}},
							}},
						},
					}}
					mockRepo.EXPECT().Config().Return(gitCfg, nil)
					mockRepo.EXPECT().GetRepoConfig().Return(&types.LocalConfig{Tokens: map[string][]string{}}, nil)
					buf := bytes.NewBuffer(nil)
					err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("no push token(s) found"))
					Expect(buf.String()).To(Equal(""))
				})

				It("should return nil error and non-empty output and when push tokens where found for the selected origin", func() {
					gitCfg := &gogitcfg.Config{Raw: &fmtcfg.Config{
						Sections: fmtcfg.Sections{
							{Name: "remote", Subsections: []*fmtcfg.Subsection{
								{Name: "origin", Options: []*fmtcfg.Option{
									{Key: "url", Value: "http://127.0.0.1:8002/r/repo"},
								}},
								{Name: "origin2", Options: []*fmtcfg.Option{
									{Key: "push", Value: "http://127.0.0.1:8002/r/repo2"},
								}},
							}},
						},
					}}
					mockRepo.EXPECT().Config().Return(gitCfg, nil)
					mockRepo.EXPECT().GetRepoConfig().Return(&types.LocalConfig{Tokens: map[string][]string{
						"origin":  {"token1", "token2"},
						"origin2": {"token3"},
					}}, nil)
					buf := bytes.NewBuffer(nil)
					err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
					Expect(err).To(BeNil())
					Expect(strings.Split(buf.String(), ",")).To(ContainElements("token1", "token2", "token3"))
				})

				It("should skip remote with kitignore option", func() {
					gitCfg := &gogitcfg.Config{Raw: &fmtcfg.Config{
						Sections: fmtcfg.Sections{
							{Name: "remote", Subsections: []*fmtcfg.Subsection{
								{Name: "origin", Options: []*fmtcfg.Option{
									{Key: "url", Value: "http://127.0.0.1:8002/r/repo"},
									{Key: "kitignore", Value: "true"},
								}},
							}},
						},
					}}
					mockRepo.EXPECT().Config().Return(gitCfg, nil)
					buf := bytes.NewBuffer(nil)
					err := AskPassCmd(mockRepo, []string{"", "Username for 'http://127.0.0.1:8002':"}, buf)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("no push token(s) found"))
					Expect(buf.String()).To(Equal(""))
				})
			})
		})
	})
})
