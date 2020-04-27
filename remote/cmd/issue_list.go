package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// IssueListCmd list all issues
func IssueListCmd(targetRepo core.BareRepo, limit int, reverse bool, dateFmt string) error {

	// Get issue posts
	issues, err := plumbing2.GetPosts(targetRepo, func(ref *plumbing.Reference) bool {
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
	if reverse {
		for i, j := 0, len(issues)-1; i < j; i, j = i+1, j-1 {
			issues[i], issues[j] = issues[j], issues[i]
		}
	}

	// Limited the issues if requested
	if limit > 0 && limit < len(issues) {
		issues = issues[:limit]
	}

	buf := bytes.NewBuffer(nil)
	for i, issue := range issues {

		// Format date if date format is specified
		date := issue.Comment.Created.String()
		if dateFmt != "" {
			switch dateFmt {
			case "unix":
				date = fmt.Sprintf("%d", issue.Comment.Created.Unix())
			case "utc":
				date = issue.Comment.Created.UTC().String()
			case "rfc3339":
				date = issue.Comment.Created.Format(time.RFC3339)
			case "rfc822":
				date = issue.Comment.Created.Format(time.RFC822)
			default:
				date = issue.Comment.Created.Format(dateFmt)
			}
		}

		// Extract preview
		preview := plumbing2.GetCommentPreview(issue.Comment)

		format := `` + color.YellowString("issue %s") + `
Author: %s <%s>
Title:  %s
Date:   %s
%s
`
		str := fmt.Sprintf(format, issue.Comment.Hash, issue.Comment.Author,
			issue.Comment.AuthorEmail, issue.Title, date, preview)

		if i > 0 {
			buf.WriteString("\n")
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

	openInPager(pagerCmd, buf, os.Stdout, os.Stderr)

	return nil
}

// openInPager spawns the specified page, passing the given content to it
func openInPager(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
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
