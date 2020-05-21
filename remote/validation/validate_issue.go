package validation

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	rr "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
)

var (
	MaxIssueContentLen        = 1024 * 8 // 8KB
	MaxIssueTitleLen          = 256
	ErrCannotWriteToClosedRef = fmt.Errorf("cannot write to a closed reference")
)

// ValidatePostCommitArg contains arguments for ValidatePostCommit
type ValidatePostCommitArg struct {
	OldHash         string
	Change          *core.ItemChange
	TxDetail        *types.TxDetail
	PushKeyGetter   core.PushKeyGetter
	CheckPostCommit PostCommitChecker
	CheckCommit     CommitChecker
}

// ValidatePostCommit validate a pushed post commit.
// commit is the recent post commit in the post reference.
func ValidatePostCommit(repo types.LocalRepo, commit types.Commit, args *ValidatePostCommitArg) error {

	// Post reference history cannot have merge commits (merge commit not permitted)
	hasMerges, err := repo.HasMergeCommits(args.TxDetail.Reference)
	if err != nil {
		return errors.Wrap(err, "failed to check for merge commits in post reference")
	} else if hasMerges {
		return fmt.Errorf("post history must not include merge commits")
	}

	// Check the latest commit using standard commit validation rules
	unwrapped := commit.UnWrap()
	if err = args.CheckCommit(unwrapped, args.TxDetail, args.PushKeyGetter); err != nil {
		return err
	}

	// Collect the ancestors of the latest commit that were pushed along and
	// not part of the post history prior to the current push
	ancestors, err := repo.GetAncestors(unwrapped, args.OldHash, true)
	if err != nil {
		return err
	}

	// Validate the new post commits by replaying the commits individually
	// beginning from the first ancestor.
	postCommits := append(ancestors, unwrapped)
	refState := repo.GetState().References.Get(args.TxDetail.Reference)
	for i, postCommit := range postCommits {

		// Define the post commit checker arguments
		icArgs := &CheckPostCommitArgs{
			Reference: args.TxDetail.Reference,
			OldHash:   args.OldHash,
			IsNew:     refState.IsNil(),
		}

		// If there are ancestors, set IsNew to false at index > 0.
		// It ensures CheckPostCommit does not treat the post reference as
		// a new one. Also, set OldHash to the previous ancestor to mimic
		// an already persisted post reference history.
		if i > 0 {
			icArgs.IsNew = false
			icArgs.OldHash = postCommits[i-1].Hash.String()
		}

		post, err := args.CheckPostCommit(repo, rr.WrapCommit(postCommit), icArgs)
		if err != nil {
			return err
		}

		// Set flag for the authorizer function to check for admin update policy
		if post.IsAdminUpdate() {
			args.TxDetail.FlagCheckAdminUpdatePolicy = true
		}

		// Set post data to reference object. Since we only acknowledge the recent (signed) commit
		// as the one that can change the reference state, we do this for only the latest commit.
		isRecentCommit := postCommit.Hash.String() == args.Change.Item.GetData()
		if isRecentCommit {
			data := args.TxDetail.Data()
			data.Close = post.Close
			data.Assignees = post.Assignees
			data.Labels = post.Labels
			data.BaseBranch = post.BaseBranch
			data.BaseBranchHash = post.BaseBranchHash
			data.TargetBranch = post.TargetBranch
			data.TargetBranchHash = post.TargetBranchHash
		}

		// When a reference exist and it is closed, the next pushed commit is expected
		// to reopen it by setting the 'close' field to 'open'.
		if isRecentCommit && refState.Data.Closed && !post.WantOpen() {
			return ErrCannotWriteToClosedRef
		}
	}

	return nil
}

// CheckPostCommitArgs includes arguments for CheckPostCommit function
type CheckPostCommitArgs struct {
	Reference string
	OldHash   string
	IsNew     bool
}

