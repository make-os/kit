package mergecmd

import (
	"fmt"
	"path"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// CreateAndSendMergeRequestCmd creates merge request proposal
// and sends it to the network
// cfg: App config object
// keyAddrOrIdx: Address or index of key
// passphrase: Passphrase of the key
func CreateAndSendMergeRequestCmd(
	cfg *config.AppConfig,
	keyAddrOrIdx,
	passphrase,
	repoName,
	proposalID,
	baseBranch,
	baseHash,
	targetBranch,
	targetHash,
	fee,
	nextNonce string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signer key
	am := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
	unlocked, err := am.UIUnlockKey(keyAddrOrIdx, passphrase)
	if err != nil {
		return errors.Wrap(err, "unable to unlock")
	}
	fmt.Println("")

	// Determine the next nonce, if unset from flag
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfAccount(unlocked.GetAddress(), rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Create the merge request transaction
	data := map[string]interface{}{
		"name":         repoName,
		"id":           proposalID,
		"type":         core.TxTypeRepoProposalMergeRequest,
		"fee":          fee,
		"senderPubKey": unlocked.GetKey().PubKey().Base58(),
		"nonce":        nextNonce,
		"base":         baseBranch,
		"baseHash":     baseHash,
		"target":       targetBranch,
		"targetHash":   targetHash,
		"timestamp":    time.Now().Unix(),
	}

	// Attempt to load the transaction from map to its native object.
	// If we successfully create its native object, we are sure that the server will
	// be able to do it without error. Also use the object `Sign` method to create
	// a signature and set it on the map
	o := core.NewBareRepoProposalMergeRequest()
	if err := o.FromMap(data); err != nil {
		return errors.Wrap(err, "invalid transaction data")
	}
	sig, err := o.Sign(unlocked.GetKey().PrivKey().Base58())
	if err != nil {
		return errors.Wrap(err, "failed to sign transaction")
	}
	data["sig"] = util.ToHex(sig)

	txHash, err := api.SendTxPayload(data, rpcClient, remoteClients)
	if err != nil {
		return err
	}

	fmt.Println(color.GreenString("Success! Merge proposal sent."))
	fmt.Println("Hash:", txHash)

	return nil
}
