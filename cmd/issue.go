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
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/remote/cmd/common"
	"gitlab.com/makeos/mosdef/remote/cmd/issuecmd"
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
		rejectFlagCombo(cmd, "close", "reopen")

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		noBody, _ := cmd.Flags().GetBool("no-body")
		cls, _ := cmd.Flags().GetBool("close")
		forceNew, _ := cmd.Flags().GetBool("new")
		open, _ := cmd.Flags().GetBool("reopen")
		editorPath, _ := cmd.Flags().GetString("editor")
		labels, _ := cmd.Flags().GetString("labels")
		reactions, _ := cmd.Flags().GetStringSlice("reactions")
		assignees, _ := cmd.Flags().GetString("assignees")
		targetID, _ := cmd.Flags().GetInt("id")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		// When target post ID is unset and the current HEAD is a post reference,
		// use the reference short name as the post ID
		if !forceNew && targetID == 0 {
			head, err := curRepo.Head()
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
			} else if plumbing.IsIssueReference(head) {
				id := plumbing.GetReferenceShortName(head)
				targetID, _ = strconv.Atoi(id)
			} else {
				log.Fatal("HEAD is not an issue reference")
			}
		}

		if editorPath != "" {
			useEditor = true
		}

		issueCreateArgs := &issuecmd.IssueCreateArgs{
			ID:                 targetID,
			Title:              title,
			Body:               body,
			NoBody:             noBody,
			ReplyHash:          commentCommitID,
			Reactions:          funk.UniqString(reactions),
			UseEditor:          useEditor,
			EditorPath:         editorPath,
			Open:               open,
			StdOut:             os.Stdout,
			StdIn:              os.Stdin,
			PostCommentCreator: plumbing.CreatePostCommit,
			EditorReader:       util.ReadFromEditor,
			InputReader:        util.ReadInput,
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

		if err := issuecmd.IssueCreateCmd(curRepo, issueCreateArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueListCmd represents a sub-command to list all issues
var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		reverse, _ := cmd.Flags().GetBool("reverse")
		dateFmt, _ := cmd.Flags().GetString("date")
		format, _ := cmd.Flags().GetString("format")
		noPager, _ := cmd.Flags().GetBool("no-pager")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueListCmd(curRepo, &issuecmd.IssueListArgs{
			Limit:      limit,
			Reverse:    reverse,
			DateFmt:    dateFmt,
			PostGetter: plumbing.GetPosts,
			PagerWrite: common.WriteToPager,
			Format:     format,
			NoPager:    noPager,
			StdOut:     os.Stdout,
			StdErr:     os.Stderr,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueReadCmd represents a sub-command to read an issue
var issueReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		reverse, _ := cmd.Flags().GetBool("reverse")
		dateFmt, _ := cmd.Flags().GetString("date")
		format, _ := cmd.Flags().GetString("format")
		noPager, _ := cmd.Flags().GetBool("no-pager")
		noCloseStatus, _ := cmd.Flags().GetBool("no-close-status")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueReadCmd(curRepo, &issuecmd.IssueReadArgs{
			Reference:     getIssueRef(curRepo, args),
			Limit:         limit,
			Reverse:       reverse,
			DateFmt:       dateFmt,
			Format:        format,
			PagerWrite:    common.WriteToPager,
			PostGetter:    plumbing.GetPosts,
			NoPager:       noPager,
			NoCloseStatus: noCloseStatus,
			StdOut:        os.Stdout,
			StdErr:        os.Stderr,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueCloseCmd represents a sub-command to close an issue
var issueCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueCloseCmd(curRepo, &issuecmd.IssueCloseArgs{
			Reference:          getIssueRef(curRepo, args),
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueReopenCmd represents a sub-command to reopen a merge request
var issueReopenCmd = &cobra.Command{
	Use:   "reopen",
	Short: "Reopen a closed issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueReopenCmd(curRepo, &issuecmd.IssueReopenArgs{
			Reference:          getIssueRef(curRepo, args),
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueStatusCmd represents a sub-command to check status of an issue
var issueStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = issuecmd.IssueStatusCmd(curRepo, &issuecmd.IssueStatusArgs{
			Reference:    getIssueRef(curRepo, args),
			ReadPostBody: plumbing.ReadPostBody,
			StdOut:       os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	issueCmd.AddCommand(issueCreateCmd)
	issueCmd.AddCommand(issueReadCmd)
	issueCmd.AddCommand(issueListCmd)
	issueCmd.AddCommand(issueCloseCmd)
	issueCmd.AddCommand(issueReopenCmd)
	issueCmd.AddCommand(issueStatusCmd)
	rootCmd.AddCommand(issueCmd)

	issueCmd.PersistentFlags().Bool("no-pager", false, "Prevent output from being piped into a pager")
	issueCreateCmd.Flags().StringP("title", "t", "", "The issue title (max. 250 B)")
	issueCreateCmd.Flags().StringP("body", "b", "", "The issue message (max. 8 KB)")
	issueCreateCmd.Flags().StringP("reply", "r", "", "Specify the hash of a comment to respond to")
	issueCreateCmd.Flags().StringSliceP("reactions", "e", nil, "Add reactions to a reply (max. 10)")
	issueCreateCmd.Flags().StringP("labels", "l", "", "Specify labels to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().StringP("assignees", "a", "", "Specify push key of assignees to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	issueCreateCmd.Flags().Bool("no-body", false, "Skip prompt for issue body")
	issueCreateCmd.Flags().Bool("new", false, "Force new issue to be created instead of adding a comment to HEAD")
	issueCreateCmd.Flags().String("editor", "", "Specify an editor to use instead of the git configured editor")
	issueCreateCmd.Flags().IntP("id", "i", 0, "Specify a target issue number")
	issueCreateCmd.Flags().BoolP("close", "c", false, "Close the issue")
	issueCreateCmd.Flags().BoolP("reopen", "o", false, "Open a closed issue")
	issueReadCmd.Flags().Bool("no-close-status", false, "Hide the close status indicator")

	var commonIssueFlags = func(commands ...*cobra.Command) {
		for _, cmd := range commands {
			cmd.Flags().IntP("limit", "n", 0, "Limit the number of records to returned")
			cmd.Flags().Bool("reverse", false, "Return the result in reversed order")
			cmd.Flags().StringP("date", "d", "Mon Jan _2 15:04:05 2006 -0700", "Set date format")
			cmd.Flags().StringP("format", "f", "", "Set output format")
		}
	}

	commonIssueFlags(issueListCmd, issueReadCmd)
}
