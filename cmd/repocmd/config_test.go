package repocmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	mocks2 "github.com/make-os/kit/util/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thoas/go-funk"
	config2 "gopkg.in/src-d/go-git.v4/config"
)

var _ = Describe("ConfigCmd", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var repoCfg *config2.Config
	var hooksDir, gitDir string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		repoCfg = config2.NewConfig()
		mockRepo.EXPECT().Config().Return(repoCfg, nil).AnyTimes()
		mockRepo.EXPECT().GetPath().Return(cfg.DataDir()).AnyTimes()
		hooksDir = filepath.Join(cfg.DataDir(), ".git", "hooks")
		gitDir = filepath.Join(cfg.DataDir(), ".git")
		Expect(os.MkdirAll(hooksDir, 0700)).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConfigCmd", func() {
		BeforeEach(func() {
			mockRepo.EXPECT().GetName().Return("repo1").AnyTimes()
		})

		It("should return error when unable to get repo config", func() {
			mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error"))
			args := &ConfigArgs{}
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(MatchError("error"))
		})

		It("should set args.SigningKeyPass to <APPNAME>_PASS if args.SigningKeyPass is nil", func() {
			var actual string
			var expected = "pass"
			_ = os.Setenv(common.MakePassEnvVar(config.AppName), expected)
			defer os.Unsetenv(common.MakePassEnvVar(config.AppName))
			args := &ConfigArgs{
				PassAgentPort: funk.PtrOf("9004").(*string),
				PassAgentUp: func(port string) bool {
					return true
				},
				PassAgentSet: func(port, key, pass string, ttl int) error {
					actual = pass
					return fmt.Errorf("")
				}}
			_ = ConfigCmd(nil, mockRepo, args)
			Expect(actual).To(Equal(expected))
		})

		It("should set user.fee if args.Fee is set", func() {
			fee := 12.3
			args := &ConfigArgs{Fee: &fee}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("fee")).To(Equal("12.3"))
		})

		It("should set user.fee = 0 if args.Fee is unset", func() {
			args := &ConfigArgs{Fee: nil}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("fee")).To(Equal("0"))
		})

		It("should set user.value if args.Value is set", func() {
			value := 12.3
			args := &ConfigArgs{Value: &value}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("value")).To(Equal("12.3"))
		})

		It("should set user.nonce if args.Nonce is set", func() {
			nonce := uint64(23)
			args := &ConfigArgs{Nonce: &nonce}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("nonce")).To(Equal("23"))
		})

		It("should set user.nonce=0 if args.Nonce is unset", func() {
			args := &ConfigArgs{Nonce: nil}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("nonce")).To(Equal("0"))
		})

		It("should set user.signingKey if args.SigningKey is set", func() {
			signingKey := "key"
			args := &ConfigArgs{SigningKey: &signingKey}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("signingKey")).To(Equal("key"))
		})

		It("should set user.signingKey to args.PushKey if args.PushKey is set", func() {
			signingKey := "key"
			pushKey := "push_key"
			args := &ConfigArgs{SigningKey: &signingKey, PushKey: &pushKey}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("signingKey")).To(Equal(pushKey))
		})

		It("should set user.signingKey if args.SigningKey is set and args.PushKey is empty", func() {
			signingKey := "key"
			pushKey := ""
			args := &ConfigArgs{SigningKey: &signingKey, PushKey: &pushKey}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Raw.Section("user").Option("signingKey")).To(Equal("key"))
		})

		When("starting the passphrase agent (when signing key passphrase is set)", func() {
			It("should attempt to start the passphrase agent if not already started and return error on failure to start", func() {
				passphrase := "pass"
				args := &ConfigArgs{
					SigningKeyPass: &passphrase,
					PassAgentPort:  funk.PtrOf("9004").(*string),
					PassAgentUp: func(port string) bool {
						Expect(port).To(Equal("9004"))
						return false
					},
					CommandCreator: func(name string, args ...string) util.Cmd {
						mockCmd := mocks2.NewMockCmd(ctrl)
						mockCmd.EXPECT().SetStdout(gomock.Any())
						mockCmd.EXPECT().SetStderr(gomock.Any())
						mockCmd.EXPECT().Start().Return(fmt.Errorf("error"))
						return mockCmd
					},
				}
				err = ConfigCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to start passphrase agent: error"))
			})

			It("should return error when pass cache TTL duration is invalid", func() {
				passphrase := "pass"
				args := &ConfigArgs{
					SigningKeyPass: &passphrase,
					PassAgentPort:  funk.PtrOf("9004").(*string),
					PassAgentUp: func(port string) bool {
						return true
					},
					PassCacheTTL: "invalid",
				}
				err = ConfigCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("passphrase cache duration is not valid"))
			})

			It("should return error when unable to send set request to passphrase agent", func() {
				passphrase := "pass"
				args := &ConfigArgs{
					SigningKeyPass: &passphrase,
					PassAgentPort:  funk.PtrOf("9004").(*string),
					PassAgentUp: func(port string) bool {
						return true
					},
					PassCacheTTL: "1h",
					PassAgentSet: func(port, key, pass string, ttl int) error {
						return fmt.Errorf("error")
					},
				}
				err = ConfigCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to send set request to passphrase agent: error"))
			})
		})

		It("should set remotes", func() {
			args := &ConfigArgs{Remotes: []Remote{{Name: "r1", URL: "remote.com,remote2.com"}, {Name: "r2", URL: "remote3.com"}}}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(repoCfg.Remotes).To(HaveLen(2))
			Expect(repoCfg.Remotes["r1"].Name).To(Equal("r1"))
			Expect(repoCfg.Remotes["r1"].URLs).To(And(ContainElement("remote.com"), ContainElement("remote2.com")))
			Expect(repoCfg.Remotes["r2"].Name).To(Equal("r2"))
			Expect(repoCfg.Remotes["r2"].URLs).To(ContainElement("remote3.com"))
		})

		When("no user defined remote", func() {
			It("should set default remote", func() {
				config.DefaultRemoteServerAddress = "127.0.0.1:9000"
				args := &ConfigArgs{Remotes: []Remote{}}
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				err = ConfigCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
				Expect(repoCfg.Remotes).To(HaveLen(1))
				Expect(repoCfg.Remotes["origin"].Name).To(Equal("origin"))
				Expect(repoCfg.Remotes["origin"].URLs).To(ContainElement(fmt.Sprintf("http://%s/r/%s", config.DefaultRemoteServerAddress, "repo1")))
			})

			It("should set default remote host to localhost if it is missing", func() {
				config.DefaultRemoteServerAddress = ":9000"
				args := &ConfigArgs{Remotes: []Remote{}}
				mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
				err = ConfigCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
				Expect(repoCfg.Remotes).To(HaveLen(1))
				Expect(repoCfg.Remotes["origin"].Name).To(Equal("origin"))
				Expect(repoCfg.Remotes["origin"].URLs).To(ContainElement(fmt.Sprintf("http://127.0.0.1:9000/r/%s", "repo1")))
			})
		})

		It("should add pre-push hook file", func() {
			args := &ConfigArgs{}
			mockRepo.EXPECT().SetConfig(repoCfg).Return(nil)
			err = ConfigCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
			prePush, err := ioutil.ReadFile(filepath.Join(hooksDir, "pre-push"))
			Expect(err).To(BeNil())
			Expect(prePush).ToNot(BeEmpty())
		})
	})

	Describe(".addHooks", func() {
		It("should add hook files if they don't already exist", func() {
			err := addHooks(config.AppName, gitDir)
			Expect(err).To(BeNil())
			prePush, err := ioutil.ReadFile(filepath.Join(hooksDir, "pre-push"))
			Expect(err).To(BeNil())
			Expect(string(prePush)).To(Equal(fmt.Sprintf("#!/bin/sh\n%s repo hook $1", config.AppName)))
			prePush, err = ioutil.ReadFile(filepath.Join(hooksDir, "post-commit"))
			Expect(err).To(BeNil())
			Expect(prePush).ToNot(BeEmpty())
			Expect(string(prePush)).To(Equal(fmt.Sprintf("#!/bin/sh\n%s repo hook -c", config.AppName)))
		})

		When("hook files already exist but the have no <AppName> command", func() {
			It("should add <AppName> command to the files", func() {
				err := ioutil.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte("line 1"), 0700)
				Expect(err).To(BeNil())
				err = addHooks(config.AppName, gitDir)
				Expect(err).To(BeNil())
				prePush, err := ioutil.ReadFile(filepath.Join(hooksDir, "pre-push"))
				Expect(err).To(BeNil())
				Expect(string(prePush)).To(ContainSubstring(config.AppName))
			})
		})

		When("hook files already exist and have a CLIName command", func() {
			It("should not re-add CLIName command to the files", func() {
				err = addHooks(config.AppName, gitDir)
				Expect(err).To(BeNil())
				prePush, err := ioutil.ReadFile(filepath.Join(hooksDir, "pre-push"))
				Expect(err).To(BeNil())
				Expect(string(prePush)).To(ContainSubstring(config.AppName))

				err = addHooks("", gitDir)
				Expect(err).To(BeNil())
				prePush2, err := ioutil.ReadFile(filepath.Join(hooksDir, "pre-push"))
				Expect(err).To(BeNil())
				Expect(string(prePush)).To(ContainSubstring(config.AppName))
				Expect(prePush).To(Equal(prePush2))
			})
		})
	})
})
