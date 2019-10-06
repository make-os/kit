package node

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"

	"github.com/golang/mock/gomock"

	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/ticket"

	l "github.com/makeos/mosdef/logic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
)

var _ = Describe("App", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var state *tree.SafeTree
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
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = l.New(c, state, cfg)
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
				mockLogic := mocks.NewMockLogic(ctrl)
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

		When("initialization succeeds", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().WorkingHash().Return(nil).Times(2)
				mockLogic.EXPECT().StateTree().Return(mockTree).Times(2)
				mockLogic.EXPECT().WriteGenesisState().Return(nil)
				app.logic = mockLogic
			})

			It("should return an empty response", func() {
				resp := app.InitChain(abcitypes.RequestInitChain{})
				Expect(resp).To(Equal(abcitypes.ResponseInitChain{}))
			})
		})
	})

	Describe(".Info", func() {
		When("getting last block info and error is returned and it is not ErrBlockInfoNotFound", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
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
				mockLogic := mocks.NewMockLogic(ctrl)
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

		When("tx type is TxTypeTicketValidator; max. TxTypeTicketValidator per "+
			"block is 1; 1 TxTypeTicketValidator tx has previously been seen", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				params.MaxValTicketsPerBlock = 1
				app.ticketPurchaseTxs = append(app.ticketPurchaseTxs, &tickPurchaseTx{})
				tx := types.NewTx(types.TxTypeTicketValidator, 0, sender.Addr(), sender, "10", "1", 1)
				res = app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx.Bytes()})
			})

			It("should return code=types.ErrCodeMaxValTxTypeReached and"+
				" log='failed to execute tx: validator ticket capacity reached'", func() {
				Expect(res.Code).To(Equal(uint32(types.ErrCodeMaxTxTypeReached)))
				Expect(res.Log).To(Equal("failed to execute tx: validator ticket capacity reached"))
			})
		})

		When("tx type is TxTypeTicketValidator and is successfully executed", func() {
			BeforeEach(func() {
				tx := types.NewTx(types.TxTypeTicketValidator, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic := mocks.NewMockLogic(ctrl)
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
				app.workingBlock.Height = 4
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
				app.workingBlock.Height = 5
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
				app.workingBlock.Height = 5

				mockLogic := mocks.NewMockLogic(ctrl)
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
				app.workingBlock.Height = 5

				mockLogic := mocks.NewMockLogic(ctrl)
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
				app.workingBlock.Height = 5

				mockLogic := mocks.NewMockLogic(ctrl)
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
		When("error occurred during tree version update", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().SaveVersion().Return(nil, int64(0), fmt.Errorf("something bad"))
				mockLogic.EXPECT().StateTree().Return(mockTree)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when saving latest block info", func() {
			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().SaveVersion().Return(nil, int64(0), nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().SaveBlockInfo(&types.BlockInfo{}).Return(fmt.Errorf("bad"))
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().StateTree().Return(mockTree)
				app.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("epoch secret tx is set", func() {
			var appHash = []byte("app_hash")
			var tx *types.Transaction

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 18
				app.epochSecretTx = tx
			})

			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().SaveVersion().Return(appHash, int64(0), nil)

				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().SetHighestDrandRound(gomock.Any())
				mockSysKeeper.EXPECT().SaveBlockInfo(&types.BlockInfo{
					AppHash:             appHash,
					EpochSecret:         tx.Secret,
					EpochPreviousSecret: tx.PreviousSecret,
					EpochRound:          tx.SecretRound,
				}).Return(nil)

				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				app.logic = mockLogic
			})

			Specify("SaveBlockInfo is passed the epoch secret tx fields", func() {
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})
		})

		When("cached validator tickets are indexed", func() {
			var appHash = []byte("app_hash")

			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockTree := mocks.NewMockTree(ctrl)
				mockTree.EXPECT().SaveVersion().Return(appHash, int64(0), nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().SaveBlockInfo(&types.BlockInfo{AppHash: appHash}).Return(nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().StateTree().Return(mockTree)
				mockTicketMgr := mocks.NewMockTicketManager(ctrl)
				tx := types.NewTx(types.TxTypeTicketValidator, 0, sender.Addr(), sender, "10", "1", 1)
				app.ticketPurchaseTxs = append(app.ticketPurchaseTxs, &tickPurchaseTx{
					Tx:    tx,
					index: 1,
				})
				mockTicketMgr.EXPECT().Index(tx, tx.SenderPubKey.String(), uint64(0), 1)
				app.logic = mockLogic
				app.ticketMgr = mockTicketMgr
				res := app.Commit()
				Expect(res.Data).To(Equal(appHash))
			})

			It("should reset the app's caches and flag members", func() {
				Expect(app.ticketPurchaseTxs).To(BeEmpty())
				Expect(app.workingBlock).To(Equal(&types.BlockInfo{}))
				Expect(app.txIndex).To(Equal(0))
			})
		})
	})
})
