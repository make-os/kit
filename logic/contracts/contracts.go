package contracts

import (
	"github.com/make-os/lobe/logic/contracts/createrepo"
	"github.com/make-os/lobe/logic/contracts/depositproposalfee"
	"github.com/make-os/lobe/logic/contracts/gitpush"
	"github.com/make-os/lobe/logic/contracts/purchaseticket"
	"github.com/make-os/lobe/logic/contracts/registernamespace"
	"github.com/make-os/lobe/logic/contracts/registerpushkey"
	"github.com/make-os/lobe/logic/contracts/registerrepopushkeys"
	"github.com/make-os/lobe/logic/contracts/setdelcommission"
	"github.com/make-os/lobe/logic/contracts/transfercoin"
	"github.com/make-os/lobe/logic/contracts/unbondticket"
	"github.com/make-os/lobe/logic/contracts/updatedelpushkey"
	"github.com/make-os/lobe/logic/contracts/updatenamespacedomains"
	"github.com/make-os/lobe/logic/contracts/updaterepo"
	"github.com/make-os/lobe/logic/contracts/upsertowner"
	"github.com/make-os/lobe/logic/contracts/voteproposal"
	"github.com/make-os/lobe/types/core"
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
