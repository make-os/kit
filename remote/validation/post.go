package validation

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jinzhu/copier"
	"github.com/make-os/kit/logic/contracts/mergerequest"
	pl "github.com/make-os/kit/remote/plumbing"
	rr "github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/identifier"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
)

var (
	MaxIssueContentLen        = 1024 * 8 // 8KB
	MaxIssueTitleLen          = 256
	ErrCannotWriteToClosedRef = fmt.Errorf("cannot write to a closed reference")
	mergeReqFields            = []string{"base", "baseHash", "target", "targetHash"}
)

// ValidatePostCommitArg contains arguments for ValidatePostCommit
type ValidatePostCommitArg struct {
	Keepers         core.Keepers
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

	// Collect pushed commit ancestors if the target reference exists.
	var ancestors []*object.Commit
	reference := repo.GetState().References.Get(args.TxDetail.Reference)
	if !reference.IsNil() {
		ancestors, err = repo.GetAncestors(unwrapped, args.OldHash, true)
		if err != nil {
			return err
		}
	}

	// Add the pushed commit as the last ancestor
	ancestors = append(ancestors, unwrapped)

	// Validate the new post commits by replaying the commits individually
	// beginning from the first ancestor.
	for i, ancestor := range ancestors {

		// Define the post commit checker arguments
		pcArgs := &CheckPostCommitArgs{
			Keepers:   args.Keepers,
			Reference: args.TxDetail.Reference,
			OldHash:   args.OldHash,
			IsNew:     reference.IsNil(),
		}

		// If there are ancestors, set IsNew to false at index > 0.
		// It ensures CheckPostCommit does not treat the post reference as
		// a new one. Also, set OldHash to the previous ancestor to mimic
		// an already persisted post reference history.
		if i > 0 {
			pcArgs.IsNew = false
			pcArgs.OldHash = ancestors[i-1].Hash.String()
		}

		post, err := args.CheckPostCommit(repo, rr.WrapCommit(ancestor), pcArgs)
		if err != nil {
			return err
		}

		// Set flag for the authorizer function to check for admin update policy
		// if post includes admin fields
		args.TxDetail.FlagCheckAdminUpdatePolicy = post.IncludesAdminFields()

		// If ancestor is the tip (the pushed commit):
		// - Update reference data in tx detail with post data from tip commit.
		// - When a reference exist and it is closed, the next pushed commit is expected
		// 	 to reopen it by setting the 'close' field to 'open'. Return error if
		//   this is not the case.
		if ancestor.Hash.String() == args.Change.Item.GetData() {
			copier.Copy(args.TxDetail.Data(), post)
			if reference.Data.Closed && !post.WantOpen() {
				return ErrCannotWriteToClosedRef
			}
		}
	}

	return nil
}

// CheckPostCommitArgs includes arguments for CheckPostCommit function
type CheckPostCommitArgs struct {
	Keepers   core.Keepers
	Reference string
	OldHash   string
	IsNew     bool
}

// PostCommitChecker describes a function for validating a post commit.
type PostCommitChecker func(
	repo types.LocalRepo,
	commit types.Commit,
	args *CheckPostCommitArgs) (*pl.PostBody, error)

