package issuecmd

import (
	"os"
	"strconv"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/util"
	cmdutil "github.com/make-os/kit/util/cmd"
	"github.com/make-os/kit/util/io"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
)

var (
	cfg = config.GetConfig()
	log = cfg.G().Log
)

// IssueCmd represents the issue command
var IssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Create, read, list and respond to issues",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// issueCreateCmd represents a sub-command to create an issue
var issueCreateCmd = &cobra.Command{
	Use:   "create [flags]",
	Short: "Create an issue or add a comment to an existing issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		util.FatalOnError(cmdutil.RejectFlagCombo(cmd, "close", "reopen"))

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		noBody, _ := cmd.Flags().GetBool("no-body")
		cls, _ := cmd.Flags().GetBool("close")
		forceNew, _ := cmd.Flags().GetBool("new")
		force, _ := cmd.Flags().GetBool("force")
		reopen, _ := cmd.Flags().GetBool("reopen")
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
			}
		}

		if editorPath != "" {
			useEditor = true
		}

		issueCreateArgs := &IssueCreateArgs{
			ID:                 targetID,
			Title:              title,
			Body:               body,
			NoBody:             noBody,
			ReplyHash:          commentCommitID,
			Reactions:          funk.UniqString(reactions),
			UseEditor:          useEditor,
			EditorPath:         editorPath,
			Force:              force,
			StdOut:             os.Stdout,
			StdIn:              os.Stdin,
			PostCommentCreator: plumbing.CreatePostCommit,
			EditorReader:       util.ReadFromEditor,
			InputReader:        io.ReadInput,
		}

		if cmd.Flags().Changed("close") {
			issueCreateArgs.Close = &cls
		}

		if cmd.Flags().Changed("reopen") {
			issueCreateArgs.Close = pointer.ToBool(reopen)
		}

		if cmd.Flags().Changed("labels") {
			if labels == "" {
				issueCreateArgs.Labels = []string{}
			} else {
				labels := funk.UniqString(strings.Split(labels, ","))
				issueCreateArgs.Labels = labels
			}
		}

		if cmd.Flags().Changed("assignees") {
			if assignees == "" {
				issueCreateArgs.Assignees = []string{}
			} else {
				assignees := funk.UniqString(strings.Split(assignees, ","))
				issueCreateArgs.Assignees = assignees
			}
		}

		if _, err := IssueCreateCmd(curRepo, issueCreateArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueListCmd represents a sub-command to list all issues
var issueListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "List all issues in the current repository",
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

		if err = IssueListCmd(curRepo, &IssueListArgs{
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
	Use:   "read [flags] [<issueId>]",
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

		if err = IssueReadCmd(curRepo, &IssueReadArgs{
			Reference:     NormalizeIssueReferenceName(curRepo, args),
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
	Use:   "close [flags] [<issueId>]",
	Short: "Close an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if _, err = IssueCloseCmd(curRepo, &IssueCloseArgs{
			Reference:          NormalizeIssueReferenceName(curRepo, args),
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
			Force:              force,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueReopenCmd represents a sub-command to reopen a merge request
var issueReopenCmd = &cobra.Command{
	Use:   "reopen [options] [<issueId>]",
	Short: "Reopen a closed issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if _, err = IssueReopenCmd(curRepo, &IssueReopenArgs{
			Reference:          NormalizeIssueReferenceName(curRepo, args),
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
			Force:              force,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// issueStatusCmd represents a sub-command to check status of an issue
var issueStatusCmd = &cobra.Command{
	Use:   "status [<issueId>]",
	Short: "Get the status of an issue",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = IssueStatusCmd(curRepo, &IssueStatusArgs{
			Reference:    NormalizeIssueReferenceName(curRepo, args),
			ReadPostBody: plumbing.ReadPostBody,
			StdOut:       os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	IssueCmd.AddCommand(issueCreateCmd)
	IssueCmd.AddCommand(issueReadCmd)
	IssueCmd.AddCommand(issueListCmd)
	IssueCmd.AddCommand(issueCloseCmd)
	IssueCmd.AddCommand(issueReopenCmd)
	IssueCmd.AddCommand(issueStatusCmd)

	IssueCmd.PersistentFlags().Bool("no-pager", false, "Prevent output from being piped into a pager")
	issueCreateCmd.Flags().StringP("title", "t", "", "The title of the issue (max. 250 B)")
	issueCreateCmd.Flags().StringP("body", "b", "", "The body of the issue (max. 8 KB)")
	issueCreateCmd.Flags().StringP("reply", "r", "", "Specify the hash of a comment to respond to")
	issueCreateCmd.Flags().StringSliceP("reactions", "e", nil, "Add reactions to a reply (max. 10)")
	issueCreateCmd.Flags().StringP("labels", "l", "", "Specify labels to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().StringP("assignees", "a", "", "Specify push key of assignees to add to the issue/comment (max. 10)")
	issueCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git's `core.editor` program to write the body")
	issueCreateCmd.Flags().Bool("no-body", false, "Skip prompt for issue body")
	issueCreateCmd.Flags().Bool("new", false, "Force a new issue to be created instead of adding a comment to HEAD")
	issueCreateCmd.Flags().String("editor", "", "Specify an editor to use instead of the git configured editor")
	issueCreateCmd.Flags().IntP("id", "i", 0, "Specify a target issue number")
	issueCreateCmd.Flags().BoolP("close", "c", false, "Close the issue")
	issueCreateCmd.Flags().BoolP("reopen", "o", false, "Open a closed issue")
	issueCreateCmd.Flags().BoolP("force", "f", false, "Forcefully create the close comment (uncommitted changes will be lost)")
	issueReadCmd.Flags().Bool("no-close-status", false, "Hide the close status indicator")

	issueCloseCmd.Flags().BoolP("force", "f", false, "Forcefully create the close comment (uncommitted changes will be lost)")
	issueReopenCmd.Flags().BoolP("force", "f", false, "Forcefully create the close comment (uncommitted changes will be lost)")

	var commonIssueFlags = func(commands ...*cobra.Command) {
		for _, cmd := range commands {
			cmd.Flags().IntP("limit", "n", 0, "Limit the number of records returned")
			cmd.Flags().Bool("reverse", false, "Return the result in reversed order")
			cmd.Flags().StringP("date", "d", "Mon Jan _2 15:04:05 2006 -0700", "Set date format")
			cmd.Flags().StringP("format", "f", "", "Set output format")
		}
	}

	commonIssueFlags(issueListCmd, issueReadCmd)
}
