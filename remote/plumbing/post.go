package plumbing

import (
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
	Content   []byte
	Title     string
	ReplyTo   string
	Reactions []string
	Labels    []string
	Assignees []string
	Fixers    []string
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