// CheckPostCommit validates new commits of a post reference. It returns nil post body
// and error if validation failed or a post body and nil if validation passed.
func CheckPostCommit(repo types.LocalRepo, commit types.Commit, args *CheckPostCommitArgs) (*pl.PostBody, error) {

	// Reference name must be valid
	if !pl.IsIssueReference(args.Reference) && !pl.IsMergeRequestReference(args.Reference) {
		return nil, fmt.Errorf("post number is not valid. Must be numeric")
	}

	// Post commits can't have multiple parents (merge commit not permitted)
	if commit.NumParents() > 1 {
		return nil, fmt.Errorf("post commit cannot have more than one parent")
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
	if err = CheckPostBody(args.Keepers, repo, args.Reference, commit, args.IsNew, cfm.FrontMatter, cfm.Content); err != nil {
		return nil, err
	}

	return pl.PostBodyFromContentFrontMatter(&cfm), nil
}

// CheckPostBody checks whether the front matter and content extracted from a post body is ok.
// keepers: The application state keepers
// repo: The target repo
// commit: The commit whose content needs to be checked.
// isNewRef: indicates that the post reference is new.
// fm: The front matter data.
// content: The content from the post commit.
func CheckPostBody(
	keepers core.Keepers,
	repo types.LocalRepo,
	reference string,
	commit types.Commit,
	isNewRef bool,
	fm map[string]interface{},
	content []byte) error {

	var commonFields = []string{"title", "reactions", "replyTo", "close"}
	var issueFields = []string{"labels", "assignees"}
	var allowedFields []string
	var isIssuePost = pl.IsIssueReference(reference)
	var isMergeReqPost = pl.IsMergeRequestReference(reference)

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
		return CheckMergeRequestPostBody(keepers, repo, commit, reference, isNewRef, fm)
	}

	return nil
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

	// Ensure title is provided if the post reference is new
	if isNewRef && len(title.String()) == 0 {
		return fe(-1, makeField("title", commitHash), "title is required")
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

// CheckMergeRequestPostBody performs sanity and consistency
// checks on post fields specific to a merge request
func CheckMergeRequestPostBody(
	keepers core.Keepers,
	repo types.LocalRepo,
	commit types.Commit,
	reference string,
	isNewRef bool,
	body map[string]interface{}) error {
	if err := checkMergeRequestPostBodySanity(commit, body, isNewRef); err != nil {
		return err
	}
	if err := CheckMergeRequestPostBodyConsistency(keepers, repo, reference, isNewRef, body); err != nil {
		return err
	}
	return nil
}

// CheckMergeRequestPostBodyConsistency performs consistency checks on
// fields of a merge request post body against the network state
func CheckMergeRequestPostBodyConsistency(
	keepers core.Keepers,
	repo types.LocalRepo,
	reference string,
	isNewRef bool,
	body map[string]interface{}) error {

	repoState := repo.GetState()

	// For old merge request reference:
	if !isNewRef {
		// Get the merge request proposal and check if
		// it has been finalized; if it has, ensure the body does not include
		// merge request fields since a finalized merge request cannot be changed.
		id := mergerequest.MakeMergeRequestProposalID(pl.GetReferenceShortName(reference))
		proposal := repoState.Proposals.Get(id)
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

	// If base branch is set, ensure it exists as reference in the repo state
	obj := objx.New(body)
	base := obj.Get("base").Str()
	fullBaseRef := plumbing.NewBranchReferenceName(base).String()
	if base != "" && !repoState.References.Has(fullBaseRef) {
		return fmt.Errorf("base branch (%s) is unknown", base)
	}

	// If the base branch hash is set, ensure the base branch hash matches
	// its equivalent reference hash in the repo state
	baseHash := obj.Get("baseHash").Str()
	curBaseHash := repoState.References.Get(fullBaseRef).Hash.HexStr(true)
	if baseHash != "" && baseHash != curBaseHash {
		return fmt.Errorf("base branch (%s) hash does not match upstream state", base)
	}

	// If target branch is set, ensure the target branch exist.
	// The target branch might be a path with the format '/repo_name/branch', in
	// this case, check if the branch exist in the repository named 'repo_name'
	var targetRepo = repoState
	target := obj.Get("target").Str()
	if target != "" {
		if target[:1] != "/" {
			if !repoState.References.Has(plumbing.NewBranchReferenceName(target).String()) {
				return fmt.Errorf("target branch (%s) is unknown", target)
			}
		} else {
			parts := strings.SplitN(target[1:], "/", 2)
			targetRepo = keepers.RepoKeeper().GetNoPopulate(parts[0])
			if targetRepo.IsEmpty() {
				return fmt.Errorf("target branch's repository (%s) does not exist", parts[0])
			}
			if !targetRepo.References.Has(plumbing.NewBranchReferenceName(parts[1]).String()) {
				return fmt.Errorf("target branch (%s) of (%s) is unknown", parts[1], parts[0])
			}
			target = parts[1]
		}
	}

	// If target hash is set, ensure the target branch hash matches
	// its equivalent reference hash in the target repo state
	targetHash := obj.Get("targetHash").Str()
	if targetHash != "" {
		fullTargetRef := plumbing.NewBranchReferenceName(target).String()
		curTargetHash := targetRepo.References.Get(fullTargetRef).Hash.HexStr(true)
		if targetHash != curTargetHash {
			return fmt.Errorf("target branch (%s) hash does not match upstream state", target)
		}
	}

	return nil
}

// checkMergeRequestPostBodySanity performs sanity checks on fields of an merge request post body
func checkMergeRequestPostBodySanity(commit types.Commit, body map[string]interface{}, isNewRef bool) error {

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

	// Base branch hash is required for only new merge request reference
	bh := baseHash.String()
	if bh == "" && isNewRef {
		return fe(-1, makeField("baseHash", commitHash), "base branch hash is required")
	}
	if len(bh) > 0 && len(bh) != 40 {
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
