package issuecmd

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/cmd/common"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/util"
	cf "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
)

type IssueListArgs struct {

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

	// Format specifies a format to use for generating each post output to Stdout.
	// The following placeholders are supported:
	// - %i    	- Index of the post
	// - %a 	- Author of the post
	// - %e 	- Author email
	// - %t 	- Title of the post
	// - %c 	- The body/preview of the post
	// - %d 	- Date of creation
	// - %H    	- The full hash of the first comment
	// - %h    	- The short hash of the first comment
	// - %n  	- The reference name of the post
	Format string

	// NoPager indicates that output must not be piped into a pager
	NoPager bool

	StdOut io.Writer
	StdErr io.Writer
}

// IssueListCmdFunc describes IssueListCmd function signature
type IssueListCmdFunc func(targetRepo pl.LocalRepo, args *IssueListArgs) (pl.Posts, error)

// IssueListCmd list all issues
func IssueListCmd(targetRepo pl.LocalRepo, args *IssueListArgs) (pl.Posts, error) {

	// Get issue posts
	issues, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return pl.IsIssueReference(ref.String())
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get issue posts")
	}

	// Sort by their first post time
	issues.SortByFirstPostCreationTimeDesc()

	// Reverse issues if requested
	if args.Reverse {
		issues.Reverse()
	}

	// Limited the issues if requested
	if args.Limit > 0 && args.Limit < len(issues) {
		issues = issues[:args.Limit]
	}

	return issues, nil
}

// FormatAndPrintIssueList prints out issues to stdout
func FormatAndPrintIssueList(targetRepo pl.LocalRepo, args *IssueListArgs, issues pl.Posts) error {
	buf := bytes.NewBuffer(nil)
	for i, issue := range issues {

		// Format date if date format is specified
		date := issue.GetComment().CreatedAt.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", issue.GetComment().CreatedAt.Unix())
			case "utc":
				date = issue.GetComment().CreatedAt.UTC().String()
			case "rfc3339":
				date = issue.GetComment().CreatedAt.Format(time.RFC3339)
			case "rfc822":
				date = issue.GetComment().CreatedAt.Format(time.RFC822)
			default:
				date = issue.GetComment().CreatedAt.Format(args.DateFmt)
			}
		}

		// Extract preview
		preview := pl.GetCommentPreview(issue.GetComment())

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + cf.YellowString("issue %H %n") + `
Title:  %t
Author: %a <%e>
Date:   %d
%c
`
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"i": i,
			"a": issue.GetComment().Author,
			"e": issue.GetComment().AuthorEmail,
			"t": issue.GetTitle(),
			"c": preview,
			"d": date,
			"H": issue.GetComment().Hash,
			"h": issue.GetComment().Hash[:7],
			"n": plumbing.ReferenceName(issue.GetName()).Short(),
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
