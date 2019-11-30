package mockutil

import (
	"github.com/golang/mock/gomock"
	drandmocks "github.com/makeos/mosdef/crypto/rand/mocks"
	"github.com/makeos/mosdef/types/mocks"
)

// MockObjects contains mocks for various structs
type MockObjects struct {
	Logic           *mocks.MockLogic
	AtomicLogic     *mocks.MockAtomicLogic
	Sys             *mocks.MockSysLogic
	Tx              *mocks.MockTxLogic
	Validator       *mocks.MockValidatorLogic
	SysKeeper       *mocks.MockSystemKeeper
	AccountKeeper   *mocks.MockAccountKeeper
	ValidatorKeeper *mocks.MockValidatorKeeper
	RepoKeeper      *mocks.MockRepoKeeper
	TxKeeper        *mocks.MockTxKeeper
	TicketManager   *mocks.MockTicketManager
	StateTree       *mocks.MockTree
	Drand           *drandmocks.MockDRander
	// RepoManager     *repomocks.MockRepositoryManager
}

// MockLogic returns logic package mocks
func MockLogic(ctrl *gomock.Controller) *MockObjects {
	mo := &MockObjects{}
	mo.Logic = mocks.NewMockLogic(ctrl)
	mo.AtomicLogic = mocks.NewMockAtomicLogic(ctrl)

	mo.Sys = mocks.NewMockSysLogic(ctrl)
	mo.Tx = mocks.NewMockTxLogic(ctrl)
	mo.Validator = mocks.NewMockValidatorLogic(ctrl)
	mo.SysKeeper = mocks.NewMockSystemKeeper(ctrl)
	mo.AccountKeeper = mocks.NewMockAccountKeeper(ctrl)
	mo.RepoKeeper = mocks.NewMockRepoKeeper(ctrl)
	mo.ValidatorKeeper = mocks.NewMockValidatorKeeper(ctrl)
	mo.TxKeeper = mocks.NewMockTxKeeper(ctrl)
	mo.Logic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.TicketManager = mocks.NewMockTicketManager(ctrl)
	mo.StateTree = mocks.NewMockTree(ctrl)
	mo.Drand = drandmocks.NewMockDRander(ctrl)
	// mo.RepoManager = repomocks.NewMockRepositoryManager(ctrl)

	mo.Logic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.Logic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.Logic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.Logic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.Logic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.Logic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.Logic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.Logic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.Logic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.Logic.EXPECT().GetDRand().Return(mo.Drand).MinTimes(0)
	// mo.Logic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.AtomicLogic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.AtomicLogic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.AtomicLogic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetDRand().Return(mo.Drand).MinTimes(0)
	// mo.AtomicLogic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)

	return mo
}
