package repo

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/mr-tron/base58"
	"github.com/stretchr/objx"
	"github.com/thoas/go-funk"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"

	gv "github.com/asaskevich/govalidator"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/crypto/bls"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	fe                               = util.FieldErrorWithIndex
	ErrSigHeaderAndReqParamsMismatch = fmt.Errorf("request transaction info and signature " +
		"transaction info did not match")
	MaxIssueContentLen = 1024 * 8 // 8KB
	MaxIssueTitleLen   = 256
)

type changeValidator func(
	repo core.BareRepo,
	oldHash string,
	change *core.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error

// validateChange validates a change to a repository
// repo: The target repository
// oldHash: The hash of the old reference
// change: The change to the reference
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func validateChange(
	repo core.BareRepo,
	oldHash string,
	change *core.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

	// Handle branch validation
	if isBranch(change.Item.GetName()) {
		commit, err := repo.CommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}

		if isIssueBranch(change.Item.GetName()) {
			return checkIssueCommit(wrapCommit(commit), txDetail.Reference, oldHash, repo)
		} else {
			return checkCommit(commit, txDetail, getPushKey)
		}
	}

	// Handle tag validation
	if isTag(change.Item.GetName()) {
		tagRef, err := repo.Tag(strings.ReplaceAll(change.Item.GetName(), "refs/tags/", ""))
		if err != nil {
			return errors.Wrap(err, "unable to get tag object")
		}

		// Get the tag object (for annotated tags)
		tagObj, err := repo.TagObject(tagRef.Hash())
		if err != nil && err != plumbing.ErrObjectNotFound {
			return err
		}

		// Here, the tag is not an annotated tag, so we need to
		// ensure the referenced commit is signed correctly
		if tagObj == nil {
			commit, err := repo.CommitObject(tagRef.Hash())
			if err != nil {
				return errors.Wrap(err, "unable to get commit")
			}
			return checkCommit(commit, txDetail, getPushKey)
		}

		// At this point, the tag is an annotated tag.
		// We have to ensure the annotated tag object is signed.
		return checkAnnotatedTag(tagObj, txDetail, getPushKey)
	}

	// Handle note validation
	if isNote(change.Item.GetName()) {
		return checkNote(repo, txDetail)
	}

	return fmt.Errorf("unrecognised change item")
}

// checkNote validates a note.
// repo: The repo where the tag exists in.
// txDetail: The pusher transaction detail
func checkNote(
	repo core.BareRepo,
	txDetail *types.TxDetail) error {

	// Get the note current hash
	noteHash, err := repo.RefGet(txDetail.Reference)
	if err != nil {
		return errors.Wrap(err, "failed to get note")
	}

	// Ensure the hash referenced in the tx detail matches the current note hash
	if noteHash != txDetail.Head {
		return fmt.Errorf("current note hash differs from signed note hash")
	}

	return nil
}

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func checkAnnotatedTag(tag *object.Tag, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. Sign the tag with your push key"
		return errors.Errorf(msg, tag.Hash.String())
	}

	pubKey, err := getPushKey(txDetail.PushKeyID)
	if err != nil {
		return errors.Wrapf(err, "failed to get pusher key(%s) to verify commit (%s)",
			txDetail.PushKeyID, tag.Hash.String())
	}

	tagTxDetail, err := verifyCommitOrTagSignature(tag, pubKey)
	if err != nil {
		return err
	}

	// Ensure the transaction detail from request matches the transaction
	// detail extracted from the signature header
	if !txDetail.Equal(tagTxDetail) {
		return ErrSigHeaderAndReqParamsMismatch
	}

	commit, err := tag.Commit()
	if err != nil {
		return errors.Wrap(err, "unable to get referenced commit")
	}

	return checkCommit(commit, txDetail, func(string) (key crypto.PublicKey, err error) {
		return pubKey, nil
	})
}

// getCommitOrTagSigMsg returns the message that is signed to create a commit or tag signature
func getCommitOrTagSigMsg(obj object.Object) string {
	encoded := &plumbing.MemoryObject{}
	switch o := obj.(type) {
	case *object.Commit:
		o.EncodeWithoutSignature(encoded)
	case *object.Tag:
		o.EncodeWithoutSignature(encoded)
	}
	rdr, _ := encoded.Reader()
	msg, _ := ioutil.ReadAll(rdr)
	return string(msg)
}

