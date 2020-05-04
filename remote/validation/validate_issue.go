package validation

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
)

var (
	MaxIssueContentLen = 1024 * 8 // 8KB
	MaxIssueTitleLen   = 256
)

// ValidateIssueCommitArg contains arguments for ValidateIssueCommit
type ValidateIssueCommitArg struct {
	OldHash          string
	Change           *core.ItemChange
	TxDetail         *types.TxDetail
	PushKeyGetter    core.PushKeyGetter
	CheckIssueCommit IssueCommitChecker
	CheckCommit      CommitChecker
}

// ValidateIssueCommit validates an issue commit.
func ValidateIssueCommit(localRepo core.BareRepo, args *ValidateIssueCommitArg) error {

	// Get the latest comment
	commitHash := plumbing.NewHash(args.Change.Item.GetData())
	commit, err := localRepo.WrappedCommitObject(commitHash)
	if err != nil {
		return errors.Wrap(err, "unable to get commit object")
	}

	// Check the latest commit using standard commit validation rules
	if err = args.CheckCommit(commit.UnWrap(), args.TxDetail, args.PushKeyGetter); err != nil {
		return err
	}

	// Now, check that the recent commit down to the old (pre-push) commit
	// pass issue commit validation
	for {
		if err = args.CheckIssueCommit(commit, args.TxDetail.Reference, args.OldHash, localRepo); err != nil {
			return err
		}
		if commit.NumParents() == 0 {
			break
		}
		parent, err := commit.Parent(0)
		if err != nil {
			return err
		}
		if parent.GetHash().String() == args.OldHash {
			break
		}
		commit = parent
	}

	return nil
}

// IssueCommitChecker describes a function for validating an issue commit
type IssueCommitChecker func(commit core.Commit, reference, oldHash string, repo core.BareRepo) error

// CheckIssueCommit checks commits of an issue branch.
func CheckIssueCommit(commit core.Commit, reference, oldHash string, repo core.BareRepo) error {

	// Issue reference name must be valid
	if !plumbing2.IsIssueReference(reference) {
		return fmt.Errorf("issue number is not valid. Must be numeric")
	}

	// Issue commits can't have multiple parents (merge commit not permitted)
	if commit.NumParents() > 1 {
		return fmt.Errorf("issue commit cannot have more than one parent")
	}

	// Issue commit history cannot have merge commits in it (merge commit not permitted)
	hasMerges, err := repo.HasMergeCommits(reference)
	if err != nil {
		return errors.Wrap(err, "failed to check for merges in issue commit history")
	} else if hasMerges {
		return fmt.Errorf("issue commit history must not include a merge commit")
	}

	// New issue's first commit must have zero hash parent (orphan commit)
	isNewIssue := !repo.GetState().References.Has(reference)
	if isNewIssue && commit.NumParents() != 0 {
		return fmt.Errorf("first commit of a new issue must have no parent")
	}

	// Issue commit history must not alter the current history (rebasing not permitted)
	if !isNewIssue && repo.IsAncestor(oldHash, commit.GetHash().String()) != nil {
		return fmt.Errorf("issue commit must not alter history")
	}

	tree, err := commit.GetTree()
	if err != nil {
		return fmt.Errorf("unable to read issue commit tree")
	}

	// Issue commit tree cannot be left empty
	if len(tree.Entries) == 0 {
		return fmt.Errorf("issue commit must have a 'body' file")
	}

	// Issue commit must include one file
	if len(tree.Entries) > 1 {
		return fmt.Errorf("issue commit tree must only include a 'body' file")
	}

	// Issue commit must include only a body file
	body := tree.Entries[0]
	if body.Mode != filemode.Regular {
		return fmt.Errorf("issue body file is not a regular file")
	}

	file, _ := tree.File(body.Name)
	content, err := file.Contents()
	if err != nil {
		return fmt.Errorf("issue body file could not be read")
	}

	// The body file must be parsable (extract front matter and content)
	cfm, err := pageparser.ParseFrontMatterAndContent(bytes.NewBufferString(content))
	if err != nil {
		return fmt.Errorf("issue body could not be parsed")
	}

	// Validate extracted front matter
	if err = CheckIssueBody(repo, commit, isNewIssue, cfm.FrontMatter, cfm.Content); err != nil {
		return err
	}

	return nil
}

// CheckIssueBody checks whether the front matter and content extracted from an issue body is ok.
// Valid front matter fields:
// - title: The title of the issue (optional)
// - labels: Labels categorize issues into arbitrary or conceptual units
// - replyTo: Indicates the issue is a response an earlier comment.
// - assignees: List push keys assigned to the issue and open for interpretation by clients.
// - fixers: List push keys assigned to fix an issue and is enforced by the protocol.
func CheckIssueBody(
	repo core.BareRepo,
	commit core.Commit,
	isNewIssue bool,
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
		"fixers"}
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

	fixers := obj.Get("fixers")
	if !fixers.IsNil() && !fixers.IsInterSlice() {
		return fe(-1, makeField("fixers"), "expected a list of string values")
	}

	// Ensure issue commit do not have a replyTo value
	if isNewIssue && len(replyTo.String()) > 0 {
		return fe(-1, makeField("replyTo"), "not expected in a new issue commit")
	}

	// Ensure title is unset when replyTo is set
	if len(replyTo.String()) > 0 && len(title.String()) > 0 {
		return fe(-1, makeField("title"), "title is not required when replying")
	}

	// Ensure title is provided if issue is new
	if isNewIssue && len(title.String()) == 0 {
		return fe(-1, makeField("title"), "title is required")
	} else if !isNewIssue && len(title.String()) > 0 {
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

	// When ReplyTo is set, ensure the issue commit is a descendant of the replyTo
	if len(replyToVal) > 0 {
		if repo.IsAncestor(replyToVal, commit.GetHash().String()) != nil {
			return fe(-1, makeField("replyTo"), "not a valid hash of a commit in the issue")
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
			if err := util.IsValidIdentifierName(strings.TrimPrefix(val.(string), "-")); err != nil {
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

	// Check fixers if set.
	if val := fixers.InterSlice(); len(val) > 0 {
		if len(val) > 10 {
			return fe(-1, makeField("fixers"), "too many fixers. Cannot exceed 10")
		}
		if !util.IsString(val[0]) {
			return fe(-1, makeField("fixers"), "expected a string list")
		}
		for i, fixer := range val {
			if !util.IsValidPushAddr(strings.TrimPrefix(fixer.(string), "-")) {
				return fe(i, makeField("fixers"), "invalid push key ID")
			}
		}
	}

	// Issue content is required for a new issue
	if isNewIssue && len(content) == 0 {
		return fe(-1, makeField("content"), "issue content is required")
	}

	// Issue content cannot be greater than the maximum allowed
	if len(content) > MaxIssueContentLen {
		return fe(-1, makeField("content"), "issue content length exceeded max character limit")
	}

	return nil
}
