package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/repo"
)

// IsGitSignRequest checks whether the program arguments
// indicate a request from git to sign a message
func IsGitSignRequest(args []string) bool {
	return len(args) == 4 && args[1] == "--status-fd=2" && args[2] == "-bsau"
}

// IsGitVerifyRequest checks whether the program arguments
// indicate a request from git to verify a signature
func IsGitVerifyRequest(args []string) bool {
	return len(args) == 6 && funk.ContainsString(args, "--verify")
}

// fallbackCmd is called any time an unknown command is executed
var fallbackCmd = &cobra.Command{
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {

		if IsGitSignRequest(args) {
			repo.GitSignCmd(args, os.Stdin)
		}

		if IsGitVerifyRequest(args) {
			repo.GitVerifyCmd(args)
		}

		fmt.Print("Unknown command. Use --help to see commands.\n")
		os.Exit(1)
	},
}