// verifyCommitSignature verifies commit and tag signatures
func verifyCommitOrTagSignature(obj object.Object, pubKey crypto.PublicKey) (*types.TxDetail, error) {
	var sig, hash string

	// Extract the signature for commit or tag object
	encoded := &plumbing.MemoryObject{}
	switch o := obj.(type) {
	case *object.Commit:
		o.EncodeWithoutSignature(encoded)
		sig, hash = o.PGPSignature, o.Hash.String()
	case *object.Tag:
		o.EncodeWithoutSignature(encoded)
		sig, hash = o.PGPSignature, o.Hash.String()
	default:
		return nil, fmt.Errorf("unsupported object type")
	}

	// Decode sig from PEM format
	pemBlock, _ := pem.Decode([]byte(sig))
	if pemBlock == nil {
		return nil, fmt.Errorf("signature is malformed")
	}

	// Re-construct the transaction parameters
	txDetail, err := types.TxDetailFromPEMHeader(pemBlock.Headers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode PEM header")
	}

	// Re-create the signature message
	rdr, _ := encoded.Reader()
	msg, _ := ioutil.ReadAll(rdr)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Verify the signature
	pk := crypto.MustPubKeyFromBytes(pubKey.Bytes())
	if ok, err := pk.Verify(msg, pemBlock.Bytes); !ok || err != nil {
		return nil, fmt.Errorf("object (%s) signature is invalid", hash)
	}

	return txDetail, nil
}

