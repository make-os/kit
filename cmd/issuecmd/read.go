package issuecmd

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

// IssueReadArgs contains arguments used by IssueReadCmd function
type IssueReadArgs struct {

	// Reference is the full reference path to the issue
	Reference string

	// Limit sets a hard limit on the number of issues to display
	Limit int

	// Reverse indicates that the issues should be listed in reverse order
	Reverse bool

	// DateFmt is the date format to use for displaying dates
	DateFmt string

	// PostGetter is the function used to get issue posts
	PostGetter pl.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite common.PagerWriter

	// Format specifies a format to use for generating each comment output to Stdout.
	// The following placeholders are supported:
	// - %i    	- Index of the comment
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

// IssueReadCmdFunc describes IssueReadCmd function signature
type IssueReadCmdFunc func(targetRepo pl.LocalRepo, args *IssueReadArgs) (pl.Comments, error)

// IssueReadCmd read comments in an issue
func IssueReadCmd(targetRepo pl.LocalRepo, args *IssueReadArgs) (pl.Comments, error) {

	// Find the target issue
	issues, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return pl.IsIssueReference(ref.String()) && ref.String() == args.Reference
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find issue")
	} else if len(issues) == 0 {
		return nil, fmt.Errorf("issue not found")
	}

	isClosed, err := issues[0].IsClosed()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check close status")
	}

	// Get all comments in the issue
	comments, err := issues[0].GetComments()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get comments")
	}

	// Reverse issues if requested
	if args.Reverse {
		comments.Reverse()
	}

	// Limited the issues if requested
	if args.Limit > 0 && args.Limit < len(comments) {
		comments = comments[:args.Limit]
	}

	// Format and print if stdout is provided
	if args.StdOut != nil {
		if err = formatAndPrintIssueComments(targetRepo, args, isClosed, issues[0].GetTitle(), comments); err != nil {
			return nil, err
		}
	}

	return comments, nil
}

// formatAndPrintIssueComments prints out an issue to stdout
func formatAndPrintIssueComments(
	targetRepo pl.LocalRepo,
	args *IssueReadArgs,
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

		var assignees, assigneeFmt string
		if comment.Body.Assignees != nil && len(comment.Body.Assignees) > 0 {
			assignees = strings.Join(comment.Body.Assignees, ",")
		}
		if assignees != "" {
			assigneeFmt = "\nAssignees:  %as"
		}

		var labels, labelsFmt string
		if comment.Body.Labels != nil && len(comment.Body.Labels) > 0 {
			labels = strings.Join(comment.Body.Labels, ",")
		}
		if labels != "" {
			labelsFmt = "\nLabels:     %l"
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
			"a":  comment.Author,
			"e":  comment.AuthorEmail,
			"t":  titleCpy,
			"c":  content,
			"d":  date,
			"H":  comment.Hash,
			"h":  comment.Hash[:7],
			"l":  labels,
			"as": assignees,
			"R":  replyTo,
			"r":  comment.Body.ReplyTo,
			"rs": reactions,
			"cl": isClosed,
		}

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + fmt2.YellowString("comments %H #%i") + `
Author:     %a <%e>
Title:      %t
Date:       %d` + replyToFmt + `` + assigneeFmt + `` + labelsFmt + `` + reactionsFmt + `

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
