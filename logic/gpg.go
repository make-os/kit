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
// publicKey: The gpg public key
// senderPubKey: The account public key of the sender.
// fee: The fee paid by the sender
// chainHeight: The next chain height
//
// CONTRACT:
// - Sender's public key must be valid public key
// - The gpg public key must be valid
func (t *Transaction) execAddGPGKey(
	gpgPublicKey,
	senderPubKey string,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey)
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
	senderAcct := acctKeeper.GetAccount(sender, int64(chainHeight))
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
