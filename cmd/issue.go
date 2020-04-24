// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	remotecmd "gitlab.com/makeos/mosdef/remote/cmd"
)

// issueCmd represents the issue command
var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Create, read, list and respond to issues",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// issueCreateCmd represents a sub-command to create an issue
var issueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an issue or add a comment to an existing issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		editorPath, _ := cmd.Flags().GetString("editor")
		labels, _ := cmd.Flags().GetStringSlice("labels")
		assignees, _ := cmd.Flags().GetStringSlice("assignees")
		fixers, _ := cmd.Flags().GetStringSlice("fixers")
		issueID, _ := cmd.Flags().GetInt("issue-id")

		if err := remotecmd.IssueCreateCmd(title, body, commentCommitID, labels, assignees, fixers,
			useEditor, editorPath, os.Stdout, cfg.Node.GitBinPath, issueID); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	issueCmd.AddCommand(issueCreateCmd)
	rootCmd.AddCommand(issueCmd)
	issueCreateCmd.Flags().StringP("title", "t", "", "The issue title (max. 250 B)")
	issueCreateCmd.Flags().StringP("body", "b", "", "The issue message (max. 8 KB)")
	issueCreateCmd.Flags().StringP("reply", "r", "", "Hash or ID of comment commit to respond to")
	issueCreateCmd.Flags().StringSliceP("labels", "l", nil, "Specify labels to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().StringSliceP("assignees", "a", nil, "Specify push key of assignees to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().StringSliceP("fixers", "f", nil, "Specify push key of fixers to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	issueCreateCmd.Flags().StringP("editor", "e", "", "GetPath an editor to use instead of the git configured editor")
	issueCreateCmd.Flags().IntP("issue-id", "i", 0, "Specify a target issue number to create or add a comment")
}
