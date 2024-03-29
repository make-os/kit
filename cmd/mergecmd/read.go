package mergecmd

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/logrusorgru/aurora"
	"github.com/make-os/kit/cmd/common"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
)

// MergeRequestReadArgs contains arguments used by MergeRequestReadCmd function
type MergeRequestReadArgs struct {

	// MergeRequestPath is the full path to the merge request post
	Reference string

	// Limit sets a hard limit on the number of merge requests to display
	Limit int

	// Reverse indicates that the merge requests should be listed in reverse order
	Reverse bool

	// DateFmt is the date format to use for displaying dates
	DateFmt string

	// PostGetter is the function used to get merge request posts
	PostGetter pl.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite common.PagerWriter

	// Format specifies a format to use for generating each comment output to Stdout.
	// The following placeholders are supported:
	// - %i    	- Index of the comment
	// - %i    	- Index of the post
	// - %bb	- Base branch name
	// - %bh	- Base branch hash
	// - %tb	- Target branch name
	// - %th	- Target branch hash
	// - %a 	- Author of the comment
	// - %e 	- Author email
	// - %t 	- Title of the comment
	// - %c 	- The body of the comment
	// - %d 	- Date of creation
	// - %H    	- The full hash of the comment
	// - %h    	- The short hash of the comment
	// - %l 	- The label attached to the comment
	// - %as 	- The assignees attached to the comment
	// - %r 	- The short commit hash the current comment is replying to.
	// - %R 	- The full commit hash the current comment is replying to.
	// - %rs 	- The comment's reactions.
	// - %cl 	- Flag for close status of the post (true/false)
	Format string

	// NoPager indicates that output must not be piped into a pager
	NoPager bool

	// NoCloseStatus indicates that the close status must not be rendered
	NoCloseStatus bool

	StdOut io.Writer
	StdErr io.Writer
}

// MergeRequestReadCmdFunc describes MergeRequestReadCmd function signature
type MergeRequestReadCmdFunc func(targetRepo pl.LocalRepo, args *MergeRequestReadArgs) (pl.Comments, error)

// MergeRequestReadCmd read comments in a merge request post
func MergeRequestReadCmd(targetRepo pl.LocalRepo, args *MergeRequestReadArgs) (pl.Comments, error) {

	// Find the target merge request
	res, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return pl.IsMergeRequestReference(ref.String()) && ref.String() == args.Reference
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find merge request")
	} else if len(res) == 0 {
		return nil, fmt.Errorf("merge request not found")
	}

	isClosed, err := res[0].IsClosed()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check close status")
	}

	// Get all comments in the merge request
	comments, err := res[0].GetComments()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments")
	}

	// Reverse the merge requests if requested
	if args.Reverse {
		comments.Reverse()
	}

	// Limited the merge requests if requested
	if args.Limit > 0 && args.Limit < len(comments) {
		comments = comments[:args.Limit]
	}

	if args.StdOut != nil {
		if err = formatAndPrintMergeRequestComments(targetRepo, args, isClosed, res[0].GetTitle(), comments); err != nil {
			return nil, err
		}
	}

	return comments, nil
}

func formatAndPrintMergeRequestComments(
	targetRepo pl.LocalRepo,
	args *MergeRequestReadArgs,
	isClosed bool,
	title string,
	comments pl.Comments) error {

	buf := bytes.NewBuffer(nil)

	padding := strings.Repeat(" ", 25)
	closeFmt := fmt2.NewColor(aurora.Bold, aurora.BgBlue, aurora.White).Sprintf(padding + "CLOSED" + padding)
	if isClosed && !args.NoCloseStatus {
		buf.WriteString(closeFmt)
		buf.WriteString("\n")
	}

	for i, comment := range comments {

		// Format date if date format is specified
		date := comment.CreatedAt.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", comment.CreatedAt.Unix())
			case "utc":
				date = comment.CreatedAt.UTC().String()
			case "rfc3339":
				date = comment.CreatedAt.Format(time.RFC3339)
			case "rfc822":
				date = comment.CreatedAt.Format(time.RFC822)
			default:
				date = comment.CreatedAt.Format(args.DateFmt)
			}
		}

		content := strings.TrimSpace(string(comment.Body.Content))

		if !args.Reverse {
			i = int(math.Abs(float64(i - len(comments) + 1)))
		}

		titleCpy := title
		if i > 0 {
			titleCpy = "RE: " + title
		}

		var replyToFmt, replyTo string
		if comment.Body.ReplyTo != "" {
			replyToFmt = "\nReplyTo:    %R"
			replyTo = comment.Body.ReplyTo
		}

		var baseFmt string
		if comment.Body.BaseBranch != "" {
			baseFmt = "\nBase Branch:    %bb"
		}

		var baseHashFmt string
		if comment.Body.BaseBranchHash != "" {
			baseHashFmt = "\nBase Hash:      %bh"
		}

		var targetFmt string
		if comment.Body.TargetBranch != "" {
			targetFmt = "\nTarget Branch:  %tb"
		}

		var targetHashFmt string
		if comment.Body.TargetBranchHash != "" {
			targetHashFmt = "\nTarget Hash:    %th"
		}

		var reactions, reactionsFmt string
		if reactionsMap := comment.GetReactions(); len(reactionsMap) > 0 {
			var reactionsCountMap []string
			for name, count := range reactionsMap {
				if count > 0 {
					if code, ok := util.EmojiCodeMap[fmt.Sprintf(":%s:", name)]; ok {
						reactionsCountMap = append(reactionsCountMap, fmt.Sprintf("%s: %d", code, count))
					}
				}
			}
			reactions = strings.Join(reactionsCountMap, ", ")
		}
		if reactions != "" {
			reactionsFmt = "\nReactions:  %rs"
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"i":  i,
			"bb": comment.Body.BaseBranch,
			"bh": comment.Body.BaseBranchHash,
			"tb": comment.Body.TargetBranch,
			"th": comment.Body.TargetBranchHash,
			"a":  comment.Author,
			"e":  comment.AuthorEmail,
			"t":  titleCpy,
			"c":  content,
			"d":  date,
			"H":  comment.Hash,
			"h":  comment.Hash[:7],
			"R":  replyTo,
			"r":  comment.Body.ReplyTo,
			"rs": reactions,
			"cl": isClosed,
		}

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + fmt2.YellowString("comment %H #%i") + `
Title:          %t` + baseFmt + `` + baseHashFmt + `` + targetFmt + `` + targetHashFmt + `
Author:         %a <%e> 
Date:           %d` + replyToFmt + `` + reactionsFmt + `

%c
`
		}

		buf.WriteString(util.ParseVerbs(format, data))
		buf.WriteString("\n")
	}

	if isClosed && !args.NoCloseStatus {
		buf.WriteString(closeFmt)
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
