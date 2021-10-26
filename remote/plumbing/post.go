package plumbing

import (
	"encoding/pem"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"
)

// Comments is a collection of Comment objects
type Comments []*Comment

// Reverse reverses the posts
func (c *Comments) Reverse() {
	for i, j := 0, len(*c)-1; i < j; i, j = i+1, j-1 {
		(*c)[i], (*c)[j] = (*c)[j], (*c)[i]
	}
}

// Comment represent a reference post comment
type Comment struct {
	CreatedAt    time.Time             `json:"createdAt"`
	Reference    string                `json:"reference"`
	Hash         string                `json:"hash"`
	Author       string                `json:"author"`
	AuthorEmail  string                `json:"authorEmail"`
	Signature    string                `json:"signature"`
	Pusher       string                `json:"pusher"`
	Body         *PostBody             `json:"body,omitempty"`
	GetReactions func() map[string]int `json:"-"`
}

// Post represents a reference post
type Post struct {
	Repo types.LocalRepo `json:"-"`

	// Title is the title of the post
	Title string `json:"title,omitempty"`

	// Name is the full reference name of the post
	Name string `json:"name,omitempty"`

	// Comment is the first comment of the post.
	Comment *Comment `json:"comment,omitempty"`
}

func (p *Post) GetComment() *Comment {
	return p.Comment
}

func (p *Post) GetTitle() string {
	return p.Title
}

func (p *Post) GetName() string {
	return p.Name
}

// PostBodyReader represents a function for reading a commit's post body
type PostBodyReader func(repo types.LocalRepo, hash string) (*PostBody, *object.Commit, error)

