package logic

import (
	"crypto/rsa"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// execAddGPGKey associates a GPG key to an account
//
// ARGS:
// senderPubKey: The account public key of the sender.
// publicKey: The gpg public key
// fee: The fee paid by the sender
// chainHeight: The chain height to limit query to
//
// CONTRACT:
// - Sender's public key must be valid public key
// - The gpg public key must be valid
func (t *Transaction) execAddGPGKey(
	senderPubKey util.Bytes32,
	gpgPublicKey string,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	acctKeeper := t.logic.AccountKeeper()

	// Create a new GPGPubKey
	gpgPubKey := types.BareGPGPubKey()
	gpgPubKey.PubKey = gpgPublicKey
	gpgPubKey.Address = spk.Addr()

	// Store the new public key
	entity, _ := crypto.PGPEntityFromPubKey(gpgPublicKey)
	pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	if err := t.logic.GPGPubKeyKeeper().Update(pkID, gpgPubKey); err != nil {
		return err
	}

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