// PostCommitChecker describes a function for validating a post commit.
type PostCommitChecker func(
	repo types.LocalRepo,
	commit types.Commit,
	args *CheckPostCommitArgs) (*plumbing2.PostBody, error)

// CheckPostCommit validates new commits of a post reference. It returns nil post body
// and error if validation failed or a post body and nil if validation passed.
func CheckPostCommit(repo types.LocalRepo, commit types.Commit, args *CheckPostCommitArgs) (*plumbing2.PostBody, error) {

	// Reference name must be valid
	if !plumbing2.IsIssueReference(args.Reference) && !plumbing2.IsMergeRequestReference(args.Reference) {
		return nil, fmt.Errorf("post number is not valid. Must be numeric")
	}

	// Post commits can't have multiple parents (merge commit not permitted)
	if commit.NumParents() > 1 {
		return nil, fmt.Errorf("post commit cannot have more than one parent")
	}

	// Post's first commit must have zero hash parent (orphan commit)
	if args.IsNew && commit.NumParents() != 0 {
		return nil, fmt.Errorf("first commit of a new post must have no parent")
	}

	// Post commit history must not alter the current history (rebasing not permitted)
	if !args.IsNew && repo.IsAncestor(args.OldHash, commit.GetHash().String()) != nil {
		return nil, fmt.Errorf("post commit must not alter history")
	}

	tree, err := commit.GetTree()
	if err != nil {
		return nil, fmt.Errorf("unable to read post commit tree")
	}

	// Post commit tree cannot be left empty
	if len(tree.Entries) == 0 {
		return nil, fmt.Errorf("post commit must have a 'body' file")
	}

	// Post commit must include one file
	if len(tree.Entries) > 1 {
		return nil, fmt.Errorf("post commit tree must only include a 'body' file")
	}

	// Post commit must include only a body file
	body := tree.Entries[0]
	if body.Mode != filemode.Regular {
		return nil, fmt.Errorf("post body file is not a regular file")
	}

	file, _ := tree.File(body.Name)
	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("post body file could not be read")
	}

	// The body file must be parsable (extract front matter and content)
	cfm, err := util.ParseContentFrontMatter(bytes.NewBufferString(content))
	if err != nil {
		return nil, fmt.Errorf("post body could not be parsed")
	}

	// Validate extracted front matter
	if err = CheckPostBody(repo, commit, args.IsNew, cfm.FrontMatter, cfm.Content); err != nil {
		return nil, err
	}

	return plumbing2.PostBodyFromContentFrontMatter(&cfm), nil
}

