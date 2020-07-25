package contracts

import (
	"github.com/themakeos/lobe/logic/contracts/createrepo"
	"github.com/themakeos/lobe/logic/contracts/depositproposalfee"
	"github.com/themakeos/lobe/logic/contracts/gitpush"
	"github.com/themakeos/lobe/logic/contracts/purchaseticket"
	"github.com/themakeos/lobe/logic/contracts/registernamespace"
	"github.com/themakeos/lobe/logic/contracts/registerpushkey"
	"github.com/themakeos/lobe/logic/contracts/registerrepopushkeys"
	"github.com/themakeos/lobe/logic/contracts/setdelcommission"
	"github.com/themakeos/lobe/logic/contracts/transfercoin"
	"github.com/themakeos/lobe/logic/contracts/unbondticket"
	"github.com/themakeos/lobe/logic/contracts/updatedelpushkey"
	"github.com/themakeos/lobe/logic/contracts/updatenamespacedomains"
	"github.com/themakeos/lobe/logic/contracts/updaterepo"
	"github.com/themakeos/lobe/logic/contracts/upsertowner"
	"github.com/themakeos/lobe/logic/contracts/voteproposal"
	"github.com/themakeos/lobe/types/core"
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
