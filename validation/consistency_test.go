package validation_test

import (
	"fmt"
	"os"

	"github.com/make-os/lobe/remote/push/types"
	storagetypes "github.com/make-os/lobe/storage/types"
	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	crypto2 "github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"

	"github.com/make-os/lobe/params"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxValidator", func() {
	var key = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)
	var appDB, stateTreeDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockObjects *testutil.MockObjects
	var mockLogic *mocks.MockLogic
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockTickMgr *mocks.MockTicketManager
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()

		ctrl = gomock.NewController(GinkgoT())
		mockObjects = testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockSysKeeper = mockObjects.SysKeeper
		mockTickMgr = mockObjects.TicketManager
		mockRepoKeeper = mockObjects.RepoKeeper
		mockPushKeyKeeper = mockObjects.PushKeyKeeper
		mockNSKeeper = mockObjects.NamespaceKeeper
		mockAcctKeeper = mockObjects.AccountKeeper
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxCoinTransferConsistency", func() {
		When("recipient address is a repo address of which the repo does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "r/repo"
				mockRepoKeeper.EXPECT().Get("repo").Return(state.BareRepository())
				err = validation.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient repo not found"))
			})
		})

		When("recipient address is a namespaced address of which the target is a repo address "+
			"pointing to repo that does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "namespace/cool-repo"

				mockNSKeeper.EXPECT().GetTarget(tx.To.String()).Return("r/repo", nil)
				mockRepoKeeper.EXPECT().Get("repo").Return(state.BareRepository())

				err = validation.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient repo not found"))
			})
		})

		When("recipient address is a namespaced address of which the namespace could not be found", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "namespace/cool-repo"
				mockNSKeeper.EXPECT().GetTarget(tx.To.String()).Return("", fmt.Errorf("error"))
				err = validation.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:error"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.Value = "10.2"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				mockLogic.EXPECT().DrySend(key.PubKey(), tx.Value, tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))
				err = validation.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxTicketPurchaseConsistency", func() {
		When("delegate is set", func() {
			var delegate = crypto.NewKeyFromIntSeed(1)

			When("unable to get active ticket of delegate", func() {
				BeforeEach(func() {
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					tx.Delegate = crypto.BytesToPublicKey(delegate.PubKey().MustBytes())

					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return(nil, fmt.Errorf("error"))

					err = validation.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("failed to get active delegate tickets: error"))
				})
			})

			When("delegate has no active ticket", func() {
				BeforeEach(func() {
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					tx.Delegate = crypto.BytesToPublicKey(delegate.PubKey().MustBytes())

					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return([]*tickettypes.Ticket{}, nil)

					err = validation.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:delegate, msg:specified delegate is not active"))
				})
			})

			When("for non-delegated, validator ticket - ticket price is less than current ticket price", func() {
				BeforeEach(func() {
					params.MinValidatorsTicketPrice = 10
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
					tx.Value = "1"
					tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					err = validation.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(ContainSubstring("field:value, msg:value is lower than the minimum ticket price"))
				})
			})

			When("coin transfer dry-run fails", func() {
				BeforeEach(func() {
					params.MinValidatorsTicketPrice = 10
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
					tx.Value = "10.5"
					tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					mockLogic.EXPECT().DrySend(key.PubKey(), tx.Value, tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))

					err = validation.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("error"))
				})
			})
		})
	})

	Describe(".CheckTxUnbondTicketConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
				err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("target ticket does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
				tx.TicketHash = util.StrToHexBytes("ticket_hash")

				bi := &state.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(nil)

				err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, msg:ticket not found"))
			})
		})

		When("ticket is not delegated", func() {
			When("sender is not the ticket proposer", func() {
				BeforeEach(func() {
					key2 := crypto.NewKeyFromIntSeed(2)
					tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
					tx.TicketHash = util.StrToHexBytes("ticket_hash")
					tx.SetSenderPubKey(key2.PubKey().MustBytes())

					bi := &state.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}
					mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

					err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:hash, msg:sender not authorized to unbond this ticket"))
				})
			})
		})

		When("ticket is delegated", func() {
			When("sender is not the delegator", func() {
				BeforeEach(func() {
					key2 := crypto.NewKeyFromIntSeed(2)
					tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
					tx.TicketHash = util.StrToHexBytes("ticket_hash")
					tx.SetSenderPubKey(key2.PubKey().MustBytes())

					bi := &state.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &tickettypes.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						Delegator:      key.Addr().String(),
					}
					mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

					err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:hash, msg:sender not authorized to unbond this ticket"))
				})
			})
		})

		When("ticket expire height is set and greater than current block height", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
				tx.TicketHash = util.StrToHexBytes("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &state.BlockInfo{Height: 50}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &tickettypes.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					ExpireBy:       100,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, msg:ticket is already expiring"))
			})
		})

		When("ticket expire height is set less than current block height", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
				tx.TicketHash = util.StrToHexBytes("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &state.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &tickettypes.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					ExpireBy:       100,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, msg:ticket has already expired"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
				tx.TicketHash = util.StrToHexBytes("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &state.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &tickettypes.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					ExpireBy:       0,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))

				err = validation.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxRepoCreateConsistency", func() {
		When("repo name is not unique", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxRepoCreate()
				tx.Name = "repo1"

				repo := state.BareRepository()
				repo.AddOwner("some_address", &state.RepoOwner{})
				mockRepoKeeper.EXPECT().Get(tx.Name).Return(repo)

				err = validation.CheckTxRepoCreateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:name is not available. choose another"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxRepoCreate()
				tx.Name = "repo1"
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				repo := state.BareRepository()
				mockRepoKeeper.EXPECT().Get(tx.Name).Return(repo)

				mockLogic.EXPECT().DrySend(key.PubKey(), tx.Value, tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))

				err = validation.CheckTxRepoCreateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxSetDelegateCommissionConsistency", func() {
		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxSetDelegateCommission()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))

				err = validation.CheckTxSetDelegateCommissionConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxRegisterPushKeyConsistency", func() {
		When("push public key has already been registered", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxRegisterPushKey()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				pushKey := crypto.NewKeyFromIntSeed(1)
				tx.PublicKey = crypto.BytesToPublicKey(pushKey.PubKey().MustBytes())

				pushKeyID := crypto.CreatePushKeyID(tx.PublicKey)
				mockPushKeyKeeper.EXPECT().Get(pushKeyID).Return(&state.PushKey{PubKey: pushKey.PubKey().ToPublicKey()})

				err = validation.CheckTxRegisterPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, msg:push key already registered"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxRegisterPushKey()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				pushKey := crypto.NewKeyFromIntSeed(1)
				tx.PublicKey = crypto.BytesToPublicKey(pushKey.PubKey().MustBytes())

				pushKeyID := crypto.CreatePushKeyID(tx.PublicKey)
				mockPushKeyKeeper.EXPECT().Get(pushKeyID).Return(&state.PushKey{})

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))

				err = validation.CheckTxRegisterPushKeyConsistency(tx, -1, mockLogic)

			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxUpDelPushKeyConsistency", func() {
		When("push key does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxUpDelPushKey()
				tx.ID = "pk1_abc"
				mockPushKeyKeeper.EXPECT().Get(tx.ID).Return(state.BarePushKey())
				err = validation.CheckTxUpDelPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:push key not found"))
			})
		})

		When("sender is not the owner of the target push key", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxUpDelPushKey()
				tx.ID = "pk1_abc"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				pushKey := state.BarePushKey()
				pushKey.Address = "addr1"
				mockPushKeyKeeper.EXPECT().Get(tx.ID).Return(pushKey)

				err = validation.CheckTxUpDelPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender is not the owner of the key"))
			})
		})

		When("an index in removeScopes is out of bound/range", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxUpDelPushKey()
				tx.ID = "pk1_abc"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.RemoveScopes = []int{1}

				pushKey := state.BarePushKey()
				pushKey.Address = key.Addr()
				pushKey.Scopes = []string{"scope1"}
				mockPushKeyKeeper.EXPECT().Get(tx.ID).Return(pushKey)

				err = validation.CheckTxUpDelPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:removeScopes[0], msg:index out of range"))
			})
		})

		When("balance sufficiency dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxUpDelPushKey()
				tx.ID = "pk1_abc"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.RemoveScopes = []int{0}

				pushKey := state.BarePushKey()
				pushKey.Address = key.Addr()
				pushKey.Scopes = []string{"scope1"}
				mockPushKeyKeeper.EXPECT().Get(tx.ID).Return(pushKey)

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))
				err = validation.CheckTxUpDelPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})

		When("balance sufficiency dry-run passes", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxUpDelPushKey()
				tx.ID = "pk1_abc"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.RemoveScopes = []int{0}

				pushKey := state.BarePushKey()
				pushKey.Address = key.Addr()
				pushKey.Scopes = []string{"scope1"}
				mockPushKeyKeeper.EXPECT().Get(tx.ID).Return(pushKey)

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(nil)
				err = validation.CheckTxUpDelPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNSAcquireConsistency", func() {

		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := txns.NewBareTxNamespaceRegister()
				err = validation.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("target namespace exist and not expired", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceRegister()
				tx.Name = name

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 9}, nil)

				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{GraceEndAt: 10})
				err = validation.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:name, msg:chosen name is not currently available'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:chosen name is not currently available"))
			})
		})

		When("target repo does not exist", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceRegister()
				tx.Name = name
				tx.To = "repo1"

				bi := &state.BlockInfo{Height: 9}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockRepoKeeper.EXPECT().Get(tx.To).Return(state.BareRepository())

				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{GraceEndAt: 0})
				err = validation.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:toRepo, msg:repo does not exist'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:repo does not exist"))
			})
		})

		When("target account does not exist", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceRegister()
				tx.Name = name
				tx.To = "os1m4aaslnzmdp4k3g52tk6eh94ghr547exvtcrkd"

				bi := &state.BlockInfo{Height: 9}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockAcctKeeper.EXPECT().Get(identifier.Address(tx.To)).Return(state.BareAccount())

				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{GraceEndAt: 0})
				err = validation.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:toAccount, msg:account does not exist'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:account does not exist"))
			})
		})

		When("balance sufficiency dry-run fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxNamespaceRegister()
				tx.Value = "10.2"
				tx.Name = "name1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				bi := &state.BlockInfo{Height: 10}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{GraceEndAt: 9})

				mockLogic.EXPECT().DrySend(key.PubKey(), tx.Value, tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))
				err = validation.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxNamespaceDomainUpdateConsistency", func() {
		When("target namespace is not found", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceDomainUpdate()
				tx.Name = name
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				mockNSKeeper.EXPECT().Get(tx.Name).Return(state.BareNamespace())

				err = validation.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:namespace not found"))
			})
		})

		When("sender not owner of target namespace", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceDomainUpdate()
				tx.Name = name
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				key2 := crypto.NewKeyFromIntSeed(2)
				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{
					GraceEndAt: 10,
					Owner:      key2.Addr().String(),
				})

				err = validation.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:senderPubKey, msg:sender not permitted to perform this operation'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender not permitted to perform this operation"))
			})
		})

		When("balance sufficiency dry-run fails", func() {
			BeforeEach(func() {
				name := "name1"
				tx := txns.NewBareTxNamespaceDomainUpdate()
				tx.Name = name
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				mockNSKeeper.EXPECT().Get(tx.Name).Return(&state.Namespace{
					GraceEndAt: 9,
					Owner:      key.Addr().String(),
				})

				mockLogic.EXPECT().DrySend(key.PubKey(), util.String("0"), tx.Fee, tx.Nonce, uint64(0)).Return(fmt.Errorf("error"))
				err = validation.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxPushConsistency", func() {

		When("repository does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(state.BareRepository())
				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("repo not found"))
			})
		})

		When("unable to get top hosts", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				repo := state.BareRepository()
				repo.Balance = "100"
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)
				params.NumTopHostsLimit = 10
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(nil, fmt.Errorf("error"))
				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("an endorsement signer is not among the top hosts", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10
				hosts := []*tickettypes.SelectedTicket{{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}}}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
				})

				repo := state.BareRepository()
				repo.References["refs/heads/master"] = &state.Reference{}
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.senderPubKey, msg:sender public key does not belong to an active host"))
			})
		})

		When("an endorsement has invalid BLS public key", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10

				hosts := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: []byte("invalid")}},
				}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Note.(*types.Note).References = append(tx.Note.(*types.Note).References, &types.PushedReference{Name: "refs/heads/master"})
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					References:     []*types.EndorsedReference{{Hash: util.RandBytes(20)}},
				})

				repo := state.BareRepository()
				repo.References["refs/heads/master"] = &state.Reference{}
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("an endorsement hash is different from the current reference hash on the repository state", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10

				hosts := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Note.(*types.Note).References = append(tx.Note.(*types.Note).References, &types.PushedReference{Name: "refs/heads/master"})
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					References:     []*types.EndorsedReference{{Hash: nil}},
				})

				repo := state.BareRepository()
				repo.Balance = "100"
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should proceed to aggregated signature verification and fail", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("could not verify aggregated endorsers' signature"))
			})
		})

		When("an endorsement has no hash and the reference does not exist in the repo state", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10

				hosts := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Note.(*types.Note).References = append(tx.Note.(*types.Note).References, &types.PushedReference{Name: "refs/heads/master"})
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					References:     []*types.EndorsedReference{{Hash: util.RandBytes(20)}},
				})

				repo := state.BareRepository()
				repo.References["refs/heads/master"] = &state.Reference{Hash: util.RandBytes(20)}
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("not the expected hash"))
			})
		})

		When("an endorsement's aggregated push signature is unset", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10

				hosts := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				refHash := util.RandBytes(20)
				tx := txns.NewBareTxPush()
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Note.(*types.Note).References = append(tx.Note.(*types.Note).References, &types.PushedReference{Name: "refs/heads/master"})
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					References:     []*types.EndorsedReference{{Hash: refHash}},
				})

				repo := state.BareRepository()
				repo.References["refs/heads/master"] = &state.Reference{Hash: refHash}
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("could not verify aggregated endorsers' signature"))
			})
		})

		When("an endorsement's aggregated push signature is invalid", func() {
			BeforeEach(func() {
				params.NumTopHostsLimit = 10

				hosts := []*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}
				mockTickMgr.EXPECT().GetTopHosts(params.NumTopHostsLimit).Return(hosts, nil)

				refHash := util.RandBytes(20)
				tx := txns.NewBareTxPush()
				tx.AggregatedSig = util.RandBytes(128)
				tx.Note.(*types.Note).RepoName = "repo1"
				tx.Note.(*types.Note).References = append(tx.Note.(*types.Note).References, &types.PushedReference{Name: "refs/heads/master"})
				tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
					NoteID:         []byte("note_id"),
					EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					References:     []*types.EndorsedReference{{Hash: refHash}},
				})

				repo := state.BareRepository()
				repo.References["refs/heads/master"] = &state.Reference{Hash: refHash}
				mockRepoKeeper.EXPECT().Get(tx.Note.(*types.Note).RepoName).Return(repo)

				err = validation.CheckTxPushConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("could not verify aggregated endorsers' signature"))
			})
		})
	})

	Describe(".CheckTxVoteConsistency", func() {
		When("repo is unknown", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				repo := state.BareRepository()
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, msg:repo not found"))
			})
		})

		When("repo does not include the proposal", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.ProposalID = "proposal_xyz"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, msg:proposal not found"))
			})
		})

		When("the proposal has been finalized/concluded", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.ProposalID = "proposal1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{Outcome: 1})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, msg:proposal has concluded"))
			})
		})

		When("unable to get current block info", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config: repo.Config.Gov,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))

				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("a proposal is in proposal deposit fee period", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config:          repo.Config.Gov,
					FeeDepositEndAt: 100,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:proposal is currently in fee deposit period"))
			})
		})

		When("a proposal has fee deposit enabled but the total deposited fee is below the proposal minimum", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Config.Gov.PropFee = 200
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config:          repo.Config.Gov,
					FeeDepositEndAt: 100,
					Fees:            map[string]string{},
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 101}, nil)

				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:total deposited proposal fee is insufficient"))
			})
		})

		When("unable to get indexed proposal vote", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config: repo.Config.Gov,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

				mockRepoKeeper.EXPECT().GetProposalVote(tx.RepoName, tx.ProposalID,
					key.Addr().String()).Return(0, false, fmt.Errorf("error"))
				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to check proposal vote: error"))
			})
		})

		When("sender already voted on the proposal", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config: repo.Config.Gov,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

				mockRepoKeeper.EXPECT().GetProposalVote(tx.RepoName, tx.ProposalID,
					key.Addr().String()).Return(0, true, nil)
				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:vote already cast on the target proposal"))
			})
		})

		When("sender is not an owner of a repo whose proposal is targetted at repo owners", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ProposalID = "proposal1"

				repo := state.BareRepository()
				repo.Config.Gov.Voter = state.VoterOwner
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config: repo.Config.Gov,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

				err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender is not one of the repo owners"))
			})
		})

		When("sender is an owner of a repo whose proposal is targetted at repo owners", func() {
			When("sender has no veto right but votes NoWithVeto", func() {
				BeforeEach(func() {
					tx := txns.NewBareRepoProposalVote()
					tx.RepoName = "repo1"
					tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					tx.ProposalID = "proposal1"
					tx.Vote = state.ProposalVoteNoWithVeto

					repo := state.BareRepository()
					repo.AddOwner(key.Addr().String(), &state.RepoOwner{})
					repo.Config.Gov.Voter = state.VoterOwner
					repo.Proposals.Add("proposal1", &state.RepoProposal{
						Config: repo.Config.Gov,
					})
					mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

					err = validation.CheckTxVoteConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender cannot vote 'no with veto' because they have no veto right"))
				})
			})
		})
	})

	Describe(".CheckTxRepoProposalSendFeeConsistency", func() {
		When("repo is unknown", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				repo := state.BareRepository()
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, msg:repo not found"))
			})
		})

		When("repo does not include the proposal", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.ID = "proposal_xyz"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, msg:proposal not found"))
			})
		})

		When("the proposal has been finalized/concluded", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.ID = "proposal1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{Outcome: 1})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, msg:proposal has concluded"))
			})
		})

		When("unable to get current block info", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config: repo.Config.Gov,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))

				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("fee deposit is not enabled for a proposal", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config:          repo.Config.Gov,
					FeeDepositEndAt: 0,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 50}, nil)

				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:fee deposit not enabled for the proposal"))
			})
		})

		When("a proposal is not in proposal fee deposit period", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config:          repo.Config.Gov,
					FeeDepositEndAt: 100,
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)

				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:proposal fee deposit period has closed"))
			})
		})

		When("failed value transfer dry-run", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalFeeSend()
				tx.RepoName = "repo1"
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.ID = "proposal1"

				repo := state.BareRepository()
				repo.Proposals.Add("proposal1", &state.RepoProposal{
					Config:          repo.Config.Gov,
					FeeDepositEndAt: 100,
				})

				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(repo)
				bi := &state.BlockInfo{Height: 10}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockLogic.EXPECT().DrySend(key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validation.CheckTxRepoProposalSendFeeConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})
	})

	Describe(".CheckProposalCommonConsistency", func() {
		When("repo is unknown", func() {
			BeforeEach(func() {
				txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
				txCommon := &txns.TxCommon{}
				txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				repo := state.BareRepository()

				mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
				_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, msg:repo not found"))
			})
		})

		When("proposal with matching ID exist", func() {
			BeforeEach(func() {
				txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
				txCommon := &txns.TxCommon{}
				txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				txProposal.ID = "1"
				repo := state.BareRepository()
				repo.Proposals[txProposal.ID] = &state.RepoProposal{EndAt: 1000}

				mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
				_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:proposal id has been used, choose another"))
			})
		})

		When("proposal does not need a proposal fee but it is set", func() {
			BeforeEach(func() {
				txProposal := &txns.TxProposalCommon{RepoName: "repo1", ID: "1", Value: "10"}
				txCommon := &txns.TxCommon{}
				txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				repo := state.BareRepository()
				repo.Balance = "100"
				repo.Config.Gov.PropFee = 0

				mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
				_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:" + constants.ErrProposalFeeNotExpected.Error()))
			})
		})

		When("proposal fee is less than repo minimum", func() {
			BeforeEach(func() {
				txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
				txCommon := &txns.TxCommon{}
				txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				txProposal.Value = "10"
				repo := state.BareRepository()
				repo.Config.Gov.PropFee = 100

				mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
				_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:value, msg:proposal fee cannot be less than repo minimum (100.000000)"))
			})
		})

		When("repo config allows only owners to create proposals", func() {
			When("sender is not one of the repo owners", func() {
				BeforeEach(func() {
					txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
					txCommon := &txns.TxCommon{}
					txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					txProposal.Value = "101"
					repo := state.BareRepository()
					repo.Config.Gov.PropCreator = state.ProposalCreatorOwner
					repo.Config.Gov.PropFee = 100
					repo.Config.Gov.Voter = state.VoterOwner

					mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
					_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("field:senderPubKey, msg:sender is not permitted to create proposal"))
				})
			})

			When("sender is one of the repo owners", func() {
				BeforeEach(func() {
					txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
					txCommon := &txns.TxCommon{}
					txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
					txProposal.Value = "101"
					repo := state.BareRepository()
					repo.Config.Gov.PropCreator = state.ProposalCreatorOwner
					repo.Config.Gov.PropFee = 100
					repo.Config.Gov.Voter = state.VoterOwner
					repo.Owners[key.Addr().String()] = &state.RepoOwner{}

					mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
					mockLogic.EXPECT().DrySend(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
				})

				It("should return no error", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		When("failed value transfer dry-run", func() {
			BeforeEach(func() {
				txProposal := &txns.TxProposalCommon{RepoName: "repo1"}
				txCommon := &txns.TxCommon{}
				txCommon.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				txProposal.Value = "101"
				repo := state.BareRepository()
				repo.Config.Gov.PropFee = 100
				repo.Config.Gov.Voter = state.VoterOwner
				repo.Owners[key.Addr().String()] = &state.RepoOwner{}

				mockRepoKeeper.EXPECT().Get(txProposal.RepoName).Return(repo)
				mockLogic.EXPECT().DrySend(key.PubKey(), txProposal.Value, txCommon.Fee, txCommon.Nonce, uint64(0)).Return(fmt.Errorf("error"))

				_, err = validation.CheckProposalCommonConsistency(txProposal, txCommon, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})
	})

	Describe(".CheckTxRepoProposalUpsertOwnerConsistency", func() {
		When("target repo does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalUpsertOwner()
				tx.RepoName = "unknown"
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(state.BareRepository())
				err = validation.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:repo not found"))
			})
		})
	})

	Describe(".CheckTxRepoProposalUpdateConsistency", func() {
		When("target repo does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalUpdate()
				tx.RepoName = "unknown"
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(state.BareRepository())
				err = validation.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:repo not found"))
			})
		})
	})

	Describe(".CheckTxRepoProposalRegisterPushKeyConsistency()", func() {
		When("namespace is set but does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalRegisterPushKey()
				tx.Namespace = "ns1"
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.Namespace)).Return(state.BareNamespace())
				err = validation.CheckTxRepoProposalRegisterPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:namespace, msg:namespace not found"))
			})
		})

		When("namespaceOnly is set but does not exist", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalRegisterPushKey()
				tx.NamespaceOnly = "ns1"
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.NamespaceOnly)).Return(state.BareNamespace())
				err = validation.CheckTxRepoProposalRegisterPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:namespaceOnly, msg:namespace not found"))
			})
		})

		When("namespace is not owned by the target repo", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalRegisterPushKey()
				tx.RepoName = "repo1"
				tx.Namespace = "ns1"
				ns := state.BareNamespace()
				ns.Owner = "repo2"
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.Namespace)).Return(ns)
				err = validation.CheckTxRepoProposalRegisterPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:namespace, msg:namespace not owned by the target repository"))
			})
		})

		When("namespace is not owned by the target repo", func() {
			BeforeEach(func() {
				tx := txns.NewBareRepoProposalRegisterPushKey()
				tx.RepoName = "repo1"
				tx.Namespace = "ns1"
				ns := state.BareNamespace()
				ns.Owner = "repo1"
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.Namespace)).Return(ns)
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(state.BareRepository())
				err = validation.CheckTxRepoProposalRegisterPushKeyConsistency(tx, -1, mockLogic)
			})

			It("should not return err='namespace not owned by the target repository'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).ToNot(MatchError("field:namespace, msg:namespace not owned by the target repository"))
			})
		})
	})
})
