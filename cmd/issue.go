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
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/repo/issues"
	repo2 "gitlab.com/makeos/mosdef/repo/repo"
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

// readFromEditor reads input from the specified editor
func readFromEditor(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return "", nil
	}
	defer os.Remove(file.Name())

	args := strings.Split(editor, " ")
	cmd := exec.Command(args[0], append(args[1:], file.Name())...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = stdIn
	if err := cmd.Run(); err != nil {
		return "", err
	}

	bz, err := ioutil.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(bz), nil
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

		if targetIssue != "" && title != "" {
			log.Fatal("title not required when commenting on issue")
		}

		// Get the repository in the current working directory
		targetRepo, err := repo2.GetCurrentWDRepo(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT)
		go func() { <-sigs; os.Stdin.Close() }()
		rdr := bufio.NewReader(os.Stdin)

		// Read title if not provided via flag and replyTo is not set
		if len(title) == 0 && len(replyTo) == 0 {
			fmt.Println(color.HiBlackString("Title: (256 chars) - Press enter to continue"))
			title, _ = rdr.ReadString('\n')
			title = strings.TrimSpace(title)
		}

		// Read body
		if len(body) == 0 && useEditor == false {
			fmt.Println(color.HiBlackString("Body: (8192 chars) - Press ctrl-D to continue"))
			bz, _ := ioutil.ReadAll(rdr)
			body = strings.TrimSpace(string(bz))
		}

		// Read body from editor if required
		if useEditor == true {
			editor := targetRepo.GetConfig("core.editor")
			if editorPath != "" {
				editor = editorPath
			}
			body, err = readFromEditor(editor, os.Stdin, os.Stdout, os.Stderr)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed read body from editor").Error())
			}
		}

		// Create the issue body and prompt user to confirm
		issueBody := issues.MakeIssueBody(title, body, replyTo, labels, assignees, fixers)

		// Create a new issue or add comment commit to existing issue
		newIssue, ref, err := repo.AddIssueOrCommentCommitCmd(targetRepo, targetIssue, issueBody)
		if err != nil {
			log.Fatal(err.Error())
		}

		if newIssue {
			fmt.Println(color.GreenString("New issue created @ %s", ref))
		} else {
			fmt.Println(color.GreenString("New issue comment created @ %s", ref))
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
