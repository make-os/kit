package plumbing

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/jdkato/prose.v2"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Comments is a collection of Comment objects
type Comments []*Comment

// Reverse reverse the posts
func (c *Comments) Reverse() {
	for i, j := 0, len(*c)-1; i < j; i, j = i+1, j-1 {
		(*c)[i], (*c)[j] = (*c)[j], (*c)[i]
	}
}

// Comment represent a reference post comment
type Comment struct {
	Created      time.Time
	Reference    string
	Hash         string
	Author       string
	AuthorEmail  string
	Signature    string
	Pusher       string
	Body         *IssueBody
	GetReactions func() map[string]int
}

// Post represents a reference post
type Post struct {
	Repo core.LocalRepo

	// Title is the title of the post
	Title string

	// Name is the full reference name of the post
	Name string

	// First is the first comment in the post (main comment).
	First *Comment
}

func (p *Post) FirstComment() *Comment {
	return p.First
}

func (p *Post) GetTitle() string {
	return p.Title
}

func (p *Post) GetName() string {
	return p.Name
}

// ReadBody reads the body file of a commit
func ReadBody(repo core.LocalRepo, hash string) (*IssueBody, *object.Commit, error) {
	commit, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read commit (%s)", hash)
	}

	// Read the commit body file
	f, err := commit.File("body")
	if err != nil {
		if err == object.ErrFileNotFound {
			return nil, nil, fmt.Errorf("body file of commit (%s) is missing", hash)
		}
		return nil, nil, err
	}

	// Parse the body file
	rdr, err := f.Reader()
	if err != nil {
		return nil, nil, err
	}
	cfm, err := pageparser.ParseFrontMatterAndContent(rdr)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "commit (%s) has bad body file", hash)
	}

	return IssueBodyFromContentFrontMatter(&cfm), commit, nil
}

// GetComment returns the comments in the post
func (p *Post) GetComments() (comments Comments, err error) {
	hashes, err := p.Repo.GetRefCommits(p.Name, true)
	if err != nil {
		return nil, err
	}

	var reactions = ReactionMap{}
	var pusherKeyID string

	// process each comment commit
	for _, hash := range hashes {
		issueBody, commit, err := ReadBody(p.Repo, hash)
		if err != nil {
			return nil, err
		}

		// Get the pusher key from the signature header
		if commit.PGPSignature != "" {
			p, _ := pem.Decode([]byte(commit.PGPSignature))
			if p == nil {
				return nil, fmt.Errorf("unable to decode commit (%s) signature", hash)
			}
			pusherKeyID = p.Headers["pkID"]
		}

		// Expand reply hash
		if issueBody.ReplyTo != "" {
			issueBody.ReplyTo, err = p.Repo.ExpandShortHash(issueBody.ReplyTo)
			if err != nil {
				return nil, errors.Wrapf(err, "commit (%s) reply hash could not be expanded", hash)
			}
		}

		comments = append(comments, &Comment{
			Body:        issueBody,
			Hash:        commit.Hash.String(),
			Reference:   p.Name,
			Created:     commit.Committer.When,
			Author:      commit.Author.Name,
			AuthorEmail: commit.Author.Email,
			Signature:   commit.PGPSignature,
			Pusher:      pusherKeyID,
			GetReactions: func() map[string]int {
				return GetReactionsForComment(reactions, commit.Hash.String())
			},
		})

		// Compute and updates reactions if this comment replied with reaction(s)
		if issueBody.ReplyTo != "" && len(issueBody.Reactions) > 0 {
			UpdateReactions(issueBody.Reactions, issueBody.ReplyTo, pusherKeyID, reactions)
		}
	}

	return
}

// IsClosed tells whether the post is closed by checking if the last
// comment includes a "closed" directive.
func (p *Post) IsClosed() (bool, error) {
	ref, err := p.Repo.Reference(plumbing.ReferenceName(p.Name), false)
	if err != nil {
		return false, err
	}

	body, _, err := ReadBody(p.Repo, ref.Hash().String())
	if err != nil {
		return false, err
	}

	if body.Close != nil {
		return *body.Close, nil
	}

	return false, nil
}

// GetReactionsForComment returns summed reactions of a comment.
func GetReactionsForComment(reactions ReactionMap, hash string) map[string]int {
	res := map[string]int{}
	if commentReactions, ok := reactions[hash]; ok {
		for r, pushersReactions := range commentReactions {
			if _, ok := res[r]; !ok {
				res[r] = 0
			}
			res[r] = int(math.Max(funk.Sum(funk.Values(pushersReactions)), 0))
		}
	}
	return res
}

// ReactionMap represents mapping for reactions in an issue
// commentHash: (reactionName: (pusherKeyID: count))
type ReactionMap map[string]map[string]map[string]int

// UpdateReactions calculates reactions for a target comment.
// newReactions: are new reactions from a comment.
// targetHash: the unique hash of the comment being reacted to.
// pusherKeyID: the unique push key ID of the reactor.
// dest: the map that contains the reaction data of which this function must update.
func UpdateReactions(newReactions []string, targetHash, pusherKeyID string, dest ReactionMap) {
	for _, reaction := range newReactions {
		nonNegated := strings.TrimLeft(reaction, "-")

		// Add target hash reactions map if not set
		if dest[targetHash] == nil {
			dest[targetHash] = map[string]map[string]int{}
		}

		// Add reaction map in the target map if not set
		if dest[targetHash][nonNegated] == nil {
			dest[targetHash][nonNegated] = map[string]int{}
		}

		// Add the pusher
		if _, ok := dest[targetHash][nonNegated][pusherKeyID]; !ok {
			dest[targetHash][nonNegated][pusherKeyID] = 0
		}

		// For negated reaction, decrement reaction count for the pusher.
		if reaction[0] == '-' {
			if _, ok := dest[targetHash][nonNegated]; ok {
				dest[targetHash][nonNegated][pusherKeyID] -= 1
			}
			continue
		}

		// For non-negated reaction, increment reaction count for the pusher.
		dest[targetHash][reaction][pusherKeyID] += 1
	}
}

