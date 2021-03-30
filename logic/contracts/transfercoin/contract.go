package transfercoin

import (
	"fmt"

	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/make-os/kit/util/identifier"
)

var (
	fe = errors.FieldError
)

// Contract implements core.SystemContract. It is a system contract for handling credit and debit
// operation from between user accounts and/or repo accounts.
type Contract struct {
	core.Keepers
	tx          *txns.TxCoinTransfer
	chainHeight uint64
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeCoinTransfer
}

// Init initialize the contract
func (c *Contract) Init(logic core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = logic
	c.tx = tx.(*txns.TxCoinTransfer)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	var recvAcct core.BalanceAccount
	recipientAddr := c.tx.To
	recvAddr := recipientAddr
	acctKeeper := c.AccountKeeper()
	repoKeeper := c.RepoKeeper()
	senderPubKey := c.tx.GetSenderPubKey().ToBytes32()
	value := c.tx.Value
	fee := c.tx.Fee

	// If the recipient address is a user namespace, we need to resolve it
	// to the target address which is expected to be a native namespace.
	if recvAddr.IsUserNamespace() {
		target, err := c.NamespaceKeeper().GetTarget(recvAddr.String())
		if err != nil {
			return err
		}
		recvAddr = identifier.Address(target)
	}

	// If the recipient address is a full native namespace (e.g r/repo or a/repo),
	// we need to get the balance account corresponding to the namespace target.
	if recvAddr.IsFullNativeNamespace() {
		addrVal := identifier.GetDomain(recvAddr.String())
		recipientAddr = identifier.Address(addrVal)
		if identifier.IsWholeNativeRepoURI(recvAddr.String()) {
			recvAcct = repoKeeper.Get(addrVal)
		} else {
			recvAcct = acctKeeper.Get(identifier.Address(addrVal))
		}
	}

	// If the recipient address is a bech32 user address, get the account object
	// corresponding to the address.
	if recvAddr.IsUserAddress() {
		recvAcct = acctKeeper.Get(recipientAddr)
	}

	// Return error if at this point we don't have a recipient account object,
	if recvAcct == nil {
		return fmt.Errorf("recipient account not found")
	}

	// Get the sender's account and balance
	spk, _ := ed25519.PubKeyFromBytes(senderPubKey.Bytes())
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)
	senderBal := senderAcct.Balance.Decimal()

	// When the sender is also the recipient, use the sender account as recipient account
	if sender.Equal(recipientAddr) {
		recvAcct = senderAcct
	}

	// Calculate the spend amount and deduct the spend amount from
	// the sender's account, then increment sender's nonce
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up and update the sender's account
	senderAcct.Clean(c.chainHeight)
	acctKeeper.Update(sender, senderAcct)

	// Register the transaction value to the recipient balance
	recipientBal := recvAcct.GetBalance().Decimal()
	recvAcct.SetBalance(recipientBal.Add(value.Decimal()).String())

	// Clean up the recipient object
	recvAcct.Clean(c.chainHeight)

	// Save the new state of the object
	switch o := recvAcct.(type) {
	case *state.Account:
		acctKeeper.Update(recipientAddr, o)
	case *state.Repository:
		repoKeeper.Update(recipientAddr.String(), o)
	}

	return nil
}

// DryExec checks whether the given sender can execute the transaction.
//
// sender can be an address, identifier.Address or *ed25519.PubKey
//
// allowNonceGap allows nonce to have a number greater than the current account
// by more than 1.
func (c *Contract) DryExec(sender interface{}, allowNonceGap bool) error {

	senderAddr := ""
	switch o := sender.(type) {
	case *ed25519.PubKey:
		senderAddr = o.Addr().String()
	case string:
		senderAddr = o
	case identifier.Address:
		senderAddr = o.String()
	default:
		return fmt.Errorf("unexpected address type")
	}

	// Get sender and recipient accounts
	acctKeeper := c.AccountKeeper()
	senderAcct := acctKeeper.Get(identifier.Address(senderAddr))

	// When allowNonceGap is true, let nonce be anything greater than current nonce.
	// The mode is required in places like the mempool where strict monotonically
	// increasing nonce number is not required.
	if allowNonceGap {
		if c.tx.Nonce <= senderAcct.Nonce.UInt64() {
			return fe("nonce", fmt.Sprintf("tx has an invalid nonce (%d); expected a "+
				"nonce that is greater than the current account nonce (%d)", c.tx.Nonce, senderAcct.Nonce.UInt64()))
		}
	} else {
		// Ensure the transaction nonce is the next expected nonce
		expectedNonce := senderAcct.Nonce + 1
		if expectedNonce.UInt64() != c.tx.Nonce {
			return fe("nonce", fmt.Sprintf("tx has an invalid nonce (%d); expected (%d)", c.tx.Nonce, expectedNonce))
		}
	}

	// Ensure sender has enough spendable balance to pay transfer value + fee
	field := "value+fee"
	spendAmt := c.tx.Value.Decimal().Add(c.tx.Fee.Decimal())
	senderBal := senderAcct.GetAvailableBalance(c.chainHeight).Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fe(field, fmt.Sprintf("sender's spendable account balance is insufficient"))
	}

	return nil
}
