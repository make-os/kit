package contracts

import (
	"gitlab.com/makeos/mosdef/logic/contracts/createrepo"
	"gitlab.com/makeos/mosdef/logic/contracts/depositproposalfee"
	"gitlab.com/makeos/mosdef/logic/contracts/gitpush"
	"gitlab.com/makeos/mosdef/logic/contracts/purchaseticket"
	"gitlab.com/makeos/mosdef/logic/contracts/registernamespace"
	"gitlab.com/makeos/mosdef/logic/contracts/registerpushkey"
	"gitlab.com/makeos/mosdef/logic/contracts/registerrepopushkeys"
	"gitlab.com/makeos/mosdef/logic/contracts/setdelcommission"
	"gitlab.com/makeos/mosdef/logic/contracts/transfercoin"
	"gitlab.com/makeos/mosdef/logic/contracts/unbondticket"
	"gitlab.com/makeos/mosdef/logic/contracts/updatedelpushkey"
	"gitlab.com/makeos/mosdef/logic/contracts/updatenamespacedomains"
	"gitlab.com/makeos/mosdef/logic/contracts/updaterepo"
	"gitlab.com/makeos/mosdef/logic/contracts/upsertowner"
	"gitlab.com/makeos/mosdef/logic/contracts/voteproposal"
	"gitlab.com/makeos/mosdef/types/core"
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
