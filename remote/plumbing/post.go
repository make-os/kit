package plumbing

import (
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
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/types/common"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/yaml.v2"
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
	Body         *PostBody
	GetReactions func() map[string]int
}

// Post represents a reference post
type Post struct {
	Repo types.LocalRepo

	// Title is the title of the post
	Title string

	// Name is the full reference name of the post
	Name string

	// First is the first comment in the post (main comment).
	First *Comment
}

func (p *Post) Comment() *Comment {
	return p.First
}

func (p *Post) GetTitle() string {
	return p.Title
}

func (p *Post) GetName() string {
	return p.Name
}

// ReadBody reads the body file of a commit
func ReadBody(repo types.LocalRepo, hash string) (*PostBody, *object.Commit, error) {
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
	cfm, err := util.ParseContentFrontMatter(rdr)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "commit (%s) has bad body file", hash)
	}

	return PostBodyFromContentFrontMatter(&cfm), commit, nil
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
		body, commit, err := ReadBody(p.Repo, hash)
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
		if body.ReplyTo != "" {
			body.ReplyTo, err = p.Repo.ExpandShortHash(body.ReplyTo)
			if err != nil {
				return nil, errors.Wrapf(err, "commit (%s) reply hash could not be expanded", hash)
			}
		}

		comments = append(comments, &Comment{
			Body:        body,
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
		if body.ReplyTo != "" && len(body.Reactions) > 0 {
			UpdateReactions(body.Reactions, body.ReplyTo, pusherKeyID, reactions)
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

// ReactionMap represents mapping for reactions of posts.
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
type Posts []PostEntry

// Reverse reverse the posts
func (p *Posts) Reverse() {
	for i, j := 0, len(*p)-1; i < j; i, j = i+1, j-1 {
		(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
	}
}

// SortByFirstPostCreationTimeDesc sorts the posts by their first post creation time in descending order
func (p *Posts) SortByFirstPostCreationTimeDesc() {
	sort.Slice(*p, func(i, j int) bool {
		return (*p)[i].(*Post).First.Created.UnixNano() > (*p)[j].Comment().Created.UnixNano()
	})
}

// PostGetter describes a function for finding posts
type PostGetter func(targetRepo types.LocalRepo, filter func(ref plumbing.ReferenceName) bool) (posts Posts, err error)

// GetPosts returns references that conform to the post protocol
// filter is used to check whether a reference is a post reference.
// Returns a slice of posts
func GetPosts(targetRepo types.LocalRepo, filter func(ref plumbing.ReferenceName) bool) (posts Posts, err error) {
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
		cfm, err := util.ParseContentFrontMatter(rdr)
		if err != nil {
			return nil, errors.Wrapf(err, "root commit of %s has bad body file", ref.String())
		}

		fm := objx.New(cfm.FrontMatter)
		posts = append(posts, &Post{
			Name:  ref.String(),
			Title: fm.Get("title").String(),
			First: &Comment{
				Body:        PostBodyFromContentFrontMatter(&cfm),
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
	content := string(comment.Body.Content)
	if len(content) > 80 {
		content = content[:80] + "..."
	}
	return "\n    " + content
}

type PostBody struct {

	// Content is the post's main content
	Content []byte `yaml:"-" msgpack:"content,omitempty"`

	// Title is the post's title
	Title string `yaml:"title,omitempty" msgpack:"title,omitempty"`

	// ReplyTo is used to set the comment commit hash to reply to.
	ReplyTo string `yaml:"replyTo,omitempty" msgpack:"replyTo,omitempty"`

	// Reactions are emoji directed at a comment being replied to
	Reactions []string `yaml:"reactions,flow,omitempty" msgpack:"reactions,omitempty"`

	// Close indicates that the post's thread should be closed.
	Close *bool `yaml:"close,omitempty" msgpack:"close,omitempty"`

	// Issue Specific Fields
	common.IssueFields `yaml:",omitempty,inline" msgpack:",omitempty"`

	// Merge Request Fields
	common.MergeRequestFields `yaml:",omitempty,inline" msgpack:",omitempty"`
}

// WantOpen checks whether close=false
func (b *PostBody) WantOpen() bool {
	return b.Close != nil && *b.Close == false
}

// IsAdminUpdate checks whether administrative fields where set
func (b *PostBody) IsAdminUpdate() bool {
	return b.Labels != nil || b.Assignees != nil || b.Close != nil || b.MergeRequestFields != (common.MergeRequestFields{})
}

// PostBodyFromContentFrontMatter attempts to load the instance from
// the specified content front matter object; It will find expected
// fields and try to cast the their expected type. It will not validate
// or return any error.
func PostBodyFromContentFrontMatter(cfm *pageparser.ContentFrontMatter) *PostBody {
	ob := objx.New(cfm.FrontMatter)
	b := &PostBody{}
	b.Content = cfm.Content
	b.Title = ob.Get("title").String()
	b.ReplyTo = ob.Get("replyTo").String()
	b.BaseBranch = ob.Get("base").String()
	b.BaseBranchHash = ob.Get("baseHash").String()
	b.TargetBranch = ob.Get("target").String()
	b.TargetBranchHash = ob.Get("targetHash").String()
	b.Reactions = cast.ToStringSlice(ob.Get("reactions").InterSlice())

	cls := ob.Get("close").Bool()
	b.Close = &cls

	if ob.Has("labels") {
		labels := cast.ToStringSlice(ob.Get("labels").InterSlice())
		b.Labels = &labels
	}

	if ob.Has("assignees") {
		assignees := cast.ToStringSlice(ob.Get("assignees").InterSlice())
		b.Assignees = &assignees
	}

	return b
}

// PostBodyToString creates a formatted post body from an PostBody object
func PostBodyToString(body *PostBody) string {
	out, _ := yaml.Marshal(body)
	if len(out) == 3 && strings.TrimSpace(string(out)) == "{}" {
		return string(body.Content)
	}
	return fmt.Sprintf("---\n%s---\n", out) + string(body.Content)
}

type PostEntry interface {
	GetComments() (comments Comments, err error)
	IsClosed() (bool, error)
	GetTitle() string
	GetName() string
	Comment() *Comment
}

type FreePostIDFinder func(repo types.LocalRepo, startID int, postRefType string) (int, error)

// GetFreePostID finds and returns an ID that is unused within the post reference type
func GetFreePostID(repo types.LocalRepo, startID int, postRefType string) (int, error) {
	for {
		var ref string
		switch postRefType {
		case IssueBranchPrefix:
			ref = MakeIssueReference(startID)
		case MergeRequestBranchPrefix:
			ref = MakeMergeRequestReference(startID)
		default:
			return 0, fmt.Errorf("unknown post reference type")
		}
		hash, err := repo.RefGet(ref)
		if err != nil && err != ErrRefNotFound {
			return 0, err
		}
		if hash == "" {
			return startID, nil
		}
		startID++
	}
}

// PostCommitCreator is a function type for creating a post commit or adding comments to an existing post reference
type PostCommitCreator func(r types.LocalRepo, args *CreatePostCommitArgs) (isNew bool, reference string, err error)

// CreatePostCommitArgs includes argument for CreatePostCommit
type CreatePostCommitArgs struct {
	Type             string
	PostRefID        int
	Body             string
	IsComment        bool
	FreePostIDGetter FreePostIDFinder
}

// CreatePostCommit creates a new post reference or adds a comment commit to an existing one.
func CreatePostCommit(r types.LocalRepo, args *CreatePostCommitArgs) (isNew bool, reference string, err error) {

	var ref string

	// When the post reference ID is not provided, find an unused number, increment it
	// and use it to generate a post reference name
	if args.PostRefID == 0 {
		args.PostRefID, err = args.FreePostIDGetter(r, 1, args.Type)
		if err != nil {
			return false, "", errors.Wrap(err, "failed to find free post number")
		}
	}

	// Generate the full reference name
	switch args.Type {
	case IssueBranchPrefix:
		ref = MakeIssueReference(args.PostRefID)
	case MergeRequestBranchPrefix:
		ref = MakeMergeRequestReference(args.PostRefID)
	default:
		return false, "", fmt.Errorf("unknown post reference type")
	}

	// Check if the post reference already exist exist
	hash, err := r.RefGet(ref)
	if err != nil {
		if err != ErrRefNotFound {
			return false, "", errors.Wrap(err, "failed to check post reference existence")
		}
	}

	// For comments, the post reference must already exist
	if hash == "" && args.IsComment {
		return false, "", fmt.Errorf("can't add comment to a non-existing post")
	}

	// Create an post commit (pass the current reference hash as parent)
	commitHash, err := r.CreateSingleFileCommit("body", args.Body, "", hash)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to create post commit")
	}

	// Update the current hash of the post reference
	if err = r.RefUpdate(ref, commitHash); err != nil {
		return false, "", errors.Wrap(err, "failed to update post reference target hash")
	}

	return hash == "", ref, nil
}
