package logic

import (
	"crypto/rsa"

	"gitlab.com/makeos/mosdef/types/state"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// execRegisterGPGKey associates a GPG key to an account
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
func (t *Transaction) execRegisterGPGKey(
	senderPubKey util.Bytes32,
	gpgPublicKey string,
	scopes []string,
	feeCap,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())

	// Create a new GPGPubKey
	key := state.BareGPGPubKey()
	key.PubKey = gpgPublicKey
	key.Address = spk.Addr()
	key.Scopes = scopes
	key.FeeCap = feeCap

	// Store the new public key
	entity, _ := crypto.PGPEntityFromPubKey(gpgPublicKey)
	gpgID := util.CreateGPGIDFromRSA(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	t.logic.GPGPubKeyKeeper().Update(gpgID, key)

	// Deduct fee and update account
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}

// execUpDelGPGKey updates or deletes a registered gpg public key
//
// ARGS:
// senderPubKey: The account public key of the sender.
// gpgID: The gpg public key ID
// addScopes: A list of scopes to add
// removeScopes: A list of indices pointing to scopes to be deleted.
// deleteKey: Indicates that the gpg public key should be deleted.
// feeCap: The amount of fee the gpg key can spend.
// fee: The fee paid by the sender
// chainHeight: The chain height to limit query to
//
// CONTRACT:
// - Expect sender public key to be valid.
// - Expect the gpg key to exist.
// - Expect indices in removeScopes are within range of key scopes
func (t *Transaction) execUpDelGPGKey(
	senderPubKey util.Bytes32,
	gpgID string,
	addScopes []string,
	removeScopes []int,
	deleteKey bool,
	feeCap,
	fee util.String,
	chainHeight uint64) error {

	gpgKeeper := t.logic.GPGPubKeyKeeper()
	key := gpgKeeper.Get(gpgID)

	// If delete is requested, delete immediately and return.
	if deleteKey {
		gpgKeeper.Remove(gpgID)
		goto debit_fee
	}

	// If there are scopes to remove, remove them
	for c, i := range removeScopes {
		i = i - c
		key.Scopes = key.Scopes[:i+copy(key.Scopes[i:], key.Scopes[i+1:])]
	}

	// If there are scopes to add, add them
	for _, s := range addScopes {
		key.Scopes = append(key.Scopes, s)
	}

	// Set fee cap if set
	if feeCap != "" {
		key.FeeCap = feeCap
	}

	// Update GPG key
	gpgKeeper.Update(gpgID, key)

debit_fee:

	// Deduct network fee + proposal fee from sender
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}
