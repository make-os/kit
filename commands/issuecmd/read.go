package issuecmd

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/commands/common"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
	fmt2 "gitlab.com/makeos/mosdef/util/colorfmt"
	"gopkg.in/src-d/go-git.v4/plumbing"
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
	PostGetter plumbing2.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite common.PagerWriter

	// Format specifies a format to use for generating each comment output to Stdout.
	// The following place holders are supported:
	// - %i%    - Index of the comment
	// - %a% 	- Author of the comment
	// - %e% 	- Author email
	// - %t% 	- Title of the comment
	// - %c% 	- The body of the comment
	// - %d% 	- Date of creation
	// - %H%    - The full hash of the comment
	// - %h%    - The short hash of the comment
	// - %n%  	- The reference name of the issue
	// - %l% 	- The label attached to the comment
	// - %as% 	- The assignees attached to the comment
	// - %r% 	- The short commit hash the current comment is replying to.
	// - %R% 	- The full commit hash the current comment is replying to.
	// - %rs% 	- The comment's reactions.
	// - %pk% 	- The pushers push key ID
	// - %cl% 	- Flag for close status of the post (true/false)
	Format string

	// NoPager indicates that output must not be piped into a pager
	NoPager bool

	// NoCloseStatus indicates that the close status must not be rendered
	NoCloseStatus bool

	StdOut io.Writer
	StdErr io.Writer
}

// IssueReadCmd read comments in an issue
func IssueReadCmd(targetRepo types.LocalRepo, args *IssueReadArgs) error {

	// Find the target issue
	issues, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return plumbing2.IsIssueReference(ref.String()) && ref.String() == args.Reference
	})
	if err != nil {
		return errors.Wrap(err, "failed to find issue")
	} else if len(issues) == 0 {
		return fmt.Errorf("issue not found")
	}

	isClosed, err := issues[0].IsClosed()
	if err != nil {
		return errors.Wrap(err, "failed to check close status")
	}

	// Get all comments in the issue
	comments, err := issues[0].GetComments()
	if err != nil {
		return errors.Wrap(err, "failed to get comments")
	}

	// Reverse issues if requested
	if args.Reverse {
		comments.Reverse()
	}

	// Limited the issues if requested
	if args.Limit > 0 && args.Limit < len(comments) {
		comments = comments[:args.Limit]
	}

	return formatAndPrintIssueComments(targetRepo, args, isClosed, issues[0].GetTitle(), comments)
}

func formatAndPrintIssueComments(
	targetRepo types.LocalRepo,
	args *IssueReadArgs,
	isClosed bool,
	title string,
	comments plumbing2.Comments) error {

	buf := bytes.NewBuffer(nil)

	padding := strings.Repeat(" ", 25)
	closeFmt := fmt2.NewColor(color.Bold, color.BgBlue, color.FgWhite).Sprintf(padding + "CLOSED" + padding)
	if isClosed && !args.NoCloseStatus {
		buf.WriteString(closeFmt)
		buf.WriteString("\n")
	}

	for i, comment := range comments {

		// Format date if date format is specified
		date := comment.Created.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", comment.Created.Unix())
			case "utc":
				date = comment.Created.UTC().String()
			case "rfc3339":
				date = comment.Created.Format(time.RFC3339)
			case "rfc822":
				date = comment.Created.Format(time.RFC822)
			default:
				date = comment.Created.Format(args.DateFmt)
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
			replyToFmt = "\nReplyTo:    %R%"
			replyTo = comment.Body.ReplyTo
		}

		var assignees, assigneeFmt string
		if comment.Body.Assignees != nil && len(*comment.Body.Assignees) > 0 {
			assignees = strings.Join(*comment.Body.Assignees, ",")
		}
		if assignees != "" {
			assigneeFmt = "\nAssignees:  %as%"
		}

		var labels, labelsFmt string
		if comment.Body.Labels != nil && len(*comment.Body.Labels) > 0 {
			labels = strings.Join(*comment.Body.Labels, ",")
		}
		if labels != "" {
			labelsFmt = "\nLabels:     %l%"
		}

		var reactions, reactionsFmt string
		if reactionsMap := comment.GetReactions(); len(reactionsMap) > 0 {
			reactionsCountMap := []string{}
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
			reactionsFmt = "\nReactions:  %rs%"
		}

		pusherKeyFmt := ""
		if comment.Pusher != "" {
			pusherKeyFmt = "\nPusher:     %pk%"
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
			"n":  plumbing.ReferenceName(comment.Reference).Short(),
			"rs": reactions,
			"pk": comment.Pusher,
			"cl": isClosed,
		}

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + fmt2.YellowString("comments %H% #%i%") + `
Author:     %a% <%e%>` + pusherKeyFmt + `
Title:      %t%
Date:       %d%` + replyToFmt + `` + assigneeFmt + `` + labelsFmt + `` + reactionsFmt + `

%c%
`
		}

		str, err := util.MustacheParseString(format, data, util.MustacheParserOpt{
			ForceRaw: true, StartTag: "%", EndTag: "%"})
		if err != nil {
			return errors.Wrap(err, "failed to parse format")
		}

		buf.WriteString(str)
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
