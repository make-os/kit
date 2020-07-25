package transfercoin

import (
	"fmt"

	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/core"
	"gitlab.com/makeos/lobe/types/state"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
	"gitlab.com/makeos/lobe/util/identifier"
)

var (
	fe = util.FieldError
)

// CoinTransferContract is a system contract for handling credit and debit
// operation from between user accounts and/or repo accounts.
// CoinTransferContract implements SystemContract.
type CoinTransferContract struct {
	core.Logic
	tx          *txns.TxCoinTransfer
	chainHeight uint64
}

// NewContract creates a new instance of CoinTransferContract
func NewContract() *CoinTransferContract {
	return &CoinTransferContract{}
}

func (c *CoinTransferContract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeCoinTransfer
}

// Init initialize the contract
func (c *CoinTransferContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Logic = logic
	c.tx = tx.(*txns.TxCoinTransfer)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *CoinTransferContract) Exec() error {

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
		addrVal := identifier.GetNativeNamespaceTarget(recvAddr.String())
		recipientAddr = identifier.Address(addrVal)
		if identifier.IsFullNativeNamespaceRepo(recvAddr.String()) {
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
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
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

// DryExec checks whether the given sender can execute the transaction
func (c *CoinTransferContract) DryExec(sender interface{}) error {

	senderAddr := ""
	switch o := sender.(type) {
	case *crypto.PubKey:
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

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce.UInt64() != c.tx.Nonce {
		return fe("nonce", fmt.Sprintf("tx has invalid nonce (%d); expected (%d)", c.tx.Nonce, expectedNonce))
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
