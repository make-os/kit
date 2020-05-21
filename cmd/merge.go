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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/remote/cmd/mergecmd"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/util"
)

// mergeReqCmd represents the issue command
var mergeReqCmd = &cobra.Command{
	Use:   "mr",
	Short: "Create, read, list and respond to merge requests",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// mergeReqCreateCmd represents a sub-command to create an issue
var mergeReqCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a merge request or add a comment to an existing one",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		rejectFlagCombo(cmd, "close", "reopen")
		requireFlag(cmd, "base", "target")

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		noBody, _ := cmd.Flags().GetBool("no-body")
		cls, _ := cmd.Flags().GetBool("close")
		open, _ := cmd.Flags().GetBool("reopen")
		editorPath, _ := cmd.Flags().GetString("editor")
		reactions, _ := cmd.Flags().GetStringSlice("reactions")
		mergeReqID, _ := cmd.Flags().GetInt("id")
		baseBranch, _ := cmd.Flags().GetString("base")
		baseBranchHash, _ := cmd.Flags().GetString("baseHash")
		targetBranch, _ := cmd.Flags().GetString("target")
		targetBranchHash, _ := cmd.Flags().GetString("targetHash")

		r, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if editorPath != "" {
			useEditor = true
		}

		// When target branch hash is not provided or is equal to '.', automatically
		// read the latest reference hash of the target branch.
		if targetBranchHash == "" || targetBranchHash == "." {
			ref, err := r.RefGet(targetBranch)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get target branch").Error())
			}
			targetBranchHash = ref
		}

		// When base branch hash is '.', we take this to mean the user wants us to
		// automatically read the latest reference hash of the base branch. We chose solely
		// this convention over an empty value because an empty base hash is interpreted
		// as zero hash value by the network.
		if baseBranchHash == "." {
			ref, err := r.RefGet(baseBranch)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get base branch").Error())
			}
			baseBranchHash = ref
		}

		mrCreateArgs := &mergecmd.MergeRequestCreateArgs{
			MergeRequestNumber: mergeReqID,
			Title:              title,
			Body:               body,
			NoBody:             noBody,
			ReplyHash:          commentCommitID,
			Reactions:          funk.UniqString(reactions),
			UseEditor:          useEditor,
			EditorPath:         editorPath,
			Open:               open,
			Base:               baseBranch,
			BaseHash:           baseBranchHash,
			Target:             targetBranch,
			TargetHash:         targetBranchHash,
			StdOut:             os.Stdout,
			StdIn:              os.Stdin,
			PostCommentCreator: plumbing.CreatePostCommit,
			EditorReader:       util.ReadFromEditor,
			InputReader:        util.ReadInput,
		}

		if cmd.Flags().Changed("close") {
			mrCreateArgs.Close = &cls
		}

		if err := mergecmd.MergeRequestCreateCmd(r, mrCreateArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(mergeReqCmd)

	mergeReqCmd.PersistentFlags().SortFlags = false
	mergeReqCmd.Flags().SortFlags = false
	mergeReqCmd.PersistentFlags().Bool("no-pager", false, "Prevent output from being piped into a pager")
	mergeReqCmd.AddCommand(mergeReqCreateCmd)

	mergeReqCreateCmd.Flags().StringP("title", "t", "", "The merge request title (max. 250 B)")
	mergeReqCreateCmd.Flags().StringP("body", "b", "", "The merge request message (max. 8 KB)")
	mergeReqCreateCmd.Flags().StringP("reply", "r", "", "Specify the hash of a comment to respond to")
	mergeReqCreateCmd.Flags().StringSliceP("reactions", "e", nil, "Add reactions to a reply (max. 10)")
	mergeReqCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	mergeReqCreateCmd.Flags().Bool("no-body", false, "Skip prompt for issue body")
	mergeReqCreateCmd.Flags().String("editor", "", "Specify an editor to use instead of the git configured editor")
	mergeReqCreateCmd.Flags().IntP("id", "i", 0, "Specify a unique merge request number")
	mergeReqCreateCmd.Flags().BoolP("close", "c", false, "Close the issue")
	mergeReqCreateCmd.Flags().BoolP("reopen", "o", false, "Open a closed issue")
	mergeReqCreateCmd.Flags().String("base", "", "Specify the base branch name")
	mergeReqCreateCmd.Flags().String("baseHash", "", "Specify the current hash of the base branch")
	mergeReqCreateCmd.Flags().String("target", "", "Specify the target branch name")
	mergeReqCreateCmd.Flags().String("targetHash", "", "Specify the hash of the target branch")

	// issueReadCmd.Flags().Bool("no-close-status", false, "Hide the close status indicator")

	// var commonIssueFlags = func(commands ...*cobra.Command) {
	// 	for _, cmd := range commands {
	// 		cmd.Flags().IntP("limit", "n", 0, "Limit the number of records to returned")
	// 		cmd.Flags().Bool("reverse", false, "Return the result in reversed order")
	// 		cmd.Flags().StringP("date", "d", "Mon Jan _2 15:04:05 2006 -0700", "Set date format")
	// 		cmd.Flags().StringP("format", "f", "", "Set output format")
	// 	}
	// }
	//
	// commonIssueFlags(issueListCmd, issueReadCmd)
}
