package repo

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"

	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
)

// AmendRecentTxLine attempts to add or amend transaction argument to
// the recent commit. If no transaction line exist, it will add a new
// one populated with the provided arguments.
// Transaction line allows the user to set transaction arguments such as
// fee, public key, etc. A txline has the format tx: fee=10, pk=ad1..xyz, nonce=1
func AmendRecentTxLine(gitBinDir, txFee, txNonce, signingKey string) error {

	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	// Since we expect the working directory to be a git working tree,
	// we need to get a repo instance
	repo, err := git.PlainOpen(wd)
	if err != nil {
		return errors.Wrap(err, "failed to open repository")
	} else if repoCfg, _ := repo.Config(); repoCfg.Core.IsBare {
		return errors.New("cannot amend a bare repository")
	}

	gitOp := NewGitOps(gitBinDir, wd)

	// Get the signing key id from the git config
	if signingKey == "" {
		signingKey = gitOp.GetConfig("user.signingKey")
	}
	if signingKey == "" {
		return errors.New("signing key was not set or provided")
	}

	// Get recent commit hash of the current branch
	hash, err := gitOp.GetRecentCommit()
	if err != nil {
		if err == ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}
	
	
	commit, _ := repo.CommitObject(plumbing.NewHash(hash))
	msg := util.RemoveTxLine(commit.Message)
	
	// Get the public key of the signing key
	pkEntity, err := crypto.GetGPGPublicKey(signingKey, gitOp.GetConfig("gpg.program"))
	if err != nil {
		return errors.Wrap(err, "failed to get gpg public key")
	}
	
	// Get the public key network ID
	pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	
	// TODO:If nonce is not provided, get nonce from a --source (remote node or local
	// node)
	
	// Construct the tx line and append to the current message
	txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", txFee, txNonce, pkID)
	msg += "\n\n" + txLine

	// Update the recent commit message
	if err = gitOp.UpdateRecentCommitMsg(msg); err != nil {
		return err
	}

	return nil
}
