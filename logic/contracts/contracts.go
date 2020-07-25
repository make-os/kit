package contracts

import (
	"gitlab.com/makeos/lobe/logic/contracts/createrepo"
	"gitlab.com/makeos/lobe/logic/contracts/depositproposalfee"
	"gitlab.com/makeos/lobe/logic/contracts/gitpush"
	"gitlab.com/makeos/lobe/logic/contracts/purchaseticket"
	"gitlab.com/makeos/lobe/logic/contracts/registernamespace"
	"gitlab.com/makeos/lobe/logic/contracts/registerpushkey"
	"gitlab.com/makeos/lobe/logic/contracts/registerrepopushkeys"
	"gitlab.com/makeos/lobe/logic/contracts/setdelcommission"
	"gitlab.com/makeos/lobe/logic/contracts/transfercoin"
	"gitlab.com/makeos/lobe/logic/contracts/unbondticket"
	"gitlab.com/makeos/lobe/logic/contracts/updatedelpushkey"
	"gitlab.com/makeos/lobe/logic/contracts/updatenamespacedomains"
	"gitlab.com/makeos/lobe/logic/contracts/updaterepo"
	"gitlab.com/makeos/lobe/logic/contracts/upsertowner"
	"gitlab.com/makeos/lobe/logic/contracts/voteproposal"
	"gitlab.com/makeos/lobe/types/core"
)

// SystemContracts is a list of all system contracts
var SystemContracts []core.SystemContract

func init() {
	SystemContracts = append(SystemContracts, []core.SystemContract{
		transfercoin.NewContract(),
		purchaseticket.NewContract(),
		unbondticket.NewContract(),
		setdelcommission.NewContract(),
		createrepo.NewContract(),
		registerpushkey.NewContract(),
		updatedelpushkey.NewContract(),
		registernamespace.NewContract(),
		updatenamespacedomains.NewContract(),
		gitpush.NewContract(),
		voteproposal.NewContract(),
		depositproposalfee.NewContract(),
		upsertowner.NewContract(&SystemContracts),
		updaterepo.NewContract(&SystemContracts),
		registerrepopushkeys.NewContract(&SystemContracts),
	}...)
}
