package issuecmd

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sort"
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
	// - %a% 	- Author of the post
	// - %e% 	- Author email
	// - %t% 	- Title of the post
	// - %p% 	- A preview of the body
	// - %d% 	- Date of creation
	// - %H%    - The full hash of the first comment
	// - %h%    - The short hash of the first comment
	Format string

	StdOut io.Writer
	StdErr io.Writer
}

// IssueListCmd list all issues
func IssueListCmd(targetRepo core.LocalRepo, args *IssueListArgs) error {

	// Get issue posts
	issues, err := args.PostGetter(targetRepo, func(ref *plumbing.Reference) bool {
		return plumbing2.IsIssueReference(ref.Name().String())
	})
	if err != nil {
		return errors.Wrap(err, "failed to get issue posts")
	}

	// Sort by latest
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Comment.Created.UnixNano() > issues[j].Comment.Created.UnixNano()
	})

	// Reverse issues if requested
	if args.Reverse {
		for i, j := 0, len(issues)-1; i < j; i, j = i+1, j-1 {
			issues[i], issues[j] = issues[j], issues[i]
		}
	}

	// Limited the issues if requested
	if args.Limit > 0 && args.Limit < len(issues) {
		issues = issues[:args.Limit]
	}

	buf := bytes.NewBuffer(nil)
	for i, issue := range issues {

		// Format date if date format is specified
		date := issue.Comment.Created.String()
		if args.DateFmt != "" {
			switch args.DateFmt {
			case "unix":
				date = fmt.Sprintf("%d", issue.Comment.Created.Unix())
			case "utc":
				date = issue.Comment.Created.UTC().String()
			case "rfc3339":
				date = issue.Comment.Created.Format(time.RFC3339)
			case "rfc822":
				date = issue.Comment.Created.Format(time.RFC822)
			default:
				date = issue.Comment.Created.Format(args.DateFmt)
			}
		}

		// Extract preview
		preview := plumbing2.GetCommentPreview(issue.Comment)

		// Get format or use default
		var format = args.Format
		if format == "" {
			format = `` + color.YellowString("issue %H%") + `
Author: %a% <%e%>
Title:  %t%
Date:   %d%
%p%
`
		}

		// Define the data for format parsing
		data := map[string]interface{}{
			"a": issue.Comment.Author,
			"e": issue.Comment.AuthorEmail,
			"t": issue.Title,
			"p": preview,
			"d": date,
			"H": issue.Comment.Hash,
			"h": issue.Comment.Hash[:7],
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
