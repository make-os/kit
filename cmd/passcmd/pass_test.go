package passcmd_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/passcmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPassCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PassCmd Suite")
}

var _ = Describe("Pass", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".PassCmd", func() {
		It("should return error when unable to start agent when args.StartAgent is true", func() {
			args := &passcmd.PassArgs{StartAgent: true}
			args.AgentStarter = func(string) error {
				return fmt.Errorf("error")
			}
			err := passcmd.PassCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to stop agent when args.StopAgent is true", func() {
			args := &passcmd.PassArgs{StopAgent: true}
			args.AgentStopper = func(string) error {
				return fmt.Errorf("error")
			}
			err := passcmd.PassCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("cache duration is set", func() {
			It("should fail when cache duration is bad", func() {
				args := &passcmd.PassArgs{CacheDuration: "1z"}
				err := passcmd.PassCmd(args)
				Expect(err).To(MatchError("bad duration: time: unknown unit z in duration 1z"))
			})

			It("should attempt to start agent process if not already started but return error when unable to start the agent", func() {
				args := &passcmd.PassArgs{CacheDuration: "1s"}
				args.AgentStatusChecker = func(port string) bool { return false }
				args.CommandCreator = func(name string, args ...string) util.Cmd {
					mockCmd := mocks.NewMockCmd(ctrl)
					mockCmd.EXPECT().Start().Return(fmt.Errorf("error"))
					mockCmd.EXPECT().SetStderr(gomock.Any())
					mockCmd.EXPECT().SetStdout(gomock.Any())
					return mockCmd
				}
				err := passcmd.PassCmd(args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to start agent: error"))
			})
		})

		It("should return error if passphrase prompt failed", func() {
			args := &passcmd.PassArgs{Key: "mykey"}
			args.AskPass = func(_ ...string) (string, error) {
				return "", fmt.Errorf("error")
			}
			err := passcmd.PassCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to ask for passphrase: error"))
		})

		It("should store passphrase to <APPNAME>_PASS env var", func() {
			args := &passcmd.PassArgs{Key: "mykey"}
			args.AskPass = func(_ ...string) (string, error) {
				return "mypass", nil
			}
			err := passcmd.PassCmd(args)
			Expect(err).To(BeNil())
			Expect(os.Getenv(common.MakePassEnvVar(config.AppName))).To(Equal("mypass"))
		})

		It("should send set request if cache is set and agent service is running", func() {
			sent := false
			args := &passcmd.PassArgs{Key: "mykey", CacheDuration: "10s"}
			args.AgentStatusChecker = func(port string) bool { return true }
			args.SetRequestSender = func(port, key, pass string, ttl int) error {
				sent = true
				return nil
			}
			args.AskPass = func(_ ...string) (string, error) {
				return "mypass", nil
			}
			err := passcmd.PassCmd(args)
			Expect(err).To(BeNil())
			Expect(sent).To(BeTrue())
		})

		It("should return error when unable to send set request", func() {
			args := &passcmd.PassArgs{Key: "mykey", CacheDuration: "10s"}
			args.AgentStatusChecker = func(port string) bool { return true }
			args.SetRequestSender = func(port, key, pass string, ttl int) error {
				return fmt.Errorf("error")
			}
			args.AskPass = func(_ ...string) (string, error) {
				return "mypass", nil
			}
			err := passcmd.PassCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to send set request: error"))
		})

		It("should return error when unable to call sub-command", func() {
			args := &passcmd.PassArgs{Key: "mykey", Args: []string{"some", "command"}}
			args.AskPass = func(_ ...string) (string, error) {
				return "mypass", nil
			}
			args.CommandCreator = func(name string, args ...string) util.Cmd {
				Expect(name).To(Equal("some"))
				Expect(args).To(ContainElement("command"))
				mockCmd := mocks.NewMockCmd(ctrl)
				mockCmd.EXPECT().Start().Return(fmt.Errorf("error"))
				mockCmd.EXPECT().SetStderr(gomock.Any())
				mockCmd.EXPECT().SetStdout(gomock.Any())
				mockCmd.EXPECT().SetStdin(gomock.Any())
				return mockCmd
			}
			err := passcmd.PassCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to run command [some command]: error"))
		})

		It("should return nil on successful call of sub-command", func() {
			args := &passcmd.PassArgs{Key: "mykey", Args: []string{"some", "command"}}
			args.AskPass = func(_ ...string) (string, error) {
				return "mypass", nil
			}
			args.CommandCreator = func(name string, args ...string) util.Cmd {
				Expect(name).To(Equal("some"))
				Expect(args).To(ContainElement("command"))
				mockCmd := mocks.NewMockCmd(ctrl)
				mockCmd.EXPECT().Start().Return(nil)
				mockCmd.EXPECT().SetStderr(gomock.Any())
				mockCmd.EXPECT().SetStdout(gomock.Any())
				mockCmd.EXPECT().SetStdin(gomock.Any())
				mockCmd.EXPECT().Wait().Return(nil)
				return mockCmd
			}
			err := passcmd.PassCmd(args)
			Expect(err).To(BeNil())
		})
	})
})
