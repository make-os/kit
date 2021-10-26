package mergecmd

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/cmd/common"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
)

type MergeRequestListArgs struct {

	// Limit sets a hard limit on the number of merge requests to display
	Limit int

	// Reverse indicates that the merge requests should be listed in reverse order
	Reverse bool

	// DateFmt is the date format to use for displaying dates
	DateFmt string

	// PostGetter is the function used to get merge-request posts
	PostGetter pl.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite common.PagerWriter

	// Format specifies a format to use for generating each post output to Stdout.
	// The following place holders are supported:
	// - %i    	- Index of the post
	// - %bb	- Base branch name
	// - %bh	- Base branch hash
	// - %tb	- Target branch name
	// - %th	- Target branch hash
	// - %a 	- Author of the post
	// - %e 	- Author email
	// - %t 	- Title of the post
	// - %c 	- The body/preview of the post
	// - %d 	- Date of creation
	// - %H    	- The full hash of the first comment
	// - %h    	- The short hash of the first comment
	// - %n  	- The reference name of the post
	// - %pk 	- The push key address
	Format string

	// NoPager indicates that output must not be piped into a pager
	NoPager bool

	StdOut io.Writer
	StdErr io.Writer
}

// MergeRequestListCmdFunc describes MergeRequestListCmd function signature
type MergeRequestListCmdFunc func(targetRepo types.LocalRepo, args *MergeRequestListArgs) (pl.Posts, error)

// MergeRequestListCmd list all merge requests
func MergeRequestListCmd(targetRepo types.LocalRepo, args *MergeRequestListArgs) (pl.Posts, error) {

	// Get merge requests posts
	mergeReqs, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return pl.IsMergeRequestReference(ref.String())
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get merge requests posts")
	}

	// Sort by their first post time
	mergeReqs.SortByFirstPostCreationTimeDesc()

	// Reverse merge requests if requested
	if args.Reverse {
		mergeReqs.Reverse()
	}

	// Limited the merge requests if requested
	if args.Limit > 0 && args.Limit < len(mergeReqs) {
		mergeReqs = mergeReqs[:args.Limit]
	}

	return mergeReqs, nil
}

// FormatAndPrintMergeRequestList prints out merge request to stdout
func FormatAndPrintMergeRequestList(targetRepo types.LocalRepo, args *MergeRequestListArgs, mergeReqs pl.Posts) error {
	buf := bytes.NewBuffer(nil)
	for i, mr := range mergeReqs {

		// Format date if date format is specified
		date := mr.GetComment().CreatedAt.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", mr.GetComment().CreatedAt.Unix())
			case "utc":
				date = mr.GetComment().CreatedAt.UTC().String()
			case "rfc3339":
				date = mr.GetComment().CreatedAt.Format(time.RFC3339)
			case "rfc822":
				date = mr.GetComment().CreatedAt.Format(time.RFC822)
			default:
				date = mr.GetComment().CreatedAt.Format(args.DateFmt)
			}
		}

		var baseFmt string
		if mr.GetComment().Body.BaseBranch != "" {
			baseFmt = "\nBase Branch:    %bb"
		}

		var baseHashFmt string
		if mr.GetComment().Body.BaseBranchHash != "" {
			baseHashFmt = "\nBase Hash:      %bh"
		}

		var targetFmt string
		if mr.GetComment().Body.TargetBranch != "" {
			targetFmt = "\nTarget Branch:  %tb"
		}

		var targetHashFmt string
		if mr.GetComment().Body.TargetBranchHash != "" {
			targetHashFmt = "\nTarget Hash:    %th"
		}

		pusherKeyFmt := ""
		if mr.GetComment().Pusher != "" {
			pusherKeyFmt = "\nPusher:         %pk"
		}

		// Extract preview
		preview := pl.GetCommentPreview(mr.GetComment())

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + fmt2.YellowString("merge-request %H %n") + `
Title:          %t` + baseFmt + `` + baseHashFmt + `` + targetFmt + `` + targetHashFmt + `
Author:         %a <%e>` + pusherKeyFmt + `
Date:           %d
%c
`
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"i":  i,
			"bb": mr.GetComment().Body.BaseBranch,
			"bh": mr.GetComment().Body.BaseBranchHash,
			"tb": mr.GetComment().Body.TargetBranch,
			"th": mr.GetComment().Body.TargetBranchHash,
			"a":  mr.GetComment().Author,
			"e":  mr.GetComment().AuthorEmail,
			"t":  mr.GetTitle(),
			"c":  preview,
			"d":  date,
			"H":  mr.GetComment().Hash,
			"h":  mr.GetComment().Hash[:7],
			"n":  plumbing.ReferenceName(mr.GetName()).Short(),
			"pk": mr.GetComment().Pusher,
		}

		if i > 0 {
			buf.WriteString("\n")
		}

		_, err := buf.WriteString(util.ParseVerbs(format, data))
		if err != nil {
			return err
		}
	}

	pagerCmd, err := targetRepo.Var("GIT_PAGER")
	if err != nil {
		return err
	}

	if args.NoPager {
		fmt.Fprint(args.StdOut, buf)
	} else {
		args.PagerWrite(pagerCmd, buf, args.StdOut, args.StdErr)
	}
	return nil
}