// Posts is a collection of Post
type Posts []IPost

// Reverse reverse the posts
func (p *Posts) Reverse() {
	for i, j := 0, len(*p)-1; i < j; i, j = i+1, j-1 {
		(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
	}
}

// SortByFirstPostCreationTimeDesc sorts the posts by their first post creation time in descending order
func (p *Posts) SortByFirstPostCreationTimeDesc() {
	sort.Slice(*p, func(i, j int) bool {
		return (*p)[i].(*Post).First.Created.UnixNano() > (*p)[j].FirstComment().Created.UnixNano()
	})
}

// PostGetter describes a function for finding posts
type PostGetter func(targetRepo core.LocalRepo, filter func(ref plumbing.ReferenceName) bool) (posts Posts, err error)

// GetPosts returns references that conform to the post protocol
// filter is used to check whether a reference is a post reference.
// Returns a slice of posts
func GetPosts(targetRepo core.LocalRepo, filter func(ref plumbing.ReferenceName) bool) (posts Posts, err error) {
	refs, err := targetRepo.GetReferences()
	if err != nil {
		return nil, err
	}

	var pusherKeyID string
	for _, ref := range refs {
		if ref.String() == "HEAD" {
			continue
		}

		// Ignore references that the filter did not return true for
		if filter != nil && !filter(ref) {
			continue
		}

		root, err := targetRepo.GetRefRootCommit(ref.String())
		if err != nil {
			return nil, err
		}

		commit, err := targetRepo.CommitObject(plumbing.NewHash(root))
		if err != nil {
			return nil, errors.Wrap(err, "failed to get first comment")
		}

		// Get the pusher key from the signature header
		if commit.PGPSignature != "" {
			p, _ := pem.Decode([]byte(commit.PGPSignature))
			if p == nil {
				return nil, fmt.Errorf("unable to decode first comment commit signature")
			}
			pusherKeyID = p.Headers["pkID"]
		}

		f, err := commit.File("body")
		if err != nil {
			if err == object.ErrFileNotFound {
				return nil, fmt.Errorf("body file is missing in %s", ref.String())
			}
			return nil, err
		}
		rdr, err := f.Reader()
		if err != nil {
			return nil, err
		}
		cfm, err := pageparser.ParseFrontMatterAndContent(rdr)
		if err != nil {
			return nil, errors.Wrapf(err, "root commit of %s has bad body file", ref.String())
		}

		fm := objx.New(cfm.FrontMatter)
		posts = append(posts, &Post{
			Name:  ref.String(),
			Title: fm.Get("title").String(),
			First: &Comment{
				Body:        IssueBodyFromContentFrontMatter(&cfm),
				Hash:        commit.Hash.String(),
				Created:     commit.Committer.When,
				Author:      commit.Author.Name,
				AuthorEmail: commit.Author.Email,
				Signature:   commit.PGPSignature,
				Pusher:      pusherKeyID,
			},
			Repo: targetRepo,
		})
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
	Labels *[]string

	// Assignees are the push keys assigned to do a task
	Assignees *[]string

	// Close indicates that the issue should be closed.
	Close *bool
}

// WantOpen checks whether close=false
func (b *IssueBody) WantOpen() bool {
	return b.Close != nil && *b.Close == false
}

// RequiresUpdatePolicy checks whether the issue body will require an 'issue-update' policy
// if the contents need to be added to the issue.
func (b *IssueBody) RequiresUpdatePolicy() bool {
	return b.Labels != nil || b.Assignees != nil || b.Close != nil
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

	close := ob.Get("close").Bool()
	b.Close = &close

	b.Reactions = cast.ToStringSlice(ob.Get("reactions").
		StringSlice(cast.ToStringSlice(ob.Get("reactions").InterSlice())))

	if ob.Has("labels") {
		labels := cast.ToStringSlice(ob.Get("labels").
			StringSlice(cast.ToStringSlice(ob.Get("labels").InterSlice())))
		b.Labels = &labels
	}

	if ob.Has("assignees") {
		assignees := cast.ToStringSlice(ob.Get("assignees").
			StringSlice(cast.ToStringSlice(ob.Get("assignees").InterSlice())))
		b.Assignees = &assignees
	}

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
	if body.Labels != nil && *body.Labels != nil {
		labelsStr, _ := json.Marshal(body.Labels)
		args += fmt.Sprintf("labels: %s\n", labelsStr)
	}
	if body.Assignees != nil && *body.Assignees != nil {
		assigneesStr, _ := json.Marshal(body.Assignees)
		args += fmt.Sprintf("assignees: %s\n", assigneesStr)
	}
	if body.Close != nil {
		args += fmt.Sprintf("close: %v\n", *body.Close)
	}

	return fmt.Sprintf(str, args) + string(body.Content)
}

type IPost interface {
	GetComments() (comments Comments, err error)
	IsClosed() (bool, error)
	GetTitle() string
	GetName() string
	FirstComment() *Comment
}
