package mergecmd

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/themakeos/lobe/commands/common"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/util"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type MergeRequestListArgs struct {

	// Limit sets a hard limit on the number of merge requests to display
	Limit int

	// Reverse indicates that the merge requests should be listed in reverse order
	Reverse bool

	// DateFmt is the date format to use for displaying dates
	DateFmt string

	// PostGetter is the function used to get merge-request posts
	PostGetter plumbing2.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite common.PagerWriter

	// Format specifies a format to use for generating each post output to Stdout.
	// The following place holders are supported:
	// - %i%    - Index of the post
	// - %bb%	- Base branch name
	// - %bh%	- Base branch hash
	// - %tb%	- Target branch name
	// - %th%	- Target branch hash
	// - %a% 	- Author of the post
	// - %e% 	- Author email
	// - %t% 	- Title of the post
	// - %c% 	- The body/preview of the post
	// - %d% 	- Date of creation
	// - %H%    - The full hash of the first comment
	// - %h%    - The short hash of the first comment
	// - %n%  	- The reference name of the post
	// - %pk% 	- The pushers push key ID
	Format string

	// NoPager indicates that output must not be piped into a pager
	NoPager bool

	StdOut io.Writer
	StdErr io.Writer
}

// MergeRequestListCmd list all merge requests
func MergeRequestListCmd(targetRepo types.LocalRepo, args *MergeRequestListArgs) error {

	// Get merge requests posts
	mergeReqs, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return plumbing2.IsMergeRequestReference(ref.String())
	})
	if err != nil {
		return errors.Wrap(err, "failed to get merge requests posts")
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

	return formatAndPrintMergeRequestList(targetRepo, args, mergeReqs)
}

func formatAndPrintMergeRequestList(targetRepo types.LocalRepo, args *MergeRequestListArgs, mergeReqs plumbing2.Posts) error {
	buf := bytes.NewBuffer(nil)
	for i, mr := range mergeReqs {

		// Format date if date format is specified
		date := mr.Comment().Created.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", mr.Comment().Created.Unix())
			case "utc":
				date = mr.Comment().Created.UTC().String()
			case "rfc3339":
				date = mr.Comment().Created.Format(time.RFC3339)
			case "rfc822":
				date = mr.Comment().Created.Format(time.RFC822)
			default:
				date = mr.Comment().Created.Format(args.DateFmt)
			}
		}

		var baseFmt string
		if mr.Comment().Body.BaseBranch != "" {
			baseFmt = "\nBase Branch:    %bb%"
		}

		var baseHashFmt string
		if mr.Comment().Body.BaseBranchHash != "" {
			baseHashFmt = "\nBase Hash:      %bh%"
		}

		var targetFmt string
		if mr.Comment().Body.TargetBranch != "" {
			targetFmt = "\nTarget Branch:  %tb%"
		}

		var targetHashFmt string
		if mr.Comment().Body.TargetBranchHash != "" {
			targetHashFmt = "\nTarget Hash:    %th%"
		}

		pusherKeyFmt := ""
		if mr.Comment().Pusher != "" {
			pusherKeyFmt = "\nPusher:         %pk%"
		}

		// Extract preview
		preview := plumbing2.GetCommentPreview(mr.Comment())

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + fmt2.YellowString("merge-request %H% %n%") + `
Title:          %t%` + baseFmt + `` + baseHashFmt + `` + targetFmt + `` + targetHashFmt + `
Author:         %a% <%e%>` + pusherKeyFmt + `
Date:           %d%
%c%
`
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"i":  i,
			"bb": mr.Comment().Body.BaseBranch,
			"bh": mr.Comment().Body.BaseBranchHash,
			"tb": mr.Comment().Body.TargetBranch,
			"th": mr.Comment().Body.TargetBranchHash,
			"a":  mr.Comment().Author,
			"e":  mr.Comment().AuthorEmail,
			"t":  mr.GetTitle(),
			"c":  preview,
			"d":  date,
			"H":  mr.Comment().Hash,
			"h":  mr.Comment().Hash[:7],
			"n":  plumbing.ReferenceName(mr.GetName()).Short(),
			"pk": mr.Comment().Pusher,
		}

		if i > 0 {
			buf.WriteString("\n")
		}

		str, err := util.MustacheParseString(format, data, util.MustacheParserOpt{
			ForceRaw: true, StartTag: "%", EndTag: "%"})
		if err != nil {
			return errors.Wrap(err, "failed to parse format")
		}

		_, err = buf.WriteString(str)
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
