package issuecmd

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type IssueListArgs struct {

	// Limit sets a hard limit on the number of issues to display
	Limit int

	// Reverse indicates that the issues should be listed in reverse order
	Reverse bool

	// DateFmt is the date format to use for displaying dates
	DateFmt string

	// PostGetter is the function used to get issue posts
	PostGetter plumbing2.PostGetter

	// PagerWrite is the function used to write to a pager
	PagerWrite pagerWriter

	// Format specifies a format to use for generating each post output to Stdout.
	// The following place holders are supported:
	// - %i%    - Index of the post
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

	StdOut io.Writer
	StdErr io.Writer
}

// IssueListCmd list all issues
func IssueListCmd(targetRepo core.LocalRepo, args *IssueListArgs) error {

	// Get issue posts
	issues, err := args.PostGetter(targetRepo, func(ref plumbing.ReferenceName) bool {
		return plumbing2.IsIssueReference(ref.String())
	})
	if err != nil {
		return errors.Wrap(err, "failed to get issue posts")
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

	return formatAndPrintIssueList(targetRepo, args, issues)
}

func formatAndPrintIssueList(targetRepo core.LocalRepo, args *IssueListArgs, issues plumbing2.Posts) error {
	buf := bytes.NewBuffer(nil)
	for i, issue := range issues {

		// Format date if date format is specified
		date := issue.First.Created.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", issue.First.Created.Unix())
			case "utc":
				date = issue.First.Created.UTC().String()
			case "rfc3339":
				date = issue.First.Created.Format(time.RFC3339)
			case "rfc822":
				date = issue.First.Created.Format(time.RFC822)
			default:
				date = issue.First.Created.Format(args.DateFmt)
			}
		}

		pusherKeyFmt := ""
		if issue.First.Pusher != "" {
			pusherKeyFmt = "\nPusher: %pk%"
		}

		// Extract preview
		preview := plumbing2.GetCommentPreview(issue.First)

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + color.YellowString("issue %H% %n%") + `
Author: %a% <%e%>` + pusherKeyFmt + `
Title:  %t%
Date:   %d%
%c%
`
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"i":  i,
			"a":  issue.First.Author,
			"e":  issue.First.AuthorEmail,
			"t":  issue.Title,
			"c":  preview,
			"d":  date,
			"H":  issue.First.Hash,
			"h":  issue.First.Hash[:7],
			"n":  plumbing.ReferenceName(issue.Name).Short(),
			"pk": issue.First.Pusher,
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

	args.PagerWrite(pagerCmd, buf, args.StdOut, args.StdErr)
	return nil
}

// pagerWriter describes a function for writing a specified content to a pager program
type pagerWriter func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer)

// WriteToPager spawns the specified page, passing the given content to it
func WriteToPager(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
	args := strings.Split(pagerCmd, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = content
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(stdOut, err.Error())
		fmt.Fprint(stdOut, content)
		return
	}
}