// CheckPostBody checks whether the front matter and content extracted from a post body is ok.
// repo: The target repo
// commit: The commit whose content needs to be checked.
// isNew: indicates that the post reference is new.
// fm: The front matter data.
// content: The content from the post commit.
//
// Valid front matter fields:
//  title: The title of the post (optional).
//  labels: Labels categorize posts into arbitrary or conceptual units.
//  replyTo: Indicates a post is a response an ancestor (a comment).
//  assignees: List push keys assigned to the posts.
func CheckPostBody(
	repo types.LocalRepo,
	commit types.Commit,
	isNew bool,
	fm map[string]interface{},
	content []byte) error {

	commitHash := commit.GetHash().String()
	var makeField = func(name string) string {
		return fmt.Sprintf("<commit#%s>.%s", commitHash[:7], name)
	}

	// Ensure only valid fields are included
	var validFields = []string{
		"title",
		"reactions",
		"labels",
		"replyTo",
		"assignees",
		"close"}
	for k := range fm {
		if !funk.ContainsString(validFields, k) {
			return fe(-1, makeField(k), "unknown field")
		}
	}

	obj := objx.New(fm)

	title := obj.Get("title")
	if !title.IsNil() && !title.IsStr() {
		return fe(-1, makeField("title"), "expected a string value")
	}

	replyTo := obj.Get("replyTo")
	if !replyTo.IsNil() && !replyTo.IsStr() {
		return fe(-1, makeField("replyTo"), "expected a string value")
	}

	reactions := obj.Get("reactions")
	if !reactions.IsNil() && !reactions.IsInterSlice() {
		return fe(-1, makeField("reactions"), "expected a list of string values")
	}

	labels := obj.Get("labels")
	if !labels.IsNil() && !labels.IsInterSlice() {
		return fe(-1, makeField("labels"), "expected a list of string values")
	}

	assignees := obj.Get("assignees")
	if !assignees.IsNil() && !assignees.IsInterSlice() {
		return fe(-1, makeField("assignees"), "expected a list of string values")
	}

	// Ensure post commit does not have a replyTo value if the post reference is new
	if isNew && len(replyTo.String()) > 0 {
		return fe(-1, makeField("replyTo"), "not expected in a new post commit")
	}

	// Ensure title is unset when replyTo is set
	if len(replyTo.String()) > 0 && len(title.String()) > 0 {
		return fe(-1, makeField("title"), "title is not required when replying")
	}

	// Ensure title is provided if the post reference is new
	if isNew && len(title.String()) == 0 {
		return fe(-1, makeField("title"), "title is required")
	} else if !isNew && len(title.String()) > 0 {
		return fe(-1, makeField("title"), "title is not required for comment commit")
	}

	// Title cannot exceed max.
	if len(title.String()) > MaxIssueTitleLen {
		return fe(-1, makeField("title"), "title is too long and cannot exceed 256 characters")
	}

	// ReplyTo must have len >= 4 or < 40
	replyToVal := replyTo.String()
	if len(replyToVal) > 0 && (len(replyToVal) < 4 || len(replyToVal) > 40) {
		return fe(-1, makeField("replyTo"), "invalid hash value")
	}

	// When ReplyTo is set, ensure the post commit is a descendant of the replyTo
	if len(replyToVal) > 0 {
		if repo.IsAncestor(replyToVal, commit.GetHash().String()) != nil {
			return fe(-1, makeField("replyTo"), "hash is not a known ancestor")
		}
	}

	// Check reactions if set.
	if val := reactions.InterSlice(); len(val) > 0 {
		if len(val) > 10 {
			return fe(-1, makeField("reactions"), "too many reactions. Cannot exceed 10")
		}
		if !util.IsString(val[0]) {
			return fe(-1, makeField("reactions"), "expected a string list")
		}
		for i, name := range reactions.InterSlice() {
			if !util.IsEmojiValid(strings.TrimPrefix(name.(string), "-")) {
				return fe(i, makeField("reactions"), "unknown reaction")
			}
		}
	}

	// Check labels if set.
	if size := len(labels.InterSlice()); size > 0 {
		if size > 10 {
			return fe(-1, makeField("labels"), "too many labels. Cannot exceed 10")
		}
		if !util.IsString(labels.InterSlice()[0]) {
			return fe(-1, makeField("labels"), "expected a string list")
		}
		for i, val := range labels.InterSlice() {
			if err := util.IsValidNameNoLen(strings.TrimPrefix(val.(string), "-")); err != nil {
				return fe(i, makeField("labels"), err.Error())
			}
		}
	}

	// Check assignees if set.
	if val := assignees.InterSlice(); len(val) > 0 {
		if len(val) > 10 {
			return fe(-1, makeField("assignees"), "too many assignees. Cannot exceed 10")
		}
		if !util.IsString(val[0]) {
			return fe(-1, makeField("assignees"), "expected a string list")
		}
		for i, assignee := range val {
			if !util.IsValidPushAddr(strings.TrimPrefix(assignee.(string), "-")) {
				return fe(i, makeField("assignees"), "invalid push key ID")
			}
		}
	}

	// Post commit content is required for a new post reference
	if isNew && len(content) == 0 {
		return fe(-1, makeField("content"), "post commit content is required")
	}

	// Post commit content cannot be greater than the maximum allowed
	if len(content) > MaxIssueContentLen {
		return fe(-1, makeField("content"), "post commit content length exceeded max character limit")
	}

	return nil
}
