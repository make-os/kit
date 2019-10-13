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
	"github.com/makeos/mosdef/types/mocks"

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
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var logic *l.Logic
	var app *App
	var ticketmgr *ticket.Manager
	var ctrl *gomock.Controller
	var sender = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		logic = l.New(c, cfg)
		app = NewApp(cfg, c, logic, ticketmgr)
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
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
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return(nil)
				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockLogic.EXPECT().WriteGenesisState().Return(fmt.Errorf("bad thing"))
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.InitChain(abcitypes.RequestInitChain{})
				}).To(Panic())
			})
		})

		When("validator indexing fails", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return(nil)
				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockLogic.EXPECT().WriteGenesisState().Return(nil)
				mockValidator := mocks.NewMockValidatorLogic(ctrl)
				mockValidator.EXPECT().Index(gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad thing"))
				mockLogic.EXPECT().Validator().Return(mockValidator)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.InitChain(abcitypes.RequestInitChain{})
				}).To(Panic())
			})
		})

		When("failure to commit state", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return(nil)
				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockLogic.EXPECT().WriteGenesisState().Return(nil)
				mockValidator := mocks.NewMockValidatorLogic(ctrl)
				mockValidator.EXPECT().Index(gomock.Any(), gomock.Any()).Return(nil)
				mockLogic.EXPECT().Commit().Return(fmt.Errorf("error"))
				mockLogic.EXPECT().Validator().Return(mockValidator)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.InitChain(abcitypes.RequestInitChain{})
				}).To(Panic())
			})
		})

		When("initialization succeeds", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return(nil).Times(2)
				mockLogic.EXPECT().StateTree().Return(mockTree).AnyTimes()
				mockLogic.EXPECT().WriteGenesisState().Return(nil)
				mockLogic.EXPECT().Commit().Return(nil)
				mockTree.EXPECT().Version().Return(int64(1))
				mockValidator := mocks.NewMockValidatorLogic(ctrl)
				mockValidator.EXPECT().Index(gomock.Any(), gomock.Any()).Return(nil)
				mockLogic.EXPECT().Validator().Return(mockValidator)
				app.logic = mockLogic
			})

			It("should return an empty response", func() {
				resp := app.InitChain(abcitypes.RequestInitChain{})
				Expect(resp).To(Equal(abcitypes.ResponseInitChain{}))
			})
		})
	})

	Describe(".updateValidators", func() {
		When("the provided height is not the block height preceding the last epoch", func() {
			It("should return nil", func() {
				Expect(app.updateValidators(4, nil)).To(BeNil())
			})
		})

		When("an error occurred when making secret", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				mockLogic.EXPECT().Sys().Return(mockSysLogic)
				app.logic = mockLogic
			})

			It("should return error", func() {
				err := app.updateValidators(6, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})

		When("an error occurred when selecting random validators", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(nil, fmt.Errorf("error selecting validators"))
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.EXPECT().Sys().Return(mockSysLogic)
				app.logic = mockLogic
				app.ticketMgr = mockTickMgr
			})

			It("should return error", func() {
				err := app.updateValidators(6, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error selecting validators"))
			})
		})

		When("when no tickets are randomly selected", func() {
			var tickets = []*types.Ticket{}

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockTickMgr

				mockLogic := mocks.NewMockAtomicLogic(ctrl)

				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.EXPECT().Sys().Return(mockSysLogic)

				app.logic = mockLogic
			})

			It("should return nil and no validator updates in endblock response", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(6, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(BeEmpty())
			})
		})

		When("when no validator currently exists and two tickets are randomly selected", func() {
			var key = crypto.NewKeyFromIntSeed(1)
			var key2 = crypto.NewKeyFromIntSeed(1)
			var t = &types.Ticket{ProposerPubKey: key.PubKey().Base58()}
			var t2 = &types.Ticket{ProposerPubKey: key2.PubKey().Base58()}
			var tickets = []*types.Ticket{t, t2}

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockTickMgr

				mockLogic := mocks.NewMockAtomicLogic(ctrl)

				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.EXPECT().Sys().Return(mockSysLogic)

				mockValKeeper := mocks.NewMockValidatorKeeper(ctrl)
				mockValKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[string]*types.Validator{}, nil)

				mockLogic.EXPECT().ValidatorKeeper().Return(mockValKeeper)
				app.logic = mockLogic
			})

			It("should add the two validators to the response object", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(6, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
			})
		})

		When("when one validator currently exists and another different validator is selected", func() {
			var existingValKey = crypto.NewKeyFromIntSeed(1)
			var keyOfNewTicket = crypto.NewKeyFromIntSeed(2)
			var newTicket = &types.Ticket{ProposerPubKey: keyOfNewTicket.PubKey().Base58(), Height: 100}
			var tickets = []*types.Ticket{newTicket}

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5

				// Mock the return of the tickets
				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().SelectRandom(gomock.Any(), gomock.Any(), gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockTickMgr

				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.EXPECT().Sys().Return(mockSysLogic)

				// Mock the return of the existing validator
				mockValKeeper := mocks.NewMockValidatorKeeper(ctrl)
				pubKeyBz, _ := existingValKey.PubKey().Bytes()
				pubKeyHex := types.HexBytes(pubKeyBz)
				mockValKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[string]*types.Validator{
					pubKeyHex.String(): &types.Validator{Power: 1},
				}, nil)

				mockLogic.EXPECT().ValidatorKeeper().Return(mockValKeeper)
				app.logic = mockLogic
			})

			It("should add existing validator but change power to zero (0)", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(6, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
				Expect(resp.ValidatorUpdates[1].Power).To(Equal(int64(0)))
				pubKeyBz, _ := existingValKey.PubKey().Bytes()
				Expect(resp.ValidatorUpdates[1].PubKey.GetData()).To(Equal(pubKeyBz))
			})

			It("should add new validator and set power to 1", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(6, &resp)
				Expect(err).To(BeNil())
				Expect(resp.ValidatorUpdates).To(HaveLen(2))
				Expect(resp.ValidatorUpdates[0].Power).To(Equal(int64(1)))
				pubKeyBz, _ := keyOfNewTicket.PubKey().Bytes()
				Expect(resp.ValidatorUpdates[0].PubKey.GetData()).To(Equal(pubKeyBz))
			})
		})

		When("when error occurred when fetching current validators", func() {
			var key = crypto.NewKeyFromIntSeed(1)
			var key2 = crypto.NewKeyFromIntSeed(1)
			var t = &types.Ticket{ProposerPubKey: key.PubKey().Base58()}
			var t2 = &types.Ticket{ProposerPubKey: key2.PubKey().Base58()}
			var tickets = []*types.Ticket{t, t2}

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockTickMgr := mocks.NewMockTicketManager(ctrl)
				mockTickMgr.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockTickMgr

				mockLogic := mocks.NewMockAtomicLogic(ctrl)

				mockSysLogic := mocks.NewMockSysLogic(ctrl)
				mockSysLogic.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.EXPECT().Sys().Return(mockSysLogic)

				mockValKeeper := mocks.NewMockValidatorKeeper(ctrl)
				mockValKeeper.EXPECT().GetByHeight(gomock.Any()).Return(nil, fmt.Errorf("bad error"))

				mockLogic.EXPECT().ValidatorKeeper().Return(mockValKeeper)
				app.logic = mockLogic
			})

			It("should return error", func() {
				var resp abcitypes.ResponseEndBlock
				err := app.updateValidators(6, &resp)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})
	})

	Describe(".Info", func() {
		When("getting last block info and error is returned and it is not ErrBlockInfoNotFound", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("something bad"))
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Info(abcitypes.RequestInfo{})
				}).To(Panic())
			})
		})

		When("getting last block info and error is returned and it is ErrBlockInfoNotFound", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, keepers.ErrBlockInfoNotFound)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				app.logic = mockLogic
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
				Expect(res.Log).To(Equal("unable to decode to types.Transaction"))
			})
		})

		When("tx is valid", func() {
			var res abcitypes.ResponseCheckTx
			var expectedHash util.Hash
			BeforeEach(func() {
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
				tx := types.NewTx(199, 0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return fmt.Errorf("bad error")
				}
				tx := types.NewTx(199, 0, sender.Addr(), sender, "10", "1", 1)
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
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				sysLogic := mocks.NewMockSysLogic(ctrl)

				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				req := abcitypes.RequestBeginBlock{}
				req.Header.ProposerAddress = pv.GetAddress().Bytes()

				sysLogic.EXPECT().CheckSetNetMaturity().Return(nil)
				mockLogic.EXPECT().Sys().Return(sysLogic)
				app.logic = mockLogic
				app.BeginBlock(req)
			})

			It("should set `isCurrentBlockProposer` to true", func() {
				Expect(app.isCurrentBlockProposer).To(BeTrue())
			})
		})

		When("network is not mature", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				sysLogic := mocks.NewMockSysLogic(ctrl)

				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				sysLogic.EXPECT().CheckSetNetMaturity().Return(fmt.Errorf("not mature"))
				mockLogic.EXPECT().Sys().Return(sysLogic)
				app.logic = mockLogic
				app.BeginBlock(abcitypes.RequestBeginBlock{})
			})

			It("should set `mature` to false", func() {
				Expect(app.mature).To(BeFalse())
			})
		})

		When("network is not mature", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				sysLogic := mocks.NewMockSysLogic(ctrl)

				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				req := abcitypes.RequestBeginBlock{}
				req.Header.ProposerAddress = pv.GetAddress().Bytes()

				sysLogic.EXPECT().CheckSetNetMaturity().Return(nil)
				mockLogic.EXPECT().Sys().Return(sysLogic)
				app.logic = mockLogic
				app.BeginBlock(req)
			})

			It("should set `isCurrentBlockProposer` to true", func() {
				Expect(app.mature).To(BeTrue())
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
				Expect(res.Log).To(Equal("unable to decode to types.Transaction"))
			})

			Specify("that txIndex is incremented", func() {
				Expect(app.txIndex).To(Equal(1))
			})
		})

		When("tx type is TxTypeGetTicket; max. TxTypeGetTicket per "+
			"block is 1; 1 TxTypeGetTicket tx has previously been seen", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				params.MaxValTicketsPerBlock = 1
				app.ticketPurchaseTxs = append(app.ticketPurchaseTxs, &tickPurchaseTx{})
				tx := types.NewTx(types.TxTypeGetTicket, 0, sender.Addr(), sender, "10", "1", 1)
				res = app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx.Bytes()})
			})

			It("should return code=types.ErrCodeMaxValTxTypeReached and"+
				" log='failed to execute tx: validator ticket capacity reached'", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeMaxTxTypeReached)))
				Expect(res.Log).To(Equal("failed to execute tx: validator ticket capacity reached"))
			})
		})

		When("tx type is TxTypeGetTicket and is successfully executed", func() {
			BeforeEach(func() {
				tx := types.NewTx(types.TxTypeGetTicket, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				txLogic := mocks.NewMockTxLogic(ctrl)
				txLogic.EXPECT().PrepareExec(req).Return(abcitypes.ResponseDeliverTx{})
				mockLogic.EXPECT().Tx().Return(txLogic)
				app.logic = mockLogic
				app.DeliverTx(req)
			})

			It("should return cache the validator ticket tx", func() {
				Expect(app.ticketPurchaseTxs).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeEpochSecret and the current block "+
			"is not last in the current epoch", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 4
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				res = app.DeliverTx(req)
			})

			It("should return code=ErrCodeTxTypeUnexpected and err='failed to execute tx: epoch secret not expected'", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeTxTypeUnexpected)))
				Expect(res.Log).To(Equal("failed to execute tx: epoch secret not expected"))
			})
		})

		When("tx type TxTypeEpochSecret has been seen/cached", func() {
			var res abcitypes.ResponseDeliverTx
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5
				app.epochSecretTx = tx
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				res = app.DeliverTx(req)
			})

			It("should return code=ErrCodeMaxValTxTypeReached and err='failed to execute tx: epoch secret capacity reached'", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeMaxTxTypeReached)))
				Expect(res.Log).To(Equal("failed to execute tx: epoch secret capacity reached"))
			})
		})

		When("tx type TxTypeEpochSecret is successfully executed", func() {
			var res abcitypes.ResponseDeliverTx
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTxLogic := mocks.NewMockTxLogic(ctrl)
				mockTxLogic.EXPECT().PrepareExec(gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: uint32(0),
				})
				mockLogic.EXPECT().Tx().Return(mockTxLogic)
				app.logic = mockLogic

				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				res = app.DeliverTx(req)
			})

			It("should return code=0 and epochSecretTx must be set as the processed tx", func() {
				Expect(res.Code).To(BeZero())
				Expect(app.epochSecretTx).To(Equal(tx))
			})
		})

		When("tx type TxTypeEpochSecret but it is stale", func() {
			var res abcitypes.ResponseDeliverTx
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTxLogic := mocks.NewMockTxLogic(ctrl)
				mockTxLogic.EXPECT().PrepareExec(gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: types.ErrCodeTxInvalidValue,
					Log:  types.ErrStaleSecretRound(1).Error(),
				})
				mockLogic.EXPECT().Tx().Return(mockTxLogic)
				app.logic = mockLogic

				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				res = app.DeliverTx(req)
			})

			It("should return code=types.ErrCodeTxInvalidValue, err=ErrStaleSecretRound", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeTxInvalidValue)))
				Expect(res.Log).To(ContainSubstring(types.ErrStaleSecretRound(1).Error()))
			})

			Specify("that the cached epoch tx has been invalidated", func() {
				Expect(app.epochSecretTx.GetID()).To(Equal(tx.GetID()))
				Expect(app.epochSecretTx.IsInvalidated()).To(BeTrue())
			})
		})

		When("tx type TxTypeEpochSecret but it has an early, unexpected round", func() {
			var res abcitypes.ResponseDeliverTx
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockTxLogic := mocks.NewMockTxLogic(ctrl)
				mockTxLogic.EXPECT().PrepareExec(gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: types.ErrCodeTxInvalidValue,
					Log:  types.ErrEarlySecretRound(1).Error(),
				})
				mockLogic.EXPECT().Tx().Return(mockTxLogic)
				app.logic = mockLogic

				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				res = app.DeliverTx(req)
			})

			It("should return code=types.ErrCodeTxInvalidValue, err=ErrEarlySecretRound", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeTxInvalidValue)))
				Expect(res.Log).To(ContainSubstring(types.ErrEarlySecretRound(1).Error()))
			})

			Specify("that the cached epoch tx has been invalidated", func() {
				Expect(app.epochSecretTx.GetID()).To(Equal(tx.GetID()))
				Expect(app.epochSecretTx.IsInvalidated()).To(BeTrue())
			})
		})
	})

	Describe(".Commit", func() {

		When("epoch secret tx is set", func() {
			var tx *types.Transaction

			When("when unable to save epoch secret", func() {
				BeforeEach(func() {
					tx = types.NewBareTx(types.TxTypeEpochSecret)
					tx.Secret = util.RandBytes(64)
					tx.PreviousSecret = util.RandBytes(64)
					tx.SecretRound = 18
					app.epochSecretTx = tx
				})

				BeforeEach(func() {
					mockLogic := mocks.NewMockAtomicLogic(ctrl)
					mockStateTree := mocks.NewMockTree(ctrl)
					mockStateTree.EXPECT().WorkingHash().Return(nil)
					mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
					mockSysKeeper.EXPECT().SetHighestDrandRound(gomock.Any()).Return(fmt.Errorf("error"))
					mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
					mockLogic.EXPECT().StateTree().Return(mockStateTree)
					app.logic = mockLogic
				})

				It("should panic", func() {
					Expect(func() {
						app.Commit()
					}).To(Panic())
				})
			})
		})

		When("error occurred when saving latest block info", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
				app.epochSecretTx = tx
			})

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().SetHighestDrandRound(gomock.Any()).Return(nil)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(fmt.Errorf("bad"))
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when indexing cached ticket", func() {

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				mockTicketMgr := mocks.NewMockTicketManager(ctrl)
				tx := types.NewTx(types.TxTypeGetTicket, 0, sender.Addr(), sender, "10", "1", 1)
				app.ticketPurchaseTxs = append(app.ticketPurchaseTxs, &tickPurchaseTx{
					Tx:    tx,
					index: 1,
				})
				mockTicketMgr.EXPECT().Index(gomock.Any(),
					gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))

				// It should try to remove tickets if already saved
				mockTicketMgr.EXPECT().Remove(tx.GetID())

				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree)
				app.logic = mockLogic
				app.ticketMgr = mockTicketMgr
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when saving validators", func() {

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockValKeeper := mocks.NewMockValidatorKeeper(ctrl)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				app.wBlock.Height = 10
				app.heightToSaveNewValidators = 10
				mockValKeeper.EXPECT().Index(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))

				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockLogic.EXPECT().ValidatorKeeper().Return(mockValKeeper)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when saving tree version", func() {

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				app.heightToSaveNewValidators = 100

				mockTree.EXPECT().SaveVersion().Return([]byte{}, int64(0), fmt.Errorf("error"))

				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree).AnyTimes()
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when trying to save un-indexed tx", func() {
			appHash := []byte("app_hash")

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockTxKeeper := mocks.NewMockTxKeeper(ctrl)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				app.heightToSaveNewValidators = 100

				mockTree.EXPECT().SaveVersion().Return(appHash, int64(0), nil)

				mockTxKeeper.EXPECT().Index(gomock.Any()).Return(fmt.Errorf("error"))

				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree).AnyTimes()
				mockLogic.EXPECT().TxKeeper().Return(mockTxKeeper).AnyTimes()
				app.logic = mockLogic

				app.unIndexedTxs = append(app.unIndexedTxs, types.NewBareTx(0))
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("no error occurred", func() {
			appHash := []byte("app_hash")

			BeforeEach(func() {
				mockLogic := mocks.NewMockAtomicLogic(ctrl)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)

				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return([]byte("working_hash"))

				mockSysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)

				app.heightToSaveNewValidators = 100

				mockTree.EXPECT().SaveVersion().Return(appHash, int64(0), nil)

				mockLogic.EXPECT().Commit().Return(nil)

				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				mockLogic.EXPECT().StateTree().Return(mockTree).AnyTimes()
				app.logic = mockLogic
			})

			It("should return expected app hash", func() {
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})
		})
	})
})
