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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/remote/cmd/issuecmd"
	"gitlab.com/makeos/mosdef/remote/issues"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/util"
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
		rejectFlagCombo(cmd, "close", "open")

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		noBody, _ := cmd.Flags().GetBool("no-body")
		cls, _ := cmd.Flags().GetBool("close")
		open, _ := cmd.Flags().GetBool("open")
		editorPath, _ := cmd.Flags().GetString("editor")
		labels, _ := cmd.Flags().GetString("labels")
		reactions, _ := cmd.Flags().GetStringSlice("reactions")
		assignees, _ := cmd.Flags().GetString("assignees")
		issueID, _ := cmd.Flags().GetInt("issue-id")

		targetRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		issueCreateArgs := &issuecmd.IssueCreateArgs{
			IssueNumber:         issueID,
			Title:               title,
			Body:                body,
			NoBody:              noBody,
			ReplyHash:           commentCommitID,
			Reactions:           funk.UniqString(reactions),
			UseEditor:           useEditor,
			EditorPath:          editorPath,
			Open:                open,
			StdOut:              os.Stdout,
			StdIn:               os.Stdin,
			IssueCommentCreator: issues.CreateIssueComment,
			EditorReader:        util.ReadFromEditor,
		}

		if cmd.Flags().Changed("close") {
			issueCreateArgs.Close = &cls
		}

		if cmd.Flags().Changed("labels") {
			if labels == "" {
				issueCreateArgs.Labels = []string{}
			} else {
				issueCreateArgs.Labels = funk.UniqString(strings.Split(labels, ","))
			}
		}

		if cmd.Flags().Changed("assignees") {
			if assignees == "" {
				issueCreateArgs.Assignees = []string{}
			} else {
				issueCreateArgs.Assignees = funk.UniqString(strings.Split(assignees, ","))
			}
		}

		if err := issuecmd.IssueCreateCmd(targetRepo, issueCreateArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueListCmd represents a sub-command to list all issues
var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all issues",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		reverse, _ := cmd.Flags().GetBool("reverse")
		dateFmt, _ := cmd.Flags().GetString("date")
		format, _ := cmd.Flags().GetString("format")

		targetRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueListCmd(targetRepo, &issuecmd.IssueListArgs{
			Limit:      limit,
			Reverse:    reverse,
			DateFmt:    dateFmt,
			PostGetter: plumbing.GetPosts,
			PagerWrite: issuecmd.WriteToPager,
			Format:     format,
			StdOut:     os.Stdout,
			StdErr:     os.Stderr,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	issueCmd.AddCommand(issueCreateCmd)
	issueCmd.AddCommand(issueListCmd)
	rootCmd.AddCommand(issueCmd)

	issueCreateCmd.Flags().StringP("title", "t", "", "The issue title (max. 250 B)")
	issueCreateCmd.Flags().StringP("body", "b", "", "The issue message (max. 8 KB)")
	issueCreateCmd.Flags().StringP("reply", "r", "", "Specify the hash of a comment to respond to")
	issueCreateCmd.Flags().StringSliceP("reactions", "e", nil, "Add reactions to a reply (max. 10)")
	issueCreateCmd.Flags().StringP("labels", "l", "", "Specify labels to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().StringP("assignees", "a", "", "Specify push key of assignees to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	issueCreateCmd.Flags().Bool("no-body", false, "Skip prompt for issue body")
	issueCreateCmd.Flags().String("editor", "", "GetPath an editor to use instead of the git configured editor")
	issueCreateCmd.Flags().IntP("issue-id", "i", 0, "Specify a target issue number to create or add a comment")
	issueCreateCmd.Flags().BoolP("close", "c", false, "Close the issue")
	issueCreateCmd.Flags().BoolP("open", "o", false, "Open a closed issue")

	issueListCmd.Flags().IntP("limit", "n", 0, "Limit the number of issues returned")
	issueListCmd.Flags().Bool("reverse", false, "Return the result in reversed order")
	issueListCmd.Flags().StringP("date", "d", "Mon Jan _2 15:04:05 2006 -0700", "Set date format")
	issueListCmd.Flags().StringP("format", "f", "", "Set output format")
}
