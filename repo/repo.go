package repo

import (
	"crypto/rsa"
	"fmt"
	"os"
	"strings"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/thoas/go-funk"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/pkg/errors"
)

// Repo represents a git repository
type Repo struct {
	*git.Repository
	*GitOps
	*DBOps
	Path string
	Name string
}

// getCurrentWDRepo returns a Repo instance pointed to the repository
// in the current working directory.
func getCurrentWDRepo(gitBinDir string) (*Repo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current working directory")
	}

	// Since we expect the working directory to be a git working tree,
	// we need to get a repo instance to verify it
	repo, err := getRepoWithGitOpt(gitBinDir, wd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open repository")
	} else if repoCfg, _ := repo.Config(); repoCfg.Core.IsBare {
		return nil, errors.New("expected a working tree. this is a bare repository")
	}

	return repo, nil
}

// amendRecentCommitTxLine attempts to add or amend transaction argument to
// the recent commit. If no transaction line exist, it will add a new
// one populated with the provided arguments.
// Transaction line allows the user to set transaction arguments such as
// fee, public key, etc. A txline has the format tx: fee=10, pk=ad1..xyz, nonce=1
func amendRecentCommitTxLine(gitBinDir, txFee, txNonce, signingKey string) error {

	repo, err := getCurrentWDRepo(gitBinDir)
	if err != nil {
		return err
	}

	// Get the signing key id from the git config if not passed as an argument
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Get recent commit hash of the current branch
	hash, err := repo.GetRecentCommit()
	if err != nil {
		if err == ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	commit, _ := repo.CommitObject(plumbing.NewHash(hash))
	msg := util.RemoveTxLine(commit.Message)

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPublicKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// TODO:If nonce is not provided, get nonce from a --source (remote node or local
	// node)

	// Construct the tx line and append to the current message
	txLine := util.MakeTxLine(txFee, txNonce, pkID, nil)
	msg += "\n\n" + txLine

	// Update the recent commit message
	if err = repo.UpdateRecentCommitMsg(msg, signingKey); err != nil {
		return err
	}

	return nil
}

// createTagWithTxLine creates a tag and adds a txline to the tag message
func createTagWithTxLine(args []string, gitBinDir, txFee, txNonce, signingKey string) error {

	repo, err := getCurrentWDRepo(gitBinDir)
	if err != nil {
		return err
	}

	parsed := util.ParseSimpleArgs(args)

	// If -u flag is provided in the git args, use it a signing key
	if parsed["u"] != "" {
		signingKey = parsed["u"]
	}
	// Get the signing key id from the git config if not passed via app -u flag
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	// Return error if we still don't have a signing key
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Get the user-supplied message from the arguments provided
	msg := ""
	if m, ok := parsed["m"]; ok {
		msg = m
	} else if message, ok := parsed["message"]; ok {
		msg = message
	}

	// Remove -m or --message flag from args
	args = util.RemoveFlagVal(args, []string{"m", "message", "u"})

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPublicKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// TODO:If nonce is not provided, get nonce from a --source (remote node or local
	// node)

	// Construct the tx line and append to the current message
	txLine := util.MakeTxLine(txFee, txNonce, pkID, nil)
	msg += "\n\n" + txLine

	// Create the tag
	if err = repo.CreateTagWithMsg(args, msg, signingKey); err != nil {
		return err
	}

	return nil
}

// addSignedTxBlob creates a blob object that contains a signed tx line.
func addSignedTxBlob(gitBinDir, txFee, txNonce, signingKey, note string) error {

	repo, err := getCurrentWDRepo(gitBinDir)
	if err != nil {
		return err
	}

	// Get the signing key id from the git config if not provided via -s flag
	if signingKey == "" {
		signingKey = repo.GetConfig("user.signingKey")
	}
	// Return error if we still don't have a signing key
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Enforce the inclusion of `refs/notes` to the note argument
	if !strings.HasPrefix("refs/notes", note) {
		note = "refs/notes/" + note
	}

	// Find a list of all notes entries in the note
	noteEntries, err := repo.ListTreeObjects(note, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries for tree object (%s)", note)
		return errors.Wrap(err, msg)
	}

	// From the entries, find existing tx blob and stop after the first one
	var lastTxBlob *object.Blob
	for hash := range noteEntries {
		obj, err := repo.BlobObject(plumbing.NewHash(hash))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to read object (%s)", hash))
		}
		r, err := obj.Reader()
		if err != nil {
			return err
		}
		prefix := make([]byte, 3)
		r.Read(prefix)
		if string(prefix) == util.TxLinePrefix {
			lastTxBlob = obj
			break
		}
	}

	// Remove the last tx blob from the note, if present
	if lastTxBlob != nil {
		err = repo.RemoveEntryFromNote(note, noteEntries[lastTxBlob.Hash.String()])
		if err != nil {
			return errors.Wrap(err, "failed to delete existing transaction blob")
		}
	}

	// Get the commit hash the note is currently referencing.
	// We need to add this hash to the signature.
	noteRef, err := repo.Reference(plumbing.ReferenceName(note), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	noteHash := noteRef.Hash().String()

	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPrivateKey(signingKey, repo.GetConfig("gpg.program"), "")
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}

	// Get the public key network ID
	pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	// TODO:If nonce is not provided, get nonce from a --source (remote node or local
	// node)

	// Sign a message composed of the tx information
	// fee + nonce + public key id + note hash
	sigMsg := []byte(txFee + txNonce + pkID + noteHash)
	sig, err := crypto.GPGSign(pkEntity, sigMsg)
	if err != nil {
		return errors.Wrap(err, "failed to sign transaction parameters")
	}

	// Construct the tx line
	txLine := util.MakeTxLine(txFee, txNonce, pkID, sig)

	// Create a blob with 0 byte content which be the subject of our note.
	blobHash, err := repo.CreateBlob("")
	if err != nil {
		return err
	}

	// Next we add the tx blob to the note
	if err = repo.AddEntryToNote(note, blobHash, txLine); err != nil {
		return errors.Wrap(err, "failed to add tx blob")
	}

	fmt.Println("Added signed transaction object to note")

	return nil
}

// getTreeEntries returns all entries in a tree.
func getTreeEntries(repo *Repo, treeHash string) ([]string, error) {
	entries, err := repo.ListTreeObjectsSlice(treeHash, true, true)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// getCommitHistory gets all objects that led up to the given commit, such
// as parent commits, trees and blobs.
// repo: The target repository
// commit: The target commit
// stopCommitHash: A commit hash that when found triggers the end of the search.
func getCommitHistory(repo *Repo, commit *object.Commit, stopCommitHash string) ([]string, error) {
	var hashes []string

	// Stop if commit hash matches the stop hash
	if commit.Hash.String() == stopCommitHash {
		return hashes, nil
	}

	// Add the commit and the tree hash
	hashes = append(hashes, commit.Hash.String())
	hashes = append(hashes, commit.TreeHash.String())

	// Get entries of the tree (blobs and sub-trees)
	entries, err := getTreeEntries(repo, commit.TreeHash.String())
	if err != nil {
		return nil, err
	}
	hashes = append(hashes, entries...)

	// Perform same operation on the parents of the commit
	err = commit.Parents().ForEach(func(parent *object.Commit) error {
		childHashes, err := getCommitHistory(repo, parent, stopCommitHash)
		if err != nil {
			return err
		}
		hashes = append(hashes, childHashes...)
		return nil
	})

	return funk.UniqString(hashes), err
}
