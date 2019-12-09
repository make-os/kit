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
		When("the provided height is not the block height preceding the last epoch", func() {
			It("should return nil", func() {
				Expect(app.updateValidators(4, nil)).To(BeNil())
			})
		})

		When("an error occurred when making secret", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				app.logic = mockLogic.AtomicLogic
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
				mockLogic.TicketManager.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(nil, fmt.Errorf("error selecting validators"))
				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				app.logic = mockLogic.AtomicLogic
				app.ticketMgr = mockLogic.TicketManager
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
				mockLogic.TicketManager.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockLogic.TicketManager
				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				app.logic = mockLogic.AtomicLogic
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
				mockLogic.TicketManager.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockLogic.TicketManager
				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[string]*types.Validator{}, nil)
				app.logic = mockLogic.AtomicLogic
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
				mockLogic.TicketManager.EXPECT().SelectRandom(gomock.Any(), gomock.Any(), gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockLogic.TicketManager

				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)

				// Mock the return of the existing validator
				pubKeyBz, _ := existingValKey.PubKey().Bytes()
				pubKeyHex := types.HexBytes(pubKeyBz)
				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(map[string]*types.Validator{
					pubKeyHex.String(): &types.Validator{Power: 1},
				}, nil)

				app.logic = mockLogic.AtomicLogic
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
				mockLogic.TicketManager.EXPECT().SelectRandom(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(tickets, nil)
				app.ticketMgr = mockLogic.TicketManager
				mockLogic.Sys.EXPECT().MakeSecret(gomock.Any()).Return(nil, nil)
				mockLogic.ValidatorKeeper.EXPECT().GetByHeight(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				app.logic = mockLogic.AtomicLogic
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
				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				req := abcitypes.RequestBeginBlock{}
				req.Header.ProposerAddress = pv.GetAddress().Bytes()

				mockLogic.Sys.EXPECT().CheckSetNetMaturity().Return(nil)
				app.logic = mockLogic.AtomicLogic
				app.BeginBlock(req)
			})

			It("should set `isCurrentBlockProposer` to true", func() {
				Expect(app.isCurrentBlockProposer).To(BeTrue())
			})
		})

		When("network is not mature", func() {
			BeforeEach(func() {
				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				mockLogic.Sys.EXPECT().CheckSetNetMaturity().Return(fmt.Errorf("not mature"))
				app.logic = mockLogic.AtomicLogic
				app.BeginBlock(abcitypes.RequestBeginBlock{})
			})

			It("should set `mature` to false", func() {
				Expect(app.mature).To(BeFalse())
			})
		})

		When("network is not mature", func() {
			BeforeEach(func() {
				pv := crypto.WrappedPV{FilePV: genFilePV([]byte("xyz"))}
				cfg.G().PrivVal = &pv

				req := abcitypes.RequestBeginBlock{}
				req.Header.ProposerAddress = pv.GetAddress().Bytes()

				mockLogic.Sys.EXPECT().CheckSetNetMaturity().Return(nil)
				app.logic = mockLogic.AtomicLogic
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

		When("tx is invalid", func() {
			var res abcitypes.ResponseDeliverTx
			BeforeEach(func() {
				tx := types.NewTx(100000, 0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				params.MaxValTicketsPerBlock = 1
				app.validatorTickets = append(app.validatorTickets, &ticketInfo{})
				tx := types.NewTx(types.TxTypeValidatorTicket, 0, sender.Addr(), sender, "10", "1", 1)
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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewTx(types.TxTypeValidatorTicket, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().PrepareExec(req, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				app.DeliverTx(req)
			})

			It("should return cache the validator ticket tx", func() {
				Expect(app.validatorTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeStorerTicket and response code=0", func() {
			BeforeEach(func() {
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewTx(types.TxTypeStorerTicket, 0, sender.Addr(), sender, "10", "1", 1)
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().PrepareExec(req, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should return cache the storer ticket tx", func() {
				Expect(app.storerTickets).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeUnbondStorerTicket and response code=0", func() {
			BeforeEach(func() {
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx := types.NewTx(types.TxTypeUnbondStorerTicket, 0, sender.Addr(), sender, "10", "1", 1)
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte("tid")}
				req := abcitypes.RequestDeliverTx{Tx: tx.Bytes()}
				mockLogic.Tx.EXPECT().PrepareExec(req, gomock.Any()).Return(abcitypes.ResponseDeliverTx{})
				app.logic = mockLogic.AtomicLogic
				Expect(app.DeliverTx(req).Code).To(Equal(uint32(0)))
			})

			It("should return cache the unbond storer ticket tx", func() {
				Expect(app.unbondStorerRequests).To(HaveLen(1))
			})
		})

		When("tx type is TxTypeEpochSecret and the current block "+
			"is not last in the current epoch", func() {
			var res abcitypes.ResponseDeliverTx

			BeforeEach(func() {
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 4
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic.Tx.EXPECT().PrepareExec(gomock.Any(), gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: uint32(0),
				})
				app.logic = mockLogic.AtomicLogic

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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic.Tx.EXPECT().PrepareExec(gomock.Any(), gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: types.ErrCodeTxInvalidValue,
					Log:  types.ErrStaleSecretRound(1).Error(),
				})
				app.logic = mockLogic.AtomicLogic

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
				app.validateTx = func(tx *types.Transaction, i int, logic types.Logic) error {
					return nil
				}
			})

			BeforeEach(func() {
				tx = types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
			})

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				app.wBlock.Height = 5

				mockLogic.Tx.EXPECT().PrepareExec(gomock.Any(), gomock.Any()).Return(abcitypes.ResponseDeliverTx{
					Code: types.ErrCodeTxInvalidValue,
					Log:  types.ErrEarlySecretRound(1).Error(),
				})
				app.logic = mockLogic.AtomicLogic

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
					tx.EpochSecret = &types.EpochSecret{}
					tx.EpochSecret.Secret = util.RandBytes(64)
					tx.EpochSecret.PreviousSecret = util.RandBytes(64)
					tx.EpochSecret.SecretRound = 18
					app.epochSecretTx = tx
				})

				BeforeEach(func() {
					mockLogic.StateTree.EXPECT().WorkingHash().Return(nil)
					mockLogic.SysKeeper.EXPECT().SetHighestDrandRound(gomock.Any()).Return(fmt.Errorf("error"))
					mockLogic.AtomicLogic.EXPECT().Discard().Return()
					app.logic = mockLogic.AtomicLogic
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
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				tx.EpochSecret.SecretRound = 18
				app.epochSecretTx = tx
			})

			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().SetHighestDrandRound(gomock.Any()).Return(nil)
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(fmt.Errorf("bad"))
				mockLogic.AtomicLogic.EXPECT().Discard().Return()
				app.logic = mockLogic.AtomicLogic
			})

			It("should panic", func() {
				Expect(func() {
					app.Commit()
				}).To(Panic())
			})
		})

		When("error occurred when saving validators", func() {

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.wBlock.Height = 10
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

		When("error occurred when trying to save un-indexed tx", func() {

			BeforeEach(func() {
				mockLogic.StateTree.EXPECT().WorkingHash().Return([]byte("working_hash"))
				mockLogic.SysKeeper.EXPECT().SaveBlockInfo(gomock.Any()).Return(nil)
				app.heightToSaveNewValidators = 100
				mockLogic.TxKeeper.EXPECT().Index(gomock.Any()).Return(fmt.Errorf("error"))
				mockLogic.AtomicLogic.EXPECT().Discard().Return()
				mockLogic.AtomicLogic.EXPECT().TxKeeper().Return(mockLogic.TxKeeper).AnyTimes()
				app.logic = mockLogic.AtomicLogic
				app.unIndexedTxs = append(app.unIndexedTxs, types.NewBareTx(0))
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
				app.unbondStorerRequests = append(app.unbondStorerRequests, "ticket_hash")
				mockLogic.TicketManager.EXPECT().UpdateDecayBy("ticket_hash", uint64(app.wBlock.Height))
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
