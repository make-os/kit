package common

import (
	"github.com/mr-tron/base58"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/repo/plumbing"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
)

// GetTxDetailsFromNote creates a slice of TxDetail objects from a push note.
// Limit to references specified in targetRefs
func GetTxDetailsFromNote(note *core.PushNote, targetRefs ...string) (details []*types.TxDetail) {
	for _, ref := range note.References {
		if len(targetRefs) > 0 && !funk.ContainsString(targetRefs, ref.Name) {
			continue
		}
		detail := &types.TxDetail{
			RepoName:        note.RepoName,
			RepoNamespace:   note.Namespace,
			Reference:       ref.Name,
			Fee:             ref.Fee,
			Nonce:           note.PusherAcctNonce,
			PushKeyID:       crypto.BytesToPushKeyID(note.PushKeyID),
			Signature:       base58.Encode(ref.PushSig),
			MergeProposalID: ref.MergeProposalID,
		}
		if plumbing.IsNote(detail.Reference) {
			detail.Head = ref.NewHash
		}
		details = append(details, detail)
	}
	return
}
