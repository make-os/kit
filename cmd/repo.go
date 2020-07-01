package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/remote/cmd/repocmd"
)

// repoCmd represents the repo command
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Create, find and manage repositories",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// repoCreateCmd represents a sub-command to create a repository
var repoCreateCmd = &cobra.Command{
	Use:   "create [flags] <name>",
	Short: "Create a repository",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("name is required")
		}
		return nil
	},
	Long: ``,
	Run: func(cmd *cobra.Command, args []string) {
		repocmd.CreateCmd(&repocmd.CreateArgs{
			Name: args[0],
		})
	},
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCreateCmd)
	// repoCreateCmd.Flags().StringP("title", "t", "", "The issue title (max. 250 B)")
}
