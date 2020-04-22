package cmd

import (
	"errors"
	"fmt"

	"github.com/asaskevich/govalidator"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
func SignCommitCmd(
	cfg *config.AppConfig,
	message string,
	targetRepo core.BareRepo,
	txFee,
	nextNonce string,
	amendRecent bool,
	mergeID,
	head,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, pushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// Validate merge ID is set.
	// Must be numeric and 8 bytes long
	if mergeID != "" {
		if !govalidator.IsNumeric(mergeID) {
			return fmt.Errorf("merge id must be numeric")
		} else if len(mergeID) > 8 {
			return fmt.Errorf("merge proposal id exceeded 8 bytes limit")
		}
	}

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Get the reference pointed to by HEAD
	if head == "" {
		head, err = targetRepo.Head()
		if err != nil {
			return fmt.Errorf("failed to get HEAD") // :D
		}
	} else {
		if !plumbing2.IsBranch(head) {
			head = plumbing.NewBranchReferenceName(head).String()
		}
	}

	// Gather any transaction options
	options := []string{fmt.Sprintf("reference=%s", head)}
	if mergeID != "" {
		options = append(options, fmt.Sprintf("mergeID=%s", mergeID))
	}

	// Make the transaction parameter object
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = server.SetPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	// Create a new quiet commit if recent commit amendment is not desired
	if !amendRecent {
		if err := targetRepo.CreateAndOrSignQuietCommit(message, pushKeyID); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err := targetRepo.GetRecentCommit()
	if err != nil {
		if err == repo.ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	commit, _ := targetRepo.CommitObject(plumbing.NewHash(hash))
	if message == "" {
		message = commit.Message
	}

	// Update the recent commit message
	if err = targetRepo.UpdateRecentCommitMsg(message, pushKeyID); err != nil {
		return err
	}

	return nil
}
