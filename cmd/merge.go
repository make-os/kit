package cmd

import (
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/cmd/mergecmd"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/repo"
	"github.com/themakeos/lobe/util"
	"github.com/themakeos/lobe/util/io"
	"github.com/thoas/go-funk"
	plumb "gopkg.in/src-d/go-git.v4/plumbing"
)

// mergeReqCmd represents the merge request command
var mergeReqCmd = &cobra.Command{
	Use:   "mr",
	Short: "Create, read, list and respond to merge requests",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// mergeReqCreateCmd represents a sub-command to create a merge request
var mergeReqCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a merge request or add a comment to an existing one",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		rejectFlagCombo(cmd, "close", "reopen")

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		commentCommitID, _ := cmd.Flags().GetString("reply")
		useEditor, _ := cmd.Flags().GetBool("use-editor")
		noBody, _ := cmd.Flags().GetBool("no-body")
		cls, _ := cmd.Flags().GetBool("close")
		open, _ := cmd.Flags().GetBool("reopen")
		forceNew, _ := cmd.Flags().GetBool("new")
		editorPath, _ := cmd.Flags().GetString("editor")
		reactions, _ := cmd.Flags().GetStringSlice("reactions")
		targetPostID, _ := cmd.Flags().GetInt("id")
		baseBranch, _ := cmd.Flags().GetString("base")
		baseBranchHash, _ := cmd.Flags().GetString("baseHash")
		targetBranch, _ := cmd.Flags().GetString("target")
		targetBranchHash, _ := cmd.Flags().GetString("targetHash")
		force, _ := cmd.Flags().GetBool("force")

		r, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		head, err := r.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}

		// When target post ID is unset and the current HEAD is a post reference,
		// use the reference short name as the post ID
		if !forceNew && targetPostID == 0 {
			if plumbing.IsMergeRequestReference(head) {
				id := plumbing.GetReferenceShortName(head)
				targetPostID, _ = strconv.Atoi(id)
			}
		}

		if editorPath != "" {
			useEditor = true
		}

		// When target branch hash is equal to '.', use the hash of the HEAD reference.
		// but when it is "~", read the hash from the target branch.
		// If target branch is of the format /repo/branch_name, use `branch_name` as target
		if targetBranchHash == "." || targetBranchHash == "~" {
			if targetBranch == "" {
				log.Fatal("flag (--target) is required")
			}
			targetRef := targetBranch
			if targetBranchHash == "." {
				targetRef = head
			} else {
				targetRef = targetBranch
			}
			if targetRef[:1] == "/" {
				parts := strings.SplitN(targetRef[1:], "/", 2)
				if len(parts) == 1 {
					targetRef = parts[0]
				} else {
					targetRef = parts[1]
				}
			}
			ref, err := r.RefGet(targetRef)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get target branch").Error())
			}
			targetBranchHash = ref
		}

		// When base branch hash is equal to '.', use the hash of the HEAD reference.
		// but when it is "~", read the hash from the base branch.
		if baseBranchHash == "." || baseBranchHash == "~" {
			if baseBranch == "" {
				log.Fatal("flag (--base) is required")
			}
			targetRef := baseBranch
			if baseBranchHash == "." {
				targetRef = head
			} else {
				targetRef = targetBranch
			}
			ref, err := r.RefGet(targetRef)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get base branch").Error())
			}
			baseBranchHash = ref
		}

		// If base branch is unset, use current HEAD
		if baseBranch == "" {
			baseBranch = plumb.ReferenceName(head).Short()
		}

		// If base hash is unset, use current hash of base branch
		if baseBranchHash == "" {
			baseBranchHash, _ = r.RefGet(baseBranch)
		}

		mrCreateArgs := &mergecmd.MergeRequestCreateArgs{
			ID:                 targetPostID,
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
			Force:              force,
			StdOut:             os.Stdout,
			StdIn:              os.Stdin,
			PostCommentCreator: plumbing.CreatePostCommit,
			EditorReader:       util.ReadFromEditor,
			InputReader:        io.ReadInput,
		}

		if cmd.Flags().Changed("close") {
			mrCreateArgs.Close = &cls
		}

		if err := mergecmd.MergeRequestCreateCmd(r, mrCreateArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// mergeListCmd represents a sub-command to list all merge requests
var mergeReqListCmd = &cobra.Command{
	Use:   "list",
	Short: "List merge requests",
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

		if err = mergecmd.MergeRequestListCmd(curRepo, &mergecmd.MergeRequestListArgs{
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

// mergeReqReadCmd represents a sub-command to read a merge request
var mergeReqReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read a merge request",
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

		if err = mergecmd.MergeRequestReadCmd(curRepo, &mergecmd.MergeRequestReadArgs{
			Reference:     getMergeRef(curRepo, args),
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

// mergeReqCloseCmd represents a sub-command to close a merge request
var mergeReqCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a merge request",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = mergecmd.MergeReqCloseCmd(curRepo, &mergecmd.MergeReqCloseArgs{
			Reference:          getMergeRef(curRepo, args),
			Force:              force,
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// mergeReqReopenCmd represents a sub-command to reopen a merge request
var mergeReqReopenCmd = &cobra.Command{
	Use:   "reopen",
	Short: "Reopen a closed merge request",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = mergecmd.MergeReqReopenCmd(curRepo, &mergecmd.MergeReqReopenArgs{
			Reference:          getMergeRef(curRepo, args),
			Force:              force,
			PostCommentCreator: plumbing.CreatePostCommit,
			ReadPostBody:       plumbing.ReadPostBody,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// mergeReqStatusCmd represents a sub-command to check status of a merge request
var mergeReqStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of a merge request",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		if err = mergecmd.MergeReqStatusCmd(curRepo, &mergecmd.MergeReqStatusArgs{
			Reference:    getMergeRef(curRepo, args),
			ReadPostBody: plumbing.ReadPostBody,
			StdOut:       os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// mergeReqCheckoutCmd represents a sub-command to checkout a merge request target/base branch
var mergeReqCheckoutCmd = &cobra.Command{
	Use:   "checkout [[remote] id]",
	Short: "Checkout a merge request target or base branch",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force-checkout")
		forceFetch, _ := cmd.Flags().GetBool("force-fetch")
		base, _ := cmd.Flags().GetBool("base")
		yes, _ := cmd.Flags().GetBool("yes")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		var remote string
		if len(args) == 2 {
			remote = args[0]
			args = args[1:]
		}

		if err = mergecmd.MergeReqCheckoutCmd(curRepo, &mergecmd.MergeReqCheckoutArgs{
			Reference:             getMergeRef(curRepo, args),
			ReadPostBody:          plumbing.ReadPostBody,
			ForceCheckout:         force,
			ForceFetch:            forceFetch,
			Remote:                remote,
			Base:                  base,
			YesCheckoutDiffTarget: yes,
			ConfirmInput:          io.ConfirmInput,
			StdOut:                os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// mergeReqFetchCmd represents a sub-command to fetch a merge request target/base branch
var mergeReqFetchCmd = &cobra.Command{
	Use:   "fetch [[remote] id]",
	Short: "Fetch a merge request target or base branch",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		forceFetch, _ := cmd.Flags().GetBool("force-fetch")
		base, _ := cmd.Flags().GetBool("base")

		curRepo, err := repo.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to open repo at cwd").Error())
		}

		var remote string
		if len(args) == 2 {
			remote = args[0]
			args = args[1:]
		}

		if err = mergecmd.MergeReqFetchCmd(curRepo, &mergecmd.MergeReqFetchArgs{
			Reference:    getMergeRef(curRepo, args),
			ForceFetch:   forceFetch,
			ReadPostBody: plumbing.ReadPostBody,
			Remote:       remote,
			Base:         base,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(mergeReqCmd)

	mergeReqCmd.Flags().SortFlags = false
	mergeReqCmd.PersistentFlags().SortFlags = false
	mergeReqCmd.PersistentFlags().Bool("no-pager", false, "Prevent output from being piped into a pager")
	mergeReqCmd.AddCommand(mergeReqCreateCmd)
	mergeReqCmd.AddCommand(mergeReqListCmd)
	mergeReqCmd.AddCommand(mergeReqReadCmd)
	mergeReqCmd.AddCommand(mergeReqCloseCmd)
	mergeReqCmd.AddCommand(mergeReqReopenCmd)
	mergeReqCmd.AddCommand(mergeReqStatusCmd)
	mergeReqCmd.AddCommand(mergeReqCheckoutCmd)
	mergeReqCmd.AddCommand(mergeReqFetchCmd)

	mergeReqCreateCmd.Flags().StringP("title", "t", "", "The merge request title (max. 250 B)")
	mergeReqCreateCmd.Flags().StringP("body", "b", "", "The merge request message (max. 8 KB)")
	mergeReqCreateCmd.Flags().StringP("reply", "r", "", "Specify the hash of a comment to respond to")
	mergeReqCreateCmd.Flags().StringSliceP("reactions", "e", nil, "Add reactions to a reply (max. 10)")
	mergeReqCreateCmd.Flags().BoolP("use-editor", "u", false, "Use git configured editor to write body")
	mergeReqCreateCmd.Flags().Bool("no-body", false, "Skip prompt for merge request body")
	mergeReqCreateCmd.Flags().String("editor", "", "Specify an editor to use instead of the git configured editor")
	mergeReqCreateCmd.Flags().IntP("id", "i", 0, "Specify a unique merge request number")
	mergeReqCreateCmd.Flags().BoolP("close", "c", false, "Close the merge request")
	mergeReqCreateCmd.Flags().Bool("new", false, "Force new issue to be created instead of adding a comment to HEAD")
	mergeReqCreateCmd.Flags().BoolP("reopen", "o", false, "Re-open a closed merge request")
	mergeReqCreateCmd.Flags().String("base", "", "Specify the base branch name")
	mergeReqCreateCmd.Flags().String("baseHash", "", "Specify the current hash of the base branch")
	mergeReqCreateCmd.Flags().String("target", "", "Specify the target branch name")
	mergeReqCreateCmd.Flags().String("targetHash", "", "Specify the hash of the target branch")
	mergeReqCreateCmd.Flags().BoolP("force", "f", false, "Forcefully create comment (uncommitted changes will be lost)")

	mergeReqReadCmd.Flags().Bool("no-close-status", false, "Hide the close status indicator")

	mergeReqCheckoutCmd.Flags().BoolP("force-checkout", "f", false, "Forcefully checkout while ignoring unsaved local changes")
	mergeReqCheckoutCmd.Flags().Bool("force-fetch", false, "Forcefully fetch the branch (uncommitted changes will be lost)")
	mergeReqCheckoutCmd.Flags().BoolP("base", "b", false, "Checkout the base branch instead of the target branch")
	mergeReqCheckoutCmd.Flags().BoolP("yes", "y", false, "Automatically select 'Yes' for all confirm prompts")

	mergeReqFetchCmd.Flags().Bool("force-fetch", false, "Forcefully fetch the branch (uncommitted changes will be lost)")
	mergeReqFetchCmd.Flags().BoolP("base", "b", false, "Fetch the base branch instead of the target branch")

	mergeReqCloseCmd.Flags().BoolP("force", "f", false, "Forcefully create comment (uncommitted changes will be lost)")
	mergeReqReopenCmd.Flags().BoolP("force", "f", false, "Forcefully create comment (uncommitted changes will be lost)")

	var commonFlags = func(commands ...*cobra.Command) {
		for _, cmd := range commands {
			cmd.Flags().IntP("limit", "n", 0, "Limit the number of merge requests to returned")
			cmd.Flags().Bool("reverse", false, "Return the result in reversed order")
			cmd.Flags().StringP("date", "d", "Mon Jan _2 15:04:05 2006 -0700", "Set date format")
			cmd.Flags().StringP("format", "f", "", "Set output format")
		}
	}

	commonFlags(mergeReqListCmd, mergeReqReadCmd)
}
