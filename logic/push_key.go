package logic

import (
	"gitlab.com/makeos/mosdef/types/state"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// execRegisterPushKey registers a Push key
//
// ARGS:
// senderPubKey: The account public key of the sender.
// pushPubKey: The push public key
// fee: The fee paid by the sender
// chainHeight: The chain height to limit query to
//
// CONTRACT:
// - Sender's public key must be valid public key
// - The push key public key must be valid
func (t *Transaction) execRegisterPushKey(
	senderPubKey util.Bytes32,
	pushPubKey crypto.PublicKey,
	scopes []string,
	feeCap,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())

	// Create a new PushKey
	key := state.BarePushKey()
	key.PubKey = pushPubKey
	key.Address = spk.Addr()
	key.Scopes = scopes
	key.FeeCap = feeCap

	// Store the new public key
	pushKeyID := crypto.CreatePushKeyID(pushPubKey)
	t.logic.PushKeyKeeper().Update(pushKeyID, key)

	// Deduct fee and update account
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}

// execUpDelPushKey updates or deletes a registered push key
//
// ARGS:
// senderPubKey: The public key of the sender.
// pushKeyID: The push key ID
// addScopes: A list of scopes to add
// removeScopes: A list of indices pointing to scopes to be deleted.
// deleteKey: Indicates that the push key should be deleted.
// feeCap: The amount of fee the push key can spend.
// fee: The fee paid by the sender
// chainHeight: The chain height to limit query to
//
// CONTRACT:
// - Expect sender public key to be valid.
// - Expect the push key to exist.
// - Expect indices in removeScopes are within range of key scopes
func (t *Transaction) execUpDelPushKey(
	senderPubKey util.Bytes32,
	pushKeyID string,
	addScopes []string,
	removeScopes []int,
	deleteKey bool,
	feeCap,
	fee util.String,
	chainHeight uint64) error {

	pushKeyKeeper := t.logic.PushKeyKeeper()
	key := pushKeyKeeper.Get(pushKeyID)

	// If delete is requested, delete immediately and return.
	if deleteKey {
		pushKeyKeeper.Remove(pushKeyID)
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

	// Update push key
	pushKeyKeeper.Update(pushKeyID, key)

debit_fee:

	// Deduct network fee + proposal fee from sender
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}
