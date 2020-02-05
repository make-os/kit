package node

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/privval"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/types"

	"github.com/golang/mock/gomock"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/makeos/mosdef/ticket"

	l "github.com/makeos/mosdef/logic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
)

func genFilePV(bz []byte) *privval.FilePV {
	privKey := ed25519.GenPrivKeyFromSecret(bz)
	return &privval.FilePV{
		Key: privval.FilePVKey{
			Address: privKey.PubKey().Address(),
			PubKey:  privKey.PubKey(),
			PrivKey: privKey,
		},
	}
}

var _ = Describe("App", func() {
	var c, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *l.Logic
	var app *App
	var ticketmgr *ticket.Manager
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)
	var mockLogic *testutil.MockObjects

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c, stateTreeDB = testutil.GetDB(cfg)
		logic = l.New(c, stateTreeDB, cfg)
		app = NewApp(cfg, c, logic, ticketmgr)
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".InitChain", func() {

		It("should panic if state is not empty", func() {
			logic.StateTree().Set([]byte("k"), []byte("v"))
			Expect(func() {
				app.InitChain(abcitypes.RequestInitChain{})
			}).To(Panic())
		})

		When("writing initial genesis file fails", func() {
			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return(nil)
				mockLogic.AtomicLogic.EXPECT().WriteGenesisState().Return(fmt.Errorf("bad thing"))
				app.logic = mockLogic.AtomicLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.InitChain(abcitypes.RequestInitChain{})
				}).To(Panic())
			})
		})

		When("validator indexing fails", func() {
			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return(nil)
				mockLogic.AtomicLogic.EXPECT().WriteGenesisState().Return(nil)
				mockLogic.Validator.EXPECT().Index(gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad thing"))
				app.logic = mockLogic.AtomicLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.InitChain(abcitypes.RequestInitChain{})
				}).To(Panic())
			})
		})

		When("initialization succeeds", func() {
			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return(nil).Times(2)
				mockLogic.AtomicLogic.EXPECT().WriteGenesisState().Return(nil)
				mockLogic.StateTree.EXPECT().Version().Return(int64(1))
				mockLogic.Validator.EXPECT().Index(gomock.Any(), gomock.Any()).Return(nil)
				app.logic = mockLogic.AtomicLogic
			})

			It("should return an empty response", func() {
				resp := app.InitChain(abcitypes.RequestInitChain{})
				Expect(resp).To(Equal(abcitypes.ResponseInitChain{}))
			})
		})
	})

	Describe(".updateValidators", func() {
		BeforeEach(func() {
			params.NumBlocksPerEpoch = 5
		})

		When("the provided height is not the block height preceding the last epoch", func() {
			It("should return nil", func() {
				Expect(app.updateValidators(3, nil)).To(BeNil())
			})
		})

		When("an error occurred when fetching top validators", func() {
			BeforeEach(func() {
				mockTicketMgr := mockLogic.TicketManager
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				app.ticketMgr = mockTicketMgr
			})

			It("should return error", func() {
				err := app.updateValidators(4, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})

		When("when no tickets were selected", func() {

			BeforeEach(func() {
				mockTicketMgr := mockLogic.TicketManager
				selected := []*types.SelectedTicket{}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr
			})

			It("should return nil and no validator updates in endblock response", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(4, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(BeEmpty())
			})
		})

		When("when no validator currently exists and two tickets are randomly selected", func() {
			var key = crypto.NewKeyFromIntSeed(1)
			var key2 = crypto.NewKeyFromIntSeed(1)

			BeforeEach(func() {
				mockTicketMgr := mockLogic.TicketManager
				selected := []*types.SelectedTicket{
					{Ticket: &types.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}},
					{Ticket: &types.Ticket{ProposerPubKey: key2.PubKey().MustBytes32()}},
				}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr

				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[util.Bytes32]*types.Validator{}, nil)
				app.logic = mockLogic.AtomicLogic
			})

			It("should add the two validators to the response object", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(4, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
			})
		})

		When("when one validator currently exists and another different validator is selected", func() {
			var existingValKey = crypto.NewKeyFromIntSeed(1)
			var keyOfNewTicket = crypto.NewKeyFromIntSeed(2)

			BeforeEach(func() {
				mockTicketMgr := mockLogic.TicketManager
				selected := []*types.SelectedTicket{
					{Ticket: &types.Ticket{ProposerPubKey: keyOfNewTicket.PubKey().MustBytes32()}},
				}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr

				// Mock the return of the existing validator
				pubKey := existingValKey.PubKey().MustBytes32()
				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[util.Bytes32]*types.Validator{
					pubKey: &types.Validator{PubKey: util.StrToBytes32("pub_key")},
				}, nil)

				app.logic = mockLogic.AtomicLogic
			})

			It("should add existing validator but change power to zero (0)", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(4, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
				Expect(resp.ValidatorUpdates[1].Power).To(Equal(int64(0)))
				pubKeyBz, _ := existingValKey.PubKey().Bytes()
				Expect(resp.ValidatorUpdates[1].PubKey.GetData()).To(Equal(pubKeyBz))
			})

			It("should add new validator and set power to 1", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(4, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
				Expect(resp.ValidatorUpdates[0].Power).To(Equal(int64(1)))
				pubKeyBz, _ := keyOfNewTicket.PubKey().Bytes()
				Expect(resp.ValidatorUpdates[0].PubKey.GetData()).To(Equal(pubKeyBz))
			})
		})

		When("when error occurred when fetching current validators", func() {
			var key = crypto.NewKeyFromIntSeed(1)

			BeforeEach(func() {
				mockTicketMgr := mockLogic.TicketManager
				selected := []*types.SelectedTicket{
					{Ticket: &types.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}},
				}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr

				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				app.logic = mockLogic.AtomicLogic
			})

			It("should return error", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(4, &resp)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})
	})

	Describe(".Info", func() {
		When("getting last block info and error is returned and it is not ErrBlockInfoNotFound", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("something bad"))
				app.logic = mockLogic.AtomicLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Info(abcitypes.RequestInfo{})
				}).To(Panic())
			})
		})

		When("getting last block info and error is returned and it is ErrBlockInfoNotFound", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, keepers.ErrBlockInfoNotFound)
				app.logic = mockLogic.AtomicLogic
			})

			It("should not panic", func() {
				Expect(func() {
					app.Info(abcitypes.RequestInfo{})
				}).ToNot(Panic())
			})
		})
	})

	Describe(".CheckTx", func() {
		When("tx could not be decoded into types.Transaction", func() {
			var res abcitypes.ResponseCheckTx
			BeforeEach(func() {
				res = app.CheckTx(abcitypes.RequestCheckTx{Tx: []byte("invalid bz")})
			})

			It("should return code=types.ErrCodeTxBadEncode and"+
				" log='unable to decode to types.Transaction'", func() {
				Expect(res.Code).To(Equal(types.ErrCodeTxBadEncode))
				Expect(res.Log).To(Equal("unable to decode to types.BaseTx"))
			})
		})

		When("tx is valid", func() {
			var res abcitypes.ResponseCheckTx
			var expectedHash util.Bytes32
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return nil
				}
				tx := types.NewBaseTx(0, 0, sender.Addr(), sender, "10", "1", 1)
				expectedHash = tx.GetHash()
				res = app.CheckTx(abcitypes.RequestCheckTx{Tx: tx.Bytes()})
			})

			It("should return the tx hash", func() {
				Expect(res.Code).To(Equal(uint32(0)))
				Expect(res.GetData()).To(Equal(expectedHash.Bytes()))
			})
		})

		When("tx failed validation", func() {
			var res abcitypes.ResponseCheckTx
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return fmt.Errorf("bad error")
				}
				tx := types.NewBaseTx(0, 0, sender.Addr(), sender, "10", "1", 1)
				res = app.CheckTx(abcitypes.RequestCheckTx{Tx: tx.Bytes()})
			})

			It("should return error", func() {
				Expect(res.Code).To(Equal(types.ErrCodeTxFailedValidation))
				Expect(res.Log).To(Equal(res.Log))
			})
		})
	})

	Describe(".BeginBlock", func() {
		When("current block proposer is the same as the private validator", func() {
			BeforeEach(func() {
				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				req := abcitypes.RequestBeginBlock{}
				req.Header.ProposerAddress = pv.GetAddress().Bytes()

				app.logic = mockLogic.AtomicLogic
				app.BeginBlock(req)
			})

			It("should set `isCurrentBlockProposer` to true", func() {
				Expect(app.isCurrentBlockProposer).To(BeTrue())
			})
		})
	})

	Describe(".DeliverTx", func() {
		When("tx could not be decoded into types.Transaction", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				res = app.DeliverTx(abcitypes.RequestDeliverTx{Tx: []byte("invalid bz")})
			})

			It("should return code=types.ErrCodeTxBadEncode and"+
				" log='unable to decode to types.Transaction'", func() {
				Expect(res.Code).To(Equal(types.ErrCodeTxBadEncode))
				Expect(res.Log).To(Equal("unable to decode to types.BaseTx"))
			})

			Specify("that txIndex is incremented", func() {
				Expect(app.txIndex).To(Equal(1))
			})
		})

		When("tx is invalid", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return fmt.Errorf("validation error")
				}
				tx := types.NewBaseTx(0, 0, sender.Addr(), sender, "10", "1", 1)
				res = app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx.Bytes()})
			})

			It("should return code=types.ErrCodeTxFailedValidation", func() {
				Expect(res.Code).To(Equal(types.ErrCodeTxFailedValidation))
			})
		})

		When("tx type is TxTypeValidatorTicket; max. TxTypeValidatorTicket per "+
			"block is 1; 1 TxTypeValidatorTicket tx has previously been collected", func() {
			var res abcitypes.ResponseDeliverTx

			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				params.MaxValTicketsPerBlock = 1
				app.validatorTickets = append(app.validatorTickets, &ticketInfo{})
				tx := types.NewBaseTx(types.TxTypeValidatorTicket, 0, sender.Addr(), sender, "10", "1", 1)
				res = app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx.Bytes()})
			})

			It("should return code=types.ErrCodeMaxValTxTypeReached and"+
				" log='failed to execute tx: validator ticket capacity reached'", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeMaxTxTypeReached)))
				Expect(res.Log).To(Equal("failed to execute tx: validator ticket capacity reached"))
			})
		})

		When("tx type is TxTypeValidatorTicket and is successfully executed", func() {
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewBaseTx(types.TxTypeValidatorTicket, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().ExecTx(tx, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				app.DeliverTx(req)
			})

			It("should return cache the validator ticket tx", func() {
				Expect(app.validatorTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeStorerTicket and response code=0", func() {
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewBaseTx(types.TxTypeStorerTicket, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().ExecTx(tx, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should return cache the storer ticket tx", func() {
				Expect(app.storerTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeUnbondStorerTicket and response code=0", func() {
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewBaseTx(types.TxTypeUnbondStorerTicket, 0, sender.Addr(), sender, "10", "1", 1)
				tx.(*types.TxTicketUnbond).TicketHash = util.StrToBytes32("tid")
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().ExecTx(tx, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should return cache the unbond storer ticket tx", func() {
				Expect(app.unbondStorerReqs).To(HaveLen(1))
			})
		})
	})

	Describe(".Commit", func() {

		When("error occurred when saving validators", func() {

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.curWorkingBlock.Height = 10
				app.heightToSaveNewValidators = 10
				mockLogic.ValidatorKeeper.EXPECT().Index(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
				mockLogic.AtomicLogic.EXPECT().Discard().Return()
				app.logic = mockLogic.AtomicLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when trying to save non-indexed tx", func() {

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.heightToSaveNewValidators = 100
				mockLogic.TxKeeper.EXPECT().Index(gomock.Any()).Return(fmt.Errorf("error"))
				mockLogic.AtomicLogic.EXPECT().Discard().Return()
				mockLogic.AtomicLogic.EXPECT().TxKeeper().Return(mockLogic.TxKeeper).AnyTimes()
				app.logic = mockLogic.AtomicLogic
				app.unIndexedTxs = append(app.unIndexedTxs, types.NewBareTxCoinTransfer())
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("no error occurred", func() {
			appHash := []byte("working_hash")

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash")).Times(1)
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.heightToSaveNewValidators = 100
				mockLogic.AtomicLogic.EXPECT().Commit().Return(nil)
				app.logic = mockLogic.AtomicLogic
			})

			It("should return expected app hash", func() {
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})
		})

		When("there is an unbond storer request; should attempt to update the ticket decay height", func() {
			appHash := []byte("app_hash")

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("app_hash")).Times(1)
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.heightToSaveNewValidators = 100
				app.unbondStorerReqs = append(app.unbondStorerReqs, util.StrToBytes32("ticket_hash"))
				mockLogic.TicketManager.EXPECT().UpdateDecayBy(util.StrToBytes32("ticket_hash"), uint64(app.curWorkingBlock.Height))
				mockLogic.AtomicLogic.EXPECT().Commit().Return(nil)
				app.logic = mockLogic.AtomicLogic

			})

			It("should return expected app hash", func() {
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})
		})
	})
})