// ReadPostBody reads the body file of a commit
func ReadPostBody(repo types.LocalRepo, hash string) (*PostBody, *object.Commit, error) {
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

// GetComments returns the comments in the post
func (p *Post) GetComments() (comments Comments, err error) {
	hashes, err := p.Repo.GetRefCommits(p.Name, true)
	if err != nil {
		return nil, err
	}

	var reactions = ReactionMap{}
	var pusherKeyID string

	// process each comment commit
	for _, hash := range hashes {
		body, commit, err := ReadPostBody(p.Repo, hash)
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
			CreatedAt:   commit.Committer.When,
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

	body, _, err := ReadPostBody(p.Repo, ref.Hash().String())
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

// Reverse reverses the posts
func (p *Posts) Reverse() {
	for i, j := 0, len(*p)-1; i < j; i, j = i+1, j-1 {
		(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
	}
}

// SortByFirstPostCreationTimeDesc sorts the posts by their first post creation time in descending order
func (p *Posts) SortByFirstPostCreationTimeDesc() {
	sort.Slice(*p, func(i, j int) bool {
		return (*p)[i].(*Post).Comment.CreatedAt.UnixNano() > (*p)[j].GetComment().CreatedAt.UnixNano()
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
			Comment: &Comment{
				Body:        PostBodyFromContentFrontMatter(&cfm),
				Hash:        commit.Hash.String(),
				CreatedAt:   commit.Committer.When,
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
	Content []byte `yaml:"-" msgpack:"content,omitempty" json:"content"`

	// Title is the post's title
	Title string `yaml:"title,omitempty" msgpack:"title,omitempty" json:"title"`

	// ReplyTo is used to set the comment commit hash to reply to.
	ReplyTo string `yaml:"replyTo,omitempty" msgpack:"replyTo,omitempty" json:"replyTo,omitempty"`

	// Reactions are emoji directed at a comment being replied to
	Reactions []string `yaml:"reactions,flow,omitempty" msgpack:"reactions,omitempty" json:"reactions,omitempty"`

	// Close indicates that the post's thread should be closed.
	Close *bool `yaml:"close,omitempty" msgpack:"close,omitempty" json:"close,omitempty"`

	// Issue Specific Fields
	*types.IssueFields `yaml:",omitempty,inline" msgpack:",omitempty"`

	// Merge Request Fields
	*types.MergeRequestFields `yaml:",omitempty,inline" msgpack:",omitempty"`
}

// NewEmptyPostBody returns a PostBody instance that is empty
func NewEmptyPostBody() *PostBody {
	return &PostBody{
		IssueFields:        &types.IssueFields{},
		MergeRequestFields: &types.MergeRequestFields{},
	}
}

// WantOpen checks whether close=false
func (b *PostBody) WantOpen() bool {
	return !pointer.GetBool(b.Close)
}

// IncludesAdminFields checks whether administrative fields where set
func (b *PostBody) IncludesAdminFields() bool {
	if b.Close != nil {
		return true
	}
	if b.IssueFields != nil && (len(b.Labels) > 0 || len(b.Assignees) > 0) {
		return true
	}
	if b.MergeRequestFields != nil && (len(b.BaseBranch) > 0 || len(b.BaseBranchHash) > 0 ||
		len(b.TargetBranch) > 0 || len(b.TargetBranchHash) > 0) {
		return true
	}
	return false
}

// PostBodyFromContentFrontMatter attempts to load the instance from
// the specified content front matter object; It will find expected
// fields and try to cast their expected type. It will not validate
// or return any error.
func PostBodyFromContentFrontMatter(cfm *pageparser.ContentFrontMatter) *PostBody {
	ob := objx.New(cfm.FrontMatter)
	b := NewEmptyPostBody()
	b.Content = cfm.Content
	b.Title = ob.Get("title").String()
	b.ReplyTo = ob.Get("replyTo").String()
	b.BaseBranch = ob.Get("base").String()
	b.BaseBranchHash = ob.Get("baseHash").String()
	b.TargetBranch = ob.Get("target").String()
	b.TargetBranchHash = ob.Get("targetHash").String()
	b.Reactions = cast.ToStringSlice(ob.Get("reactions").InterSlice())

	if ob.Has("close") {
		cls := ob.Get("close").Bool()
		b.Close = &cls
	}

	if ob.Has("labels") {
		labels := cast.ToStringSlice(ob.Get("labels").InterSlice())
		b.Labels = labels
	}

	if ob.Has("assignees") {
		assignees := cast.ToStringSlice(ob.Get("assignees").InterSlice())
		b.Assignees = assignees
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
	GetComment() *Comment
}

// GetFreePostIDFunc describes GetFreePostID function signature
type GetFreePostIDFunc func(repo types.LocalRepo, startID int, postRefType string) (int, error)

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

	// Type is the type of post reference
	Type string

	// ID is the unique ID of the target post reference.
	// If unset, a free ID will be used.
	// If ID is a string, it is expected that the call
	// passed a full post reference path.
	ID interface{}

	// Body is the post's body file content
	Body string

	// IsComment indicates that the post is intended to be a comment
	IsComment bool

	// Force indicates that uncommitted changes can be ignored and lost
	Force bool

	// GetFreePostID is used to find a free post ID
	GetFreePostID GetFreePostIDFunc
}

// CreatePostCommit creates a new post reference or adds a comment commit to an existing one.
func CreatePostCommit(r types.LocalRepo, args *CreatePostCommitArgs) (isNew bool, reference string, err error) {

	// Ensure we are working in a clean repository.
	// If args.Force is true, uncommitted changes are ignored.
	if !args.Force {
		isClean, err := r.IsClean()
		if err != nil {
			return false, "", errors.Wrap(err, "failed to check repo status")
		} else if !isClean {
			return false, "", fmt.Errorf("dirty working tree; there are uncommitted changes")
		}
	}

	var ref string
	switch v := args.ID.(type) {
	case string:
		ref = v
	case int:
		if v == 0 {
			v, err = args.GetFreePostID(r, 1, args.Type)
			if err != nil {
				return false, "", errors.Wrap(err, "failed to find free post number")
			}
		}
		switch args.Type {
		case IssueBranchPrefix:
			ref = MakeIssueReference(v)
		case MergeRequestBranchPrefix:
			ref = MakeMergeRequestReference(v)
		default:
			return false, "", fmt.Errorf("unknown post reference type")
		}
	}

	// Check if the post reference already exist
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

	// Create a post commit (pass the current reference hash as parent)
	commitHash, err := r.CreateSingleFileCommit("body", args.Body, "", hash)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to create post commit")
	}

	// Update the current hash of the post reference
	if err = r.RefUpdate(ref, commitHash); err != nil {
		return false, "", errors.Wrap(err, "failed to update post reference target hash")
	}

	// If HEAD is the same as the post reference, check it out to force
	// the working tree to be updated
	head, err := r.Head()
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get HEAD")
	} else if head == ref {
		if err = r.Checkout(plumbing.ReferenceName(ref).Short(), false, args.Force); err != nil {
			return false, "", errors.Wrap(err, "failed to checkout post reference")
		}
	}

	return hash == "", ref, nil
}
