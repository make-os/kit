package validation

import (
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/mr-tron/base58"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/mergerequest"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	crypto2 "gitlab.com/makeos/mosdef/util/crypto"
)

// TxDetailChecker describes a function for checking a transaction detail
type TxDetailChecker func(params *types.TxDetail, keepers core.Keepers, index int) error

// CheckTxDetail performs sanity and consistency checks on a transaction's parameters.
func CheckTxDetail(params *types.TxDetail, keepers core.Keepers, index int) error {
	if err := CheckTxDetailSanity(params, index); err != nil {
		return err
	}
	return CheckTxDetailConsistency(params, keepers, index)
}

// CheckTxDetailSanity performs sanity checks on a transaction's parameters.
// When authScope is true, only fields necessary for authentication are validated.
func CheckTxDetailSanity(params *types.TxDetail, index int) error {

	// Push key is required and must be valid
	if params.PushKeyID == "" {
		return fe(index, "pkID", "push key id is required")
	}
	if !crypto2.IsValidPushAddr(params.PushKeyID) {
		return fe(index, "pkID", "push key id is not valid")
	}

	// Nonce must be set
	if params.Nonce == 0 {
		return fe(index, "nonce", "nonce is required")
	}

	// Fee must be set
	if params.Fee.Empty() {
		return fe(index, "fee", "fee is required")
	}

	// Value field not expected for non-merge request reference
	if !params.Value.Empty() && !plumbing.IsMergeRequestReference(params.Reference) {
		return fe(index, "value", "field not expected")
	}

	// Fee must be numeric
	if !govalidator.IsFloat(params.Fee.String()) {
		return fe(index, "fee", "fee must be numeric")
	}

	// Signature format must be valid
	if _, err := base58.Decode(params.Signature); err != nil {
		return fe(index, "sig", "signature format is not valid")
	}

	// Merge proposal, if set, must be numeric and have 8 bytes length max.
	if params.MergeProposalID != "" {
		return CheckMergeProposalID(params.MergeProposalID, index)
	}

	return nil
}

// CheckTxDetailConsistency performs consistency checks on a transaction's parameters.
func CheckTxDetailConsistency(params *types.TxDetail, keepers core.Keepers, index int) error {

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
			return fe(index, "namespace", msg)
		}
	}

	// Ensure push key scope grants access to the destination repo namespace and repo name.
	if len(pushKey.Scopes) > 0 && IsBlockedByScope(pushKey.Scopes, params, paramNS) {
		msg := fmt.Sprintf("push key (%s) not permitted due to scope limitation", params.PushKeyID)
		return fe(index, "repo|namespace", msg)
	}

	// Ensure the nonce is a future nonce (> current nonce of pusher's account)
	if keyOwner := keepers.AccountKeeper().Get(pushKey.Address); params.Nonce <= keyOwner.Nonce.UInt64() {
		msg := fmt.Sprintf("nonce (%d) must be greater than current key owner nonce (%d)", params.Nonce,
			keyOwner.Nonce)
		return fe(index, "nonce", msg)
	}

	// When merge proposal ID is set, check if merge proposal exist and
	// whether it was created by the owner of the push key
	if params.MergeProposalID != "" {
		repoState := keepers.RepoKeeper().Get(params.RepoName)
		mp := repoState.Proposals.Get(mergerequest.MakeMergeRequestProposalID(params.MergeProposalID))
		if mp == nil {
			return fe(index, "mergeID", "merge proposal not found")
		}
		if mp.Action != txns.MergeRequestProposalAction {
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
