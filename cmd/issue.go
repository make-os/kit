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
	Short: "Create an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		replyTo, _ := cmd.Flags().GetString("replyTo")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		editorPath, _ := cmd.Flags().GetString("editor")
		targetIssue, _ := cmd.Flags().GetString("issue")
		labels, _ := cmd.Flags().GetStringSlice("labels")
		assignees, _ := cmd.Flags().GetStringSlice("assignees")
		fixers, _ := cmd.Flags().GetStringSlice("fixers")

		if err := remotecmd.IssueCreateCmd(title, body, replyTo, labels, assignees, fixers,
			useEditor, editorPath, targetIssue, os.Stdout, cfg.Node.GitBinPath); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	issueCmd.AddCommand(issueCreateCmd)
	rootCmd.AddCommand(issueCmd)
	issueCreateCmd.Flags().StringP("title", "t", "", "Title of the issue (max. 250 B)")
	issueCreateCmd.Flags().StringP("body", "b", "", "Body of the issue (max. 8 KB)")
	issueCreateCmd.Flags().StringP("replyTo", "r", "", "Set the hash of an issue comment to respond to")
	issueCreateCmd.Flags().StringSliceP("labels", "l", nil, "Labels to associate to the issue (max. 10)")
	issueCreateCmd.Flags().StringSliceP("assignees", "a", nil, "Push key of assignees (max. 10)")
	issueCreateCmd.Flags().StringSliceP("fixers", "f", nil, "Push key of fixers (max. 10)")
	issueCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	issueCreateCmd.Flags().StringP("editor", "e", "", "GetPath an editor to use instead of the git configured editor")
	issueCreateCmd.Flags().StringP("issue", "i", "", "Add a comment commit to the specified issue")
}
