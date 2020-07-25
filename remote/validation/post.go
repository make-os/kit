package validation

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/lobe/logic/contracts/mergerequest"
	plumbing2 "gitlab.com/makeos/lobe/remote/plumbing"
	rr "gitlab.com/makeos/lobe/remote/repo"
	"gitlab.com/makeos/lobe/remote/types"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/util"
	"gitlab.com/makeos/lobe/util/crypto"
	"gitlab.com/makeos/lobe/util/identifier"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
)

var (
	MaxIssueContentLen        = 1024 * 8 // 8KB
	MaxIssueTitleLen          = 256
	ErrCannotWriteToClosedRef = fmt.Errorf("cannot write to a closed reference")
	mergeReqFields            = []string{"base", "baseHash", "target", "targetHash"}
)

// ValidatePostCommitArg contains arguments for ValidatePostCommit
type ValidatePostCommitArg struct {
	OldHash         string
	Change          *types.ItemChange
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

		isTip := postCommit.Hash.String() == args.Change.Item.GetData()
		if isTip {

			// Update reference data in tx detail with post data from tip commit.
			copier.Copy(args.TxDetail.Data(), post)

			// When a reference exist and it is closed, the next pushed commit is expected
			// to reopen it by setting the 'close' field to 'open'.
			if refState.Data.Closed && !post.WantOpen() {
				return ErrCannotWriteToClosedRef
			}
		}

		// When the current commit is not the tip but it includes admin updates, we need
		// to reject it as only the tip can carry updates that require admin privileges
		if !isTip && post.IsAdminUpdate() {
			return fmt.Errorf("non-tip commit (%s) cannot update any field "+
				"that requires admin permission", postCommit.Hash.String()[:7])
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

	tree, err := commit.Tree()
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
	if err = CheckPostBody(repo, args.Reference, commit, args.IsNew, cfm.FrontMatter, cfm.Content); err != nil {
		return nil, err
	}

	return plumbing2.PostBodyFromContentFrontMatter(&cfm), nil
}

// CheckCommonPostBody performs sanity checks on common fields of a post body
func CheckCommonPostBody(
	repo types.LocalRepo,
	commit types.Commit,
	isNewRef bool,
	fm map[string]interface{},
	content []byte) error {

	var commitHash = commit.GetHash().String()

	obj := objx.New(fm)
	title := obj.Get("title")
	if !title.IsNil() && !title.IsStr() {
		return fe(-1, makeField("title", commitHash), "expected a string value")
	}

	replyTo := obj.Get("replyTo")
	if !replyTo.IsNil() && !replyTo.IsStr() {
		return fe(-1, makeField("replyTo", commitHash), "expected a string value")
	}

	reactions := obj.Get("reactions")
	if !reactions.IsNil() && !reactions.IsInterSlice() {
		return fe(-1, makeField("reactions", commitHash), "expected a list of string values")
	}

	// Ensure post commit does not have a replyTo value if the post reference is new
	if isNewRef && len(replyTo.String()) > 0 {
		return fe(-1, makeField("replyTo", commitHash), "not expected in a new post commit")
	}

	// Ensure title is unset when replyTo is set
	if len(replyTo.String()) > 0 && len(title.String()) > 0 {
		return fe(-1, makeField("title", commitHash), "title is not required when replying")
	}

	// Ensure title is provided if the post reference is new
	if isNewRef && len(title.String()) == 0 {
		return fe(-1, makeField("title", commitHash), "title is required")
	} else if !isNewRef && len(title.String()) > 0 {
		return fe(-1, makeField("title", commitHash), "title is not required when replying")
	}

	// Title cannot exceed max.
	if len(title.String()) > MaxIssueTitleLen {
		msg := fmt.Sprintf("title is too long; cannot exceed %d characters", MaxIssueTitleLen)
		return fe(-1, makeField("title", commitHash), msg)
	}

	// ReplyTo must have len >= 4 or < 40
	replyToVal := replyTo.String()
	if len(replyToVal) > 0 && (len(replyToVal) < 4 || len(replyToVal) > 40) {
		return fe(-1, makeField("replyTo", commitHash), "invalid hash value")
	}

	// When ReplyTo is set, ensure the post commit is a descendant of the replyTo
	if len(replyToVal) > 0 {
		if repo.IsAncestor(replyToVal, commit.GetHash().String()) != nil {
			return fe(-1, makeField("replyTo", commitHash), "hash is not a known ancestor")
		}
	}

	// Check reactions if set.
	if val := reactions.InterSlice(); len(val) > 0 {
		if len(val) > 10 {
			return fe(-1, makeField("reactions", commitHash), "too many reactions; cannot exceed 10")
		}
		if !util.IsString(val[0]) {
			return fe(-1, makeField("reactions", commitHash), "expected a string list")
		}
		for i, name := range reactions.InterSlice() {
			if !util.IsEmojiValid(strings.TrimPrefix(name.(string), "-")) {
				return fe(i, makeField("reactions", commitHash), "reaction '"+name.(string)+"' is unknown")
			}
		}
	}

	// Post commit content is required for a new post reference
	if isNewRef && len(content) == 0 {
		return fe(-1, makeField("content", commitHash), "post content is required")
	}

	// Post commit content cannot be greater than the maximum allowed
	if len(content) > MaxIssueContentLen {
		return fe(-1, makeField("content", commitHash), "post content "+
			"length exceeded max character limit")
	}

	return nil
}

// CheckIssuePostBody performs sanity checks on fields of an issue post body
func CheckIssuePostBody(commit types.Commit, fm map[string]interface{}) error {

	commitHash := commit.GetHash().String()

	obj := objx.New(fm)
	labels := obj.Get("labels")
	if !labels.IsNil() && !labels.IsInterSlice() {
		return fe(-1, makeField("labels", commitHash), "expected a list of string values")
	}

	assignees := obj.Get("assignees")
	if !assignees.IsNil() && !assignees.IsInterSlice() {
		return fe(-1, makeField("assignees", commitHash), "expected a list of string values")
	}

	// Check labels if set.
	if size := len(labels.InterSlice()); size > 0 {
		if size > 10 {
			return fe(-1, makeField("labels", commitHash), "too many labels; cannot exceed 10")
		}
		if !util.IsString(labels.InterSlice()[0]) {
			return fe(-1, makeField("labels", commitHash), "expected a string list")
		}
		for i, val := range labels.InterSlice() {
			if err := identifier.IsValidResourceNameNoMinLen(strings.TrimPrefix(val.(string), "-")); err != nil {
				return fe(i, makeField("labels", commitHash), err.Error())
			}
		}
	}

	// Check assignees if set.
	if val := assignees.InterSlice(); len(val) > 0 {
		if len(val) > 10 {
			return fe(-1, makeField("assignees", commitHash), "too many assignees; cannot exceed 10")
		}
		if !util.IsString(val[0]) {
			return fe(-1, makeField("assignees", commitHash), "expected a string list")
		}
		for i, assignee := range val {
			if !crypto.IsValidPushAddr(strings.TrimPrefix(assignee.(string), "-")) {
				return fe(i, makeField("assignees", commitHash), "invalid push key ID")
			}
		}
	}

	return nil
}

// CheckMergeRequestPostBody performs sanity and consistency checks on
// fields of a merge request post body
func CheckMergeRequestPostBody(
	repo types.LocalRepo,
	commit types.Commit,
	reference string,
	isNewRef bool,
	body map[string]interface{}) error {
	if err := checkMergeRequestPostBodySanity(commit, body, isNewRef); err != nil {
		return err
	}
	if err := CheckMergeRequestPostBodyConsistency(repo, reference, isNewRef, body); err != nil {
		return err
	}
	return nil
}

// CheckMergeRequestPostBodyConsistency performs consistency checks on
// fields of a merge request post body against the network state
func CheckMergeRequestPostBodyConsistency(
	repo types.LocalRepo,
	reference string,
	isNewRef bool,
	body map[string]interface{}) error {

	// For a non-new reference, get the merge request proposal and check if
	// it has been finalized; if it has, ensure the body does not include
	// merge request fields since a finalized merge request cannot be changed.
	if !isNewRef {
		id := mergerequest.MakeMergeRequestProposalID(plumbing2.GetReferenceShortName(reference))
		proposal := repo.GetState().Proposals.Get(id)
		if proposal == nil {
			return fmt.Errorf("merge request proposal not found") // should not happen
		} else if proposal.IsFinalized() {
			for _, f := range mergeReqFields {
				if _, ok := body[f]; ok {
					return fmt.Errorf("cannot update '%s' field of a finalized merge request proposal", f)
				}
			}
		}
	}

	return nil
}

// CheckMergeRequestPostBody performs sanity checks on fields of an merge request post body
func checkMergeRequestPostBodySanity(
	commit types.Commit,
	body map[string]interface{},
	isNewRef bool) error {

	var commitHash = commit.GetHash().String()

	obj := objx.New(body)
	base := obj.Get("base")
	if !base.IsNil() && !base.IsStr() {
		return fe(-1, makeField("base", commitHash), "expected a string value")
	}

	baseHash := obj.Get("baseHash")
	if !baseHash.IsNil() && !baseHash.IsStr() {
		return fe(-1, makeField("baseHash", commitHash), "expected a string value")
	}

	target := obj.Get("target")
	if !target.IsNil() && !target.IsStr() {
		return fe(-1, makeField("target", commitHash), "expected a string value")
	}

	targetHash := obj.Get("targetHash")
	if !targetHash.IsNil() && !targetHash.IsStr() {
		return fe(-1, makeField("targetHash", commitHash), "expected a string value")
	}

	// Base branch name is required for only new merge request reference
	if base.String() == "" && isNewRef {
		return fe(-1, makeField("base", commitHash), "base branch name is required")
	}

	if val := baseHash.String(); len(val) > 0 && len(val) != 40 {
		return fe(-1, makeField("baseHash", commitHash), "base branch hash is not valid")
	}

	// Target branch name is required for only new merge request reference
	if target.String() == "" && isNewRef {
		return fe(-1, makeField("target", commitHash), "target branch name is required")
	}

	// Target branch hash is required for only new merge request reference
	th := targetHash.String()
	if th == "" && isNewRef {
		return fe(-1, makeField("targetHash", commitHash), "target branch hash is required")
	}
	if len(th) > 0 && len(th) != 40 {
		return fe(-1, makeField("targetHash", commitHash), "target branch hash is not valid")
	}

	return nil
}

var makeField = func(name, commitHash string) string {
	return fmt.Sprintf("<commit#%s>.%s", commitHash[:7], name)
}

// CheckPostBody checks whether the front matter and content extracted from a post body is ok.
// repo: The target repo
// commit: The commit whose content needs to be checked.
// isNewRef: indicates that the post reference is new.
// fm: The front matter data.
// content: The content from the post commit.
func CheckPostBody(
	repo types.LocalRepo,
	reference string,
	commit types.Commit,
	isNewRef bool,
	fm map[string]interface{},
	content []byte) error {

	var commonFields = []string{"title", "reactions", "replyTo", "close"}
	var issueFields = []string{"labels", "assignees"}
	var allowedFields []string
	var isIssuePost = plumbing2.IsIssueReference(reference)
	var isMergeReqPost = plumbing2.IsMergeRequestReference(reference)

	// Check whether the fields are allowed for the type of post reference
	if isIssuePost {
		allowedFields = append(allowedFields, append(commonFields, issueFields...)...)
	} else if isMergeReqPost {
		allowedFields = append(allowedFields, append(commonFields, mergeReqFields...)...)
	} else {
		return fmt.Errorf("unsupported post type")
	}

	// Ensure only valid fields are included
	for k := range fm {
		if !funk.ContainsString(allowedFields, k) {
			return fe(-1, makeField(k, commit.GetHash().String()), "unexpected field")
		}
	}

	// Perform common checks
	if err := CheckCommonPostBody(repo, commit, isNewRef, fm, content); err != nil {
		return err
	}

	// Perform checks for issue post
	if isIssuePost {
		return CheckIssuePostBody(commit, fm)
	}

	// Perform checks for merge request post
	if isMergeReqPost {
		return CheckMergeRequestPostBody(repo, commit, reference, isNewRef, fm)
	}

	return nil
}
