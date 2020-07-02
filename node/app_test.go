package node

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/privval"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/logic/contracts/mergerequest"
	"gitlab.com/makeos/mosdef/params"
	pushtypes "gitlab.com/makeos/mosdef/remote/push/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"

	"gitlab.com/makeos/mosdef/logic/keepers"
	"gitlab.com/makeos/mosdef/types"

	"github.com/golang/mock/gomock"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"gitlab.com/makeos/mosdef/ticket"

	l "gitlab.com/makeos/mosdef/logic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
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

		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
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
				var selected []*tickettypes.SelectedTicket
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
				selected := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}},
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key2.PubKey().MustBytes32()}},
				}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr

				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[util.Bytes32]*core.Validator{}, nil)
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
				selected := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: keyOfNewTicket.PubKey().MustBytes32()}},
				}
				mockTicketMgr.EXPECT().GetTopValidators(gomock.Any()).Return(selected, nil)
				app.ticketMgr = mockTicketMgr

				// Mock the return of the existing validator
				pubKey := existingValKey.PubKey().MustBytes32()
				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[util.Bytes32]*core.Validator{
					pubKey: {PubKey: util.StrToBytes32("pub_key")},
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
				selected := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}},
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

		When("unable to get last block information", func() {
			var appHash = []byte("app_hash")
			var height = int64(100)

			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{
					AppHash: []byte("app_hash"),
					Height:  util.Int64(height),
				}, nil)
				app.logic = mockLogic.AtomicLogic
			})

			It("should not panic", func() {
				info := app.Info(abcitypes.RequestInfo{})
				Expect(info.LastBlockAppHash).To(Equal(appHash))
				Expect(info.LastBlockHeight).To(Equal(height))
			})
		})

		When("last block information is returned", func() {
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
			var expectedHash util.HexBytes
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error {
					return nil
				}
				tx := txns.NewCoinTransferTx(0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error {
					return fmt.Errorf("bad error")
				}
				tx := txns.NewCoinTransferTx(0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error {
					return fmt.Errorf("validation error")
				}
				tx := txns.NewCoinTransferTx(0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error { return nil }
				params.MaxValTicketsPerBlock = 1
				app.unIdxValidatorTickets = append(app.unIdxValidatorTickets, &ticketInfo{})
				tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
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
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error { return nil }
				tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}

				mockLogic.AtomicLogic.EXPECT().ExecTx(gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				app.DeliverTx(req)
			})

			It("should return cache the validator ticket tx", func() {
				Expect(app.unIdxValidatorTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeHostTicket and response code=0", func() {
			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error { return nil }
				tx := txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.AtomicLogic.EXPECT().ExecTx(gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should cache the host ticket tx", func() {
				Expect(app.unIdxHostTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeUnbondHostTicket and response code=0", func() {

			BeforeEach(func() {
				app.validateTx = func(tx types.BaseTx, i int, logic core.Logic) error { return nil }
				app.curBlock.Height = 10
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeUnbondHostTicket)
				tx.TicketHash = util.StrToHexBytes("tid")
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.AtomicLogic.EXPECT().ExecTx(gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should return cache the unbond host ticket tx", func() {
				Expect(app.unbondHostReqs).To(HaveLen(1))
			})
		})
	})

	Describe(".postExecChecks", func() {
		When("tx is TxRepoCreate", func() {
			var tx *txns.TxRepoCreate

			BeforeEach(func() {
				tx = txns.NewBareTxRepoCreate()
				tx.Name = "repo1"
				resp := &abcitypes.ResponseDeliverTx{}
				app.postExecChecks(tx, resp)
			})

			It("should add repo name to new repo index", func() {
				Expect(app.newRepos).To(HaveLen(1))
				Expect(app.newRepos).To(ContainElement(tx.Name))
			})

			It("should add tx to un-indexed cache", func() {
				Expect(app.unIdxTxs).To(ContainElement(tx))
			})
		})

		When("tx is TxRepoProposalVote", func() {
			var tx *txns.TxRepoProposalVote

			BeforeEach(func() {
				tx = txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				resp := &abcitypes.ResponseDeliverTx{}
				app.postExecChecks(tx, resp)
			})

			It("should add repo name to new repo index", func() {
				Expect(app.unIdxRepoPropVotes).To(HaveLen(1))
				Expect(app.unIdxRepoPropVotes).To(ContainElement(tx))
			})

			It("should add tx to un-indexed cache", func() {
				Expect(app.unIdxTxs).To(ContainElement(tx))
			})
		})

		When("tx is TxPush with a reference with merge proposal id", func() {
			var tx *txns.TxPush

			BeforeEach(func() {
				tx = txns.NewBareTxPush()
				tx.Note.(*pushtypes.Note).RepoName = "repo1"
				tx.Note.(*pushtypes.Note).References = []*pushtypes.PushedReference{
					{MergeProposalID: "0001"},
				}
				resp := &abcitypes.ResponseDeliverTx{}
				app.postExecChecks(tx, resp)
			})

			It("should add repo and proposal id to closable proposals", func() {
				Expect(app.unIdxClosedMergeProposal).To(HaveLen(1))
				Expect(app.unIdxClosedMergeProposal).To(ContainElement(&mergeProposalInfo{"repo1", mergerequest.MakeMergeRequestProposalID("0001")}))
			})

			It("should add tx to un-indexed cache", func() {
				Expect(app.unIdxTxs).To(ContainElement(tx))
			})
		})
	})

	Describe(".Commit", func() {

		When("error occurred when saving validators", func() {

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.curBlock.Height = 10
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
				app.unIdxTxs = append(app.unIdxTxs, txns.NewBareTxCoinTransfer())
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

		When("there is an unbond host request; should attempt to update the ticket decay height", func() {
			appHash := []byte("app_hash")

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("app_hash")).Times(1)
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.heightToSaveNewValidators = 100
				app.unbondHostReqs = append(app.unbondHostReqs, util.StrToHexBytes("ticket_hash"))
				mockLogic.TicketManager.EXPECT().UpdateDecayBy(util.StrToHexBytes("ticket_hash"), uint64(app.curBlock.Height))
				mockLogic.AtomicLogic.EXPECT().Commit().Return(nil)
				app.logic = mockLogic.AtomicLogic

			})

			It("should return expected app hash", func() {
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})
		})

		When("there are un-indexed validator or host tickets", func() {

			var valTicketInfo *ticketInfo
			var hostTicketInfo *ticketInfo
			var valTicketTx, hostTicketTx *txns.TxTicketPurchase

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("app_hash")).Times(1)
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				valTicketTx = txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				hostTicketTx = txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
				valTicketInfo = &ticketInfo{Tx: valTicketTx, index: 1}
				hostTicketInfo = &ticketInfo{Tx: hostTicketTx, index: 2}
				app.unIdxValidatorTickets = append(app.unIdxValidatorTickets, valTicketInfo)
				app.unIdxHostTickets = append(app.unIdxHostTickets, hostTicketInfo)

				mockLogic.ValidatorKeeper.EXPECT().Index(gomock.Any(), gomock.Any()).Times(1)
				mockLogic.AtomicLogic.EXPECT().Commit().Times(1)

				app.logic = mockLogic.AtomicLogic
				app.ticketMgr = mockLogic.TicketManager
			})

			It("should attempt to index all collected tickets", func() {
				mockLogic.TicketManager.EXPECT().Index(valTicketInfo.Tx, uint64(0), valTicketInfo.index).Return(nil)
				mockLogic.TicketManager.EXPECT().Index(hostTicketInfo.Tx, uint64(0), hostTicketInfo.index).Return(nil)
				app.Commit()
			})
		})

		When("there are unclosed merge proposals", func() {
			var mergePropInfo *mergeProposalInfo

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("app_hash")).Times(1)
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				mergePropInfo = &mergeProposalInfo{repo: "repo1", proposalID: "0001"}
				app.unIdxClosedMergeProposal = append(app.unIdxClosedMergeProposal, mergePropInfo)

				mockLogic.ValidatorKeeper.EXPECT().Index(gomock.Any(), gomock.Any()).Times(1)
				mockLogic.AtomicLogic.EXPECT().Commit().Times(1)

				app.logic = mockLogic.AtomicLogic
				app.ticketMgr = mockLogic.TicketManager
			})

			It("should attempt to mark the proposal as closed", func() {
				mockLogic.RepoKeeper.EXPECT().
					MarkProposalAsClosed(mergePropInfo.repo, mergePropInfo.proposalID).Return(nil)
				app.Commit()
			})
		})
	})
})
