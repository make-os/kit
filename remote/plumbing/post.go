package plumbing

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/jdkato/prose.v2"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Comment represent a reference post comment
type Comment struct {
	Created     time.Time
	Hash        string
	Author      string
	AuthorEmail string
	Signature   string
	Body        *IssueBody
}

// Post represents a reference post
type Post struct {
	Title   string
	Comment *Comment
}

// PostGetter describes a function for finding posts
type PostGetter func(targetRepo core.BareRepo, filter func(ref *plumbing.Reference) bool) (posts []Post, err error)

// GetPosts returns references that conform to the post protocol
// filter is used to check whether a reference is a post reference.
// Returns a slice of posts
func GetPosts(targetRepo core.BareRepo, filter func(ref *plumbing.Reference) bool) (posts []Post, err error) {
	itr, err := targetRepo.References()
	if err != nil {
		return nil, err
	}

	err = itr.ForEach(func(ref *plumbing.Reference) error {

		// Ignore references that the filter did not return true for
		if filter != nil && !filter(ref) {
			return nil
		}

		root, err := targetRepo.GetRefRootCommit(ref.Name().String())
		if err != nil {
			return err
		}

		commit, err := targetRepo.CommitObject(plumbing.NewHash(root))
		if err != nil {
			return err
		}

		f, err := commit.File("body")
		if err != nil {
			if err == object.ErrFileNotFound {
				return fmt.Errorf("body file is missing in %s", ref.Name().String())
			}
			return err
		}
		rdr, err := f.Reader()
		if err != nil {
			return err
		}
		cfm, err := pageparser.ParseFrontMatterAndContent(rdr)
		if err != nil {
			return errors.Wrapf(err, "root commit of %s has bad body file", ref.Name().String())
		}

		fm := objx.New(cfm.FrontMatter)
		posts = append(posts, Post{
			Title: fm.Get("title").String(),
			Comment: &Comment{
				Body:        IssueBodyFromContentFrontMatter(&cfm),
				Hash:        commit.Hash.String(),
				Created:     commit.Committer.When,
				Author:      commit.Author.Name,
				AuthorEmail: commit.Author.Email,
				Signature:   commit.PGPSignature,
			},
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return
}

// GetCommentPreview returns a preview of a comment
func GetCommentPreview(comment *Comment) string {
	doc, _ := prose.NewDocument(string(comment.Body.Content))
	var preview = ""
	if sentences := doc.Sentences(); len(sentences) > 0 {
		preview = "\n    " + sentences[0].Text
		if len(sentences) > 1 {
			preview = strings.TrimRight(preview, ".")
			preview += "..."
		}
	}
	return preview
}

type IssueBody struct {

	// Content is the issue content
	Content []byte

	// Title is the issue title
	Title string

	// ReplyTo is used to set the comment commit hash to reply to.
	ReplyTo string

	// Reactions are emoji short names used to describe an emotion
	// towards an issue comment
	Reactions []string

	// Labels describes and classifies the issue using keywords
	Labels []string

	// Assignees are the push keys assigned to do a task
	Assignees []string

	// Fixers are the push keys that should fix the issue
	Fixers []string

	// Close indicates that the issue should be closed.
	// 0: Default (open), 1: Close, 2: Open
	Close int
}

// RequiresUpdatePolicy checks whether the issue body will require an 'issue-update' policy
// if the contents need to be added to the issue.
func (b *IssueBody) RequiresUpdatePolicy() bool {
	return len(b.Labels) > 0 || len(b.Assignees) > 0 || len(b.Fixers) > 0 || b.Close > 0
}

// IssueBodyFromContentFrontMatter attempts to load the instance from
// the specified content front matter object; It will find expected
// fields and try to cast the their expected type. It will not validate
// or return any error.
func IssueBodyFromContentFrontMatter(cfm *pageparser.ContentFrontMatter) *IssueBody {
	ob := objx.New(cfm.FrontMatter)
	b := &IssueBody{}
	b.Content = cfm.Content
	b.Title = ob.Get("title").String()
	b.ReplyTo = ob.Get("replyTo").String()
	b.Close = ob.Get("close").Int()

	b.Reactions = cast.ToStringSlice(ob.Get("reactions").
		StringSlice(cast.ToStringSlice(ob.Get("reactions").InterSlice())))

	b.Labels = cast.ToStringSlice(ob.Get("labels").
		StringSlice(cast.ToStringSlice(ob.Get("labels").InterSlice())))

	b.Assignees = cast.ToStringSlice(ob.Get("assignees").
		StringSlice(cast.ToStringSlice(ob.Get("assignees").InterSlice())))

	b.Fixers = cast.ToStringSlice(ob.Get("fixers").
		StringSlice(cast.ToStringSlice(ob.Get("fixers").InterSlice())))
	return b
}

// IssueBodyToString creates a formatted issue body from an IssueBody object
func IssueBodyToString(body *IssueBody) string {

	args := ""
	str := "---\n%s---\n"

	if len(body.Title) > 0 {
		args += fmt.Sprintf("title: %s\n", body.Title)
	}
	if body.ReplyTo != "" {
		args += fmt.Sprintf("replyTo: %s\n", body.ReplyTo)
	}
	if len(body.Reactions) > 0 {
		reactionsStr, _ := json.Marshal(body.Reactions)
		args += fmt.Sprintf("reactions: %s\n", reactionsStr)
	}
	if len(body.Labels) > 0 {
		labelsStr, _ := json.Marshal(body.Labels)
		args += fmt.Sprintf("labels: %s\n", labelsStr)
	}
	if len(body.Assignees) > 0 {
		assigneesStr, _ := json.Marshal(body.Assignees)
		args += fmt.Sprintf("assignees: %s\n", assigneesStr)
	}
	if len(body.Fixers) > 0 {
		fixersStr, _ := json.Marshal(body.Fixers)
		args += fmt.Sprintf("fixers: %s\n", fixersStr)
	}
	if body.Close >= 0 {
		args += fmt.Sprintf("close: %d\n", body.Close)
	}

	return fmt.Sprintf(str, args) + string(body.Content)
}