// checkCommit validates a commit
// repo: The target repo
// commit: The target commit object
// txDetail: The push transaction detail
// getPushKey: Getter function for fetching push public key
func checkCommit(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {

	// Signature must be set
	if commit.PGPSignature == "" {
		return fmt.Errorf("commit (%s) was not signed", commit.Hash.String())
	}

	// Get the push key for signature verification
	pubKey, err := getPushKey(txDetail.PushKeyID)
	if err != nil {
		return errors.Wrapf(err, "failed to get push key (%s)", txDetail.PushKeyID)
	}

	// Verify the signature
	commitTxDetail, err := verifyCommitOrTagSignature(commit, pubKey)
	if err != nil {
		return err
	}

	// Ensure the transaction detail from request matches the transaction
	// detail extracted from the signature header
	if !txDetail.Equal(commitTxDetail) {
		return ErrSigHeaderAndReqParamsMismatch
	}

	return nil
}

// checkIssueCommit checks commits of an issue branch.
func checkIssueCommit(commit core.Commit, reference, oldHash string, repo core.BareRepo) error {

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
	isNewIssue := !repo.State().References.Has(reference)
	if isNewIssue {
		if commit.NumParents() != 0 {
			return fmt.Errorf("first commit of a new issue must have no parent")
		}
	}

	// Issue commit history must not alter the current history (rebasing not permitted)
	if !isNewIssue && repo.IsDescendant(commit.GetHash().String(), oldHash) != nil {
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
	if err = checkIssueBody(repo, commit, isNewIssue, cfm.FrontMatter, cfm.Content); err != nil {
		return errors.Wrap(err, "issue body has invalid front matter")
	}

	return nil
}

// checkIssueBody checks whether the front matter and content extracted from an issue body is ok.
// Valid front matter fields:
// - title: The title of the issue (optional)
// - labels: Labels categorize issues into arbitrary or conceptual units
// - replyTo: Indicates the issue is a response an earlier comment.
// - assignees: List push keys assigned to the issue and open for interpretation by clients.
// - fixers: List push keys assigned to fix an issue and is enforced by the protocol.
func checkIssueBody(
	repo core.BareRepo,
	commit core.Commit,
	isNewIssue bool,
	fm map[string]interface{},
	content []byte) error {

	// Ensure only valid fields are included
	var validFields = []string{"title", "labels", "replyTo", "assignees", "fixers"}
	for k := range fm {
		if !funk.ContainsString(validFields, k) {
			return fe(-1, k, "unknown field")
		}
	}

	obj := objx.New(fm)

	title := obj.Get("title")
	if !title.IsNil() && !title.IsStr() {
		return fe(-1, "title", "expected a string value")
	}

	replyTo := obj.Get("replyTo")
	if !replyTo.IsNil() && !replyTo.IsStr() {
		return fe(-1, "replyTo", "expected a string value")
	}

	labels := obj.Get("labels")
	if !labels.IsNil() && !labels.IsInterSlice() {
		return fe(-1, "labels", "expected a list of string values")
	}

	assignees := obj.Get("assignees")
	if !assignees.IsNil() && !assignees.IsInterSlice() {
		return fe(-1, "assignees", "expected a list of string values")
	}

	fixers := obj.Get("fixers")
	if !fixers.IsNil() && !fixers.IsInterSlice() {
		return fe(-1, "fixers", "expected a list of string values")
	}

	// Ensure issue commit do not have a replyTo value
	if isNewIssue && len(replyTo.String()) > 0 {
		return fe(-1, "replyTo", "not expected in a new issue commit")
	}

	// Ensure title is unset when replyTo is set
	if len(replyTo.String()) > 0 && len(title.String()) > 0 {
		return fe(-1, "title", "title is not required when replying")
	}

	// Ensure title is provided if issue is new
	if isNewIssue && len(title.String()) == 0 {
		return fe(-1, "title", "title is required")
	} else if !isNewIssue && len(title.String()) > 0 {
		return fe(-1, "title", "title is not required for comment commit")
	}

	// Title cannot exceed max.
	if len(title.String()) > MaxIssueTitleLen {
		return fe(-1, "title", "title is too long and cannot exceed 256 characters")
	}

	// ReplyTo must have len 40
	replyToVal := replyTo.String()
	if len(replyToVal) > 0 && len(replyToVal) != 40 {
		return fe(-1, "replyTo", "invalid hash value")
	}

	// When ReplyTo is set, ensure the issue commit is a descendant of the replyTo
	if len(replyToVal) > 0 {
		if repo.IsDescendant(commit.GetHash().String(), replyToVal) != nil {
			return fe(-1, "replyTo", "not a valid hash of a commit in the issue")
		}
	}

	// Check labels if set.
	// Labels cannot exceed 10.
	if len(labels.InterSlice()) > 10 {
		return fe(-1, "labels", "too many labels. Cannot exceed 10")
	}
	if len(labels.InterSlice()) > 0 {
		if reflect.TypeOf(labels.InterSlice()[0]).Kind() != reflect.String {
			return fe(-1, "labels", "expected a string list of labels")
		}
	}

	// Check assignees if set.
	// Assignees cannot exceed 10.
	// Assignees must be valid push key IDs
	if len(assignees.InterSlice()) > 10 {
		return fe(-1, "assignees", "too many assignees. Cannot exceed 10")
	}
	for i, assignee := range assignees.InterSlice() {
		pkID, ok := assignee.(string)
		if !ok {
			return fe(-1, "assignees", "expected a string list of push keys")
		}
		if !util.IsValidPushKeyID(pkID) {
			return fe(-1, fmt.Sprintf("assignees[%d]", i), "invalid push key ID")
		}
	}

	// Check fixers if set.
	// Fixers cannot exceed 10.
	// Fixers must be valid push key IDs
	if len(fixers.InterSlice()) > 10 {
		return fe(-1, "fixers", "too many fixers. Cannot exceed 10")
	}
	for i, assignee := range fixers.InterSlice() {
		pkID, ok := assignee.(string)
		if !ok {
			return fe(-1, "fixers", "expected a string list of push keys")
		}
		if !util.IsValidPushKeyID(pkID) {
			return fe(-1, fmt.Sprintf("fixers[%d]", i), "invalid push key ID")
		}
	}

	// Issue content cannot be greater than the maximum
	if len(content) > MaxIssueContentLen {
		return fe(-1, "content", "issue content length exceeded max character limit")
	}

	return nil
}

// CheckPushNoteSyntax performs syntactic checks on the fields of a push transaction
func CheckPushNoteSyntax(tx *core.PushNote) error {

	if tx.RepoName == "" {
		return util.FieldError("repoName", "repo name is required")
	}
	if util.IsValidIdentifierName(tx.RepoName) != nil {
		return util.FieldError("repoName", "repo name is not valid")
	}

	if tx.Namespace != "" && util.IsValidIdentifierName(tx.Namespace) != nil {
		return util.FieldError("namespace", "namespace is not valid")
	}

	fe := fe
	for i, ref := range tx.References {
		if ref.Name == "" {
			return fe(i, "references.name", "name is required")
		}
		if ref.OldHash == "" {
			return fe(i, "references.oldHash", "old hash is required")
		}
		if len(ref.OldHash) != 40 {
			return fe(i, "references.oldHash", "old hash is not valid")
		}
		if ref.NewHash == "" {
			return fe(i, "references.newHash", "new hash is required")
		}
		if len(ref.NewHash) != 40 {
			return fe(i, "references.newHash", "new hash is not valid")
		}
		if ref.Nonce == 0 {
			return fe(i, "references.nonce", "reference nonce must be greater than zero")
		}
		for j, obj := range ref.Objects {
			if len(obj) != 40 {
				return fe(i, fmt.Sprintf("references.objects.%d", j), "object hash is not valid")
			}
		}

		if ref.Fee == "" {
			return fe(i, "fee", "fee is required")
		}
		if !gv.IsFloat(ref.Fee.String()) {
			return fe(i, "fee", "fee must be numeric")
		}

		if ref.MergeProposalID != "" {
			return checkMergeProposalID(ref.MergeProposalID, i)
		}

		if len(ref.PushSig) == 0 {
			return fe(i, "pushSig", "signature is required")
		}
	}

	if len(tx.PushKeyID) == 0 {
		return util.FieldError("pusherKeyId", "push key id is required")
	}
	if len(tx.PushKeyID) != 20 {
		return util.FieldError("pusherKeyId", "push key id is not valid")
	}

	if tx.Timestamp == 0 {
		return util.FieldError("timestamp", "timestamp is required")
	}
	if tx.Timestamp > time.Now().Unix() {
		return util.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if tx.PusherAcctNonce == 0 {
		return util.FieldError("accountNonce", "account nonce must be greater than zero")
	}

	if tx.NodePubKey.IsEmpty() {
		return util.FieldError("nodePubKey", "push node public key is required")
	}

	pk, err := crypto.PubKeyFromBytes(tx.NodePubKey.Bytes())
	if err != nil {
		return util.FieldError("nodePubKey", "push node public key is not valid")
	}

	if len(tx.NodeSig) == 0 {
		return util.FieldError("nodeSig", "push node signature is required")
	}

	if ok, err := pk.Verify(tx.BytesNoSig(), tx.NodeSig); err != nil || !ok {
		return util.FieldError("nodeSig", "failed to verify signature")
	}

	return nil
}

// checkPushedReferenceConsistency validates pushed references
func checkPushedReferenceConsistency(
	targetRepo core.BareRepo,
	ref *core.PushedReference,
	repo *state.Repository) error {

	name, nonce := ref.Name, ref.Nonce
	oldHashIsZero := plumbing.NewHash(ref.OldHash).IsZero()

	// We need to check if the reference exists in the repo.
	// Ignore references whose old hash is a 0-hash, these are new
	// references and as such we don't expect to find it in the repo.
	if !oldHashIsZero && !repo.References.Has(name) {
		msg := fmt.Sprintf("reference '%s' is unknown", name)
		return util.FieldError("references", msg)
	}

	// If target repo is set and old hash is non-zero, we need to ensure
	// the current hash of the local version of the reference is the same as the old hash,
	// otherwise the pushed reference will not be compatible.
	if targetRepo != nil && !oldHashIsZero {
		localRef, err := targetRepo.Reference(plumbing.ReferenceName(name), false)
		if err != nil {
			msg := fmt.Sprintf("reference '%s' does not exist locally", name)
			return util.FieldError("references", msg)
		}
		if ref.OldHash != localRef.Hash().String() {
			msg := fmt.Sprintf("reference '%s' old hash does not match its local version", name)
			return util.FieldError("references", msg)
		}
	}

	// We need to check that the nonce is the expected next nonce of the
	// reference, otherwise we return an error.
	refInfo := repo.References.Get(name)
	nextNonce := refInfo.Nonce + 1
	if nextNonce != ref.Nonce {
		msg := fmt.Sprintf("reference '%s' has nonce '%d', expecting '%d'", name, nonce, nextNonce)
		return util.FieldError("references", msg)
	}

	return nil
}

// CheckPushNoteConsistency performs consistency checks against the state of the
// repository as seen by the node. If the target repo object is not set in tx,
// local reference hash comparision is not performed.
func CheckPushNoteConsistency(note *core.PushNote, logic core.Logic) error {

	// Ensure the repository exist
	repo := logic.RepoKeeper().Get(note.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", note.GetRepoName())
		return util.FieldError("repoName", msg)
	}

	// If namespace is provide, ensure it exists
	if note.Namespace != "" {
		if logic.NamespaceKeeper().Get(util.HashNamespace(note.Namespace)).IsNil() {
			return util.FieldError("namespace", fmt.Sprintf("namespace '%s' is unknown", note.Namespace))
		}
	}

	// Get push key of the pusher
	pushKey := logic.PushKeyKeeper().Get(crypto.BytesToPushKeyID(note.PushKeyID))
	if pushKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", note.PushKeyID)
		return util.FieldError("pusherKeyId", msg)
	}

	// Ensure the push key linked address matches the pusher address
	if pushKey.Address != note.PusherAddress {
		return util.FieldError("pusherAddr", "push key does not belong to pusher")
	}

	// Ensure next pusher account nonce matches the note's account nonce
	pusherAcct := logic.AccountKeeper().Get(note.PusherAddress)
	if pusherAcct.IsNil() {
		return util.FieldError("pusherAddr", "pusher account not found")
	} else if note.PusherAcctNonce != pusherAcct.Nonce+1 {
		msg := fmt.Sprintf("wrong account nonce '%d', expecting '%d'",
			note.PusherAcctNonce, pusherAcct.Nonce+1)
		return util.FieldError("accountNonce", msg)
	}

	// Check each references against the state
	for i, ref := range note.GetPushedReferences() {
		if err := checkPushedReferenceConsistency(note.GetTargetRepo(), ref, repo); err != nil {
			return err
		}

		// Verify signature
		txDetail := getTxDetailsFromNote(note, ref.Name)[0]
		pushPubKey := crypto.MustPubKeyFromBytes(pushKey.PubKey.Bytes())
		if ok, err := pushPubKey.Verify(txDetail.BytesNoSig(), ref.PushSig); err != nil || !ok {
			msg := fmt.Sprintf("reference (%s) signature is not valid", ref.Name)
			return fe(i, "references", msg)
		}
	}

	// Check whether the pusher can pay the specified transaction fee
	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}
	if err = logic.Tx().CanExecCoinTransfer(
		note.PusherAddress,
		"0",
		note.GetFee(),
		note.PusherAcctNonce,
		uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// pushNoteChecker describes a function for checking a push note
type pushNoteChecker func(tx core.RepoPushNote, dht dhttypes.DHTNode, logic core.Logic) error

// checkPushNote performs validation checks on a push transaction
func checkPushNote(tx core.RepoPushNote, dht dhttypes.DHTNode, logic core.Logic) error {

	if err := CheckPushNoteSyntax(tx.(*core.PushNote)); err != nil {
		return err
	}

	if err := CheckPushNoteConsistency(tx.(*core.PushNote), logic); err != nil {
		return err
	}

	err := fetchAndCheckReferenceObjects(tx, dht)
	if err != nil {
		return err
	}

	return nil
}

// CheckPushEnd performs sanity checks on the given PushEndorsement object
func CheckPushEnd(pushEnd *core.PushEndorsement, index int) error {

	// Push note id must be set
	if pushEnd.NoteID.IsEmpty() {
		return fe(index, "endorsements.pushNoteID", "push note id is required")
	}

	// Sender public key must be set
	if pushEnd.EndorserPubKey.IsEmpty() {
		return fe(index, "endorsements.senderPubKey", "sender public key is required")
	}

	return nil
}

// CheckPushEndConsistencyUsingHost performs consistency checks on the given PushEndorsement object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushEnd
func CheckPushEndConsistencyUsingHost(
	hosts tickettypes.SelectedTickets,
	pushEnd *core.PushEndorsement,
	logic core.Logic,
	noSigCheck bool,
	index int) error {

	// Check if the sender is one of the top hosts.
	// Ensure that the signers of the PushEndorsement are part of the hosts
	signerSelectedTicket := hosts.Get(pushEnd.EndorserPubKey)
	if signerSelectedTicket == nil {
		return fe(index, "endorsements.senderPubKey",
			"sender public key does not belong to an active host")
	}

	// Ensure the signature can be verified using the BLS public key of the selected ticket
	if !noSigCheck {
		blsPubKey, err := bls.BytesToPublicKey(signerSelectedTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		if err = blsPubKey.Verify(pushEnd.Sig.Bytes(), pushEnd.BytesNoSigAndSenderPubKey()); err != nil {
			return fe(index, "endorsements.sig", "signature could not be verified")
		}
	}

	return nil
}

// CheckPushEndConsistency performs consistency checks on the given PushEndorsement object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushEnd
func CheckPushEndConsistency(pushEnd *core.PushEndorsement, logic core.Logic, noSigCheck bool, index int) error {
	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}
	return CheckPushEndConsistencyUsingHost(hosts, pushEnd, logic, noSigCheck, index)
}

// checkPushEnd performs sanity and state consistency checks on the given PushEndorsement object
func checkPushEnd(pushEnd *core.PushEndorsement, logic core.Logic, index int) error {
	if err := CheckPushEnd(pushEnd, index); err != nil {
		return err
	}
	if err := CheckPushEndConsistency(pushEnd, logic, false, index); err != nil {
		return err
	}
	return nil
}

// fetchAndCheckReferenceObjects attempts to fetch and store new objects
// introduced by the pushed references. After fetching it performs checks
// on the objects
func fetchAndCheckReferenceObjects(tx core.RepoPushNote, dhtnode dhttypes.DHTNode) error {
	objectsSize := int64(0)

	for _, objHash := range tx.GetPushedObjects() {
	getSize:
		// Attempt to get the object's size. If we find it, it means the object
		// already exist so we don't have to fetch it from the dht.
		objSize, err := tx.GetTargetRepo().GetObjectSize(objHash)
		if err == nil {
			objectsSize += objSize
			continue
		}

		// Since the object doesn't exist locally, read the object from the DHTNode
		dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		objValue, err := dhtnode.GetObject(ctx, &dhttypes.DHTObjectQuery{
			Module:    core.RepoObjectModule,
			ObjectKey: []byte(dhtKey),
		})
		if err != nil {
			cn()
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}
		cn()

		// Write the object's content to disk
		if err := tx.GetTargetRepo().WriteObjectToFile(objHash, objValue); err != nil {
			msg := fmt.Sprintf("failed to write fetched object '%s' to disk", objHash)
			return errors.Wrap(err, msg)
		}

		goto getSize
	}

	if objectsSize != int64(tx.GetSize()) {
		msg := fmt.Sprintf("invalid size (%d bytes). "+
			"actual object size (%d bytes) is different", tx.GetSize(), objectsSize)
		return util.FieldError("size", msg)
	}

	return nil
}

// txDetailChecker describes a function for checking a transaction detail
type txDetailChecker func(params *types.TxDetail, keepers core.Keepers, index int) error

// checkTxDetail performs sanity and consistency checks on a transaction's parameters.
func checkTxDetail(params *types.TxDetail, keepers core.Keepers, index int) error {
	if err := checkTxDetailSanity(params, index); err != nil {
		return err
	}
	return checkTxDetailConsistency(params, keepers, index)
}

// checkTxDetailSanity performs sanity checks on a transaction's parameters.
// When authScope is true, only fields necessary for authentication are validated.
func checkTxDetailSanity(params *types.TxDetail, index int) error {

	// Push key is required and must be valid
	if params.PushKeyID == "" {
		return fe(index, "pkID", "push key id is required")
	} else if !util.IsValidPushKeyID(params.PushKeyID) {
		return fe(index, "pkID", "push key id is not valid")
	}

	// Nonce must be set
	if params.Nonce == 0 {
		return fe(index, "nonce", "nonce is required")
	}

	// Fee must be set
	if params.Fee.String() == "" {
		return fe(index, "fee", "fee is required")
	}

	// Fee must be numeric
	if !gv.IsFloat(params.Fee.String()) {
		return fe(index, "fee", "fee must be numeric")
	}

	// Signature format must be valid
	if _, err := base58.Decode(params.Signature); err != nil {
		return fe(index, "sig", "signature format is not valid")
	}

	// Merge proposal, if set, must be numeric and have 8 bytes length max.
	if params.MergeProposalID != "" {
		return checkMergeProposalID(params.MergeProposalID, index)
	}

	return nil
}

// checkMergeProposalID performs sanity checks on merge proposal ID
func checkMergeProposalID(id string, index int) error {
	if !gv.IsNumeric(id) {
		return fe(index, "mergeID", "merge proposal id must be numeric")
	}
	if len(id) > 8 {
		return fe(index, "mergeID", "merge proposal id exceeded 8 bytes limit")
	}
	return nil
}

// isBlockedByScope checks whether the given tx parameter satisfy a given scope
func isBlockedByScope(scopes []string, params *types.TxDetail, namespaceFromParams *state.Namespace) bool {
	blocked := true
	for _, scope := range scopes {
		if util.IsNamespaceURI(scope) {
			ns, domain, _ := util.SplitNamespaceDomain(scope)

			// If scope is r/repo-name, make sure tx info namespace is unset and repo name is 'repo-name'.
			// If scope is r/ only, make sure only tx info namespace is set
			if ns == DefaultNS && params.RepoNamespace == "" && (domain == "" || domain == params.RepoName) {
				blocked = false
				break
			}

			// If scope is some_ns/repo-name, make sure tx info namespace and repo name matches the scope
			// namespace and repo name.
			if ns != DefaultNS && ns == params.RepoNamespace && domain == params.RepoName {
				blocked = false
				break
			}

			// If scope is just some_ns/, make sure tx info namespace matches
			if ns != DefaultNS && domain == "" && ns == params.RepoNamespace {
				blocked = false
				break
			}
		}

		// At this point, the scope is just a target repo name.
		// e.g unblock if tx info namespace is default and the repo name matches the scope
		if params.RepoNamespace == "" && params.RepoName == scope {
			blocked = false
			break
		}

		// But if the scope's repo name is set, ensure the domain target matches the sc
		if params.RepoNamespace != "" {
			if target := namespaceFromParams.Domains[params.RepoName]; target != "" && target[2:] == scope {
				blocked = false
				break
			}
		}
	}

	return blocked
}

// checkTxDetailConsistency performs consistency checks on a transaction's parameters.
func checkTxDetailConsistency(params *types.TxDetail, keepers core.Keepers, index int) error {

	// Pusher key must exist
	pushKey := keepers.PushKeyKeeper().Get(params.PushKeyID)
	if pushKey.IsNil() {
		return fe(index, "pkID", "push key not found")
	}

	// Ensure repo namespace exist if set
	var paramNS = state.BareNamespace()
	if params.RepoNamespace != "" && len(pushKey.Scopes) > 0 {
		paramNS = keepers.NamespaceKeeper().Get(params.RepoNamespace)
		if paramNS.IsNil() {
			msg := fmt.Sprintf("namespace (%s) is unknown", params.RepoNamespace)
			return fe(index, "repoNamespace", msg)
		}
	}

	// Ensure push key scope grants access to the destination repo namespace and repo name.
	if len(pushKey.Scopes) > 0 {
		if isBlockedByScope(pushKey.Scopes, params, paramNS) {
			msg := fmt.Sprintf("push key (%s) not permitted due to scope limitation", params.PushKeyID)
			return fe(index, "repoName|repoNamespace", msg)
		}
	}

	// Ensure the nonce is a future nonce (> current nonce of pusher's account)
	keyOwner := keepers.AccountKeeper().Get(pushKey.Address)
	if params.Nonce <= keyOwner.Nonce {
		msg := fmt.Sprintf("nonce (%d) must be greater than current key owner nonce (%d)", params.Nonce,
			keyOwner.Nonce)
		return fe(index, "nonce", msg)
	}

	// When merge proposal ID is set, check if merge proposal exist and
	// whether it was created by the owner of the push key
	if params.MergeProposalID != "" {
		repoState := keepers.RepoKeeper().Get(params.RepoName)
		mp := repoState.Proposals.Get(params.MergeProposalID)
		if mp == nil {
			return fe(index, "mergeID", "merge proposal not found")
		}
		if mp.Action != core.TxTypeRepoProposalMergeRequest {
			return fe(index, "mergeID", "proposal is not a merge request")
		}
		if mp.Creator != pushKey.Address.String() {
			return fe(index, "mergeID", "merge error: signer did not create the proposal")
		}
	}

	// Use the key to verify the tx params signature
	pubKey, _ := crypto.PubKeyFromBytes(pushKey.PubKey.Bytes())
	if ok, err := pubKey.Verify(params.BytesNoSig(), params.MustSignatureAsBytes()); err != nil || !ok {
		return fe(index, "sig", "signature is not valid")
	}

	return nil
}

type mergeComplianceChecker func(
	repo core.BareRepo,
	change *core.ItemChange,
	oldRef core.Item,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error

// checkMergeCompliance checks whether push to a branch satisfied
// an accepted merge proposal
func checkMergeCompliance(
	repo core.BareRepo,
	change *core.ItemChange,
	oldRef core.Item,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error {

	ref := plumbing.ReferenceName(change.Item.GetName())
	if !ref.IsBranch() {
		return fmt.Errorf("merge error: pushed reference must be a branch")
	}

	prop := repo.State().Proposals.Get(mergeProposalID)
	if prop == nil {
		return fmt.Errorf("merge error: target merge proposal was not found")
	}

	// Ensure the signer is the creator of the proposal
	pushKey := keepers.PushKeyKeeper().Get(pushKeyID)
	if pushKey.Address.String() != prop.Creator {
		return fmt.Errorf("merge error: push key owner did not create the proposal")
	}

	// Check if the merge proposal has been closed
	closed, err := keepers.RepoKeeper().IsProposalClosed(repo.GetName(), mergeProposalID)
	if err != nil {
		return fmt.Errorf("merge error: %s", err)
	} else if closed {
		return fmt.Errorf("merge error: target merge proposal is already closed")
	}

	// Ensure the proposal's base branch matches the pushed branch
	var propBaseBranch string
	_ = util.ToObject(prop.ActionData[constants.ActionDataKeyBaseBranch], &propBaseBranch)
	if ref.Short() != propBaseBranch {
		return fmt.Errorf("merge error: pushed branch name and proposal base branch name must match")
	}

	// Check whether the merge proposal has been accepted
	if !prop.IsAccepted() {
		if prop.Outcome == 0 {
			return fmt.Errorf("merge error: target merge proposal is undecided")
		} else {
			return fmt.Errorf("merge error: target merge proposal was not accepted")
		}
	}

	// Get the commit that initiated the merge operation (a.k.a "pushed commit").
	// Since by convention, its parent is considered the actual merge target.
	// As such, we need to perform some validation before we compare it with
	// the merge proposal target hash.
	commit, err := repo.WrappedCommitObject(plumbing.NewHash(change.Item.GetData()))
	if err != nil {
		return errors.Wrap(err, "unable to get commit object")
	}

	var propTargetHash string
	util.ToObject(prop.ActionData[constants.ActionDataKeyTargetHash], &propTargetHash)

	// By default, the parent of the merge commit is target commit...
	targetCommit, _ := commit.Parent(0)

	// ...unless the merge commit is the proposal target, in which case
	// we use the commit as the target hash.
	if propTargetHash == commit.GetHash().String() {
		targetCommit = commit
	}

	// When the merge commit has parents, ensure the proposal target is a parent.
	// Extract it and use as the target commit.
	if commit.NumParents() > 1 {
		_, targetCommit = commit.IsParent(propTargetHash)
		if targetCommit == nil {
			return fmt.Errorf("merge error: target hash is not a parent of the merge commit")
		}
	}

	// Ensure the difference between the target commit and the pushed commit
	// only exist in the commit hash and not the tree, author and committer information.
	// By convention, the pushed commit can only modify its commit object (time,
	// message and signature).
	if commit.GetTreeHash() != targetCommit.GetTreeHash() ||
		commit.GetAuthor().String() != targetCommit.GetAuthor().String() ||
		commit.GetCommitter().String() != targetCommit.GetCommitter().String() {
		return fmt.Errorf("merge error: pushed commit must not modify target branch history")
	}

	// When no older reference (ex. a new/first branch),
	// set default hash value to zero hash.
	oldRefHash := plumbing.ZeroHash.String()
	if oldRef != nil {
		oldRefHash = oldRef.GetData()
	}

	// When no base hash is given, set default hash value to zero hash
	var propBaseHash string
	_ = util.ToObject(prop.ActionData[constants.ActionDataKeyBaseHash], &propBaseHash)
	propBaseHashStr := plumbing.ZeroHash.String()
	if propBaseHash != "" {
		propBaseHashStr = propBaseHash
	}

	// Ensure the proposals base branch hash matches the hash of the current
	// branch before this current push/change.
	if propBaseHashStr != oldRefHash {
		return fmt.Errorf("merge error: target merge proposal base branch hash is stale or invalid")
	}

	// Ensure the target commit and the proposal target match
	if targetCommit.GetHash().String() != propTargetHash {
		return fmt.Errorf("merge error: target commit hash and the merge proposal target hash must match")
	}

	return nil
}
