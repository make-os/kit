package plumbing

import (
	"fmt"
	"strings"
	"time"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
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
	Content     string
	Author      string
	AuthorEmail string
	Signature   string
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
				Hash:        commit.Hash.String(),
				Created:     commit.Committer.When,
				Author:      commit.Author.Name,
				AuthorEmail: commit.Author.Email,
				Content:     string(cfm.Content),
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
	doc, _ := prose.NewDocument(comment.Content)
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
