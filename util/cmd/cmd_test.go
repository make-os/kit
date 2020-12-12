package cmd_test

import (
	"testing"

	cmd2 "github.com/make-os/kit/util/cmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}

var _ = Describe("Cmd", func() {
	Describe(".RejectFlagCombo", func() {
		It("should return error when f and x flags were set together", func() {
			cmd := &cobra.Command{}
			cmd.Flags().StringP("f", "f", "val", "")
			cmd.Flags().StringP("x", "x", "val", "")
			err := cmd.ParseFlags([]string{"-f", "abc", "-x", "xyz"})
			Expect(err).To(BeNil())
			err = cmd2.RejectFlagCombo(cmd, "f", "x")
			Expect(err).ToNot(BeNil())
		})

		It("should return no error when f and x flags were not set together", func() {
			cmd := &cobra.Command{}
			cmd.Flags().StringP("f", "f", "val", "")
			cmd.Flags().StringP("x", "x", "val", "")
			cmd.Flags().StringP("z", "z", "val", "")
			err := cmd.ParseFlags([]string{"-f", "abc", "-x", "xyz"})
			Expect(err).To(BeNil())
			err = cmd2.RejectFlagCombo(cmd, "f", "z")
			Expect(err).To(BeNil())
		})
	})
})
