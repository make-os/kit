package validators_test

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/makeos/mosdef/params"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/validators"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxValidator", func() {
	var key = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockObjects *testutil.MockObjects
	var mockLogic *mocks.MockLogic
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockTxLogic *mocks.MockTxLogic
	var mockTickMgr *mocks.MockTicketManager
	var mockSysLogic *mocks.MockSysLogic
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockGPGPubKeyKeeper *mocks.MockGPGPubKeyKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)

		ctrl = gomock.NewController(GinkgoT())
		mockObjects = testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockSysKeeper = mockObjects.SysKeeper
		mockTxLogic = mockObjects.Tx
		mockTickMgr = mockObjects.TicketManager
		mockSysLogic = mockObjects.Sys
		mockRepoKeeper = mockObjects.RepoKeeper
		mockGPGPubKeyKeeper = mockObjects.GPGPubKeyKeeper
		mockNSKeeper = mockObjects.NamespaceKeeper
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxCoinTransferConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxCoinTransfer()
				err = validators.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("recipient address is a repo address of which the repo does not exist", func() {
			BeforeEach(func() {
				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				tx := types.NewBareTxCoinTransfer()
				tx.To = "r/repo"
				mockRepoKeeper.EXPECT().GetRepo("repo", uint64(1)).Return(types.BareRepository())
				err = validators.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:recipient repo not found"))
			})
		})

		When("recipient address is a namespaced address of which the target is a repo address "+
			"pointing to repo that does not exist", func() {
			BeforeEach(func() {
				bi := &types.BlockInfo{Height: 1}
				tx := types.NewBareTxCoinTransfer()
				tx.To = "namespace/cool-repo"

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockNSKeeper.EXPECT().GetTarget(tx.To.String(), uint64(1)).Return("r/repo", nil)
				mockRepoKeeper.EXPECT().GetRepo("repo", uint64(1)).Return(types.BareRepository())

				err = validators.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:recipient repo not found"))
			})
		})

		When("recipient address is a namespaced address of which the namespace could not be found", func() {
			BeforeEach(func() {
				bi := &types.BlockInfo{Height: 1}
				tx := types.NewBareTxCoinTransfer()
				tx.To = "namespace/cool-repo"

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockNSKeeper.EXPECT().GetTarget(tx.To.String(), uint64(1)).Return("", fmt.Errorf("error"))

				err = validators.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:error"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxCoinTransfer()
				tx.Value = "10.2"
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))
				err = validators.CheckTxCoinTransferConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxTicketPurchaseConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
				err = validators.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("delegate is set", func() {
			var delegate = crypto.NewKeyFromIntSeed(1)

			When("unable to get active ticket of delegate", func() {
				BeforeEach(func() {
					tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
					tx.Delegate = delegate.PubKey().MustBytes32()

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return(nil, fmt.Errorf("error"))

					err = validators.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("failed to get active delegate tickets: error"))
				})
			})

			When("delegate has no active ticket", func() {
				BeforeEach(func() {
					tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
					tx.Delegate = delegate.PubKey().MustBytes32()

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return([]*types.Ticket{}, nil)

					err = validators.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:delegate, error:specified delegate is not active"))
				})
			})

			When("for validator ticket - ticket price is less than current ticket price", func() {
				BeforeEach(func() {
					tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
					tx.Delegate = delegate.PubKey().MustBytes32()

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return([]*types.Ticket{&types.Ticket{}}, nil)
					mockSysLogic.EXPECT().GetCurValidatorTicketPrice().Return(10.4)

					err = validators.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:value, error:value is lower than the minimum ticket price (10.400000)"))
				})
			})

			When("coin transfer dry-run fails", func() {
				BeforeEach(func() {
					tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
					tx.Value = "10.5"
					tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
					tx.Delegate = delegate.PubKey().MustBytes32()

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetNonDelegatedTickets(delegate.PubKey().MustBytes32(), tx.Type).
						Return([]*types.Ticket{&types.Ticket{}}, nil)
					mockSysLogic.EXPECT().GetCurValidatorTicketPrice().Return(10.4)
					mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
						tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

					err = validators.CheckTxTicketPurchaseConsistency(tx, -1, mockLogic)
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
				tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
				err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("target ticket does not exist", func() {
			BeforeEach(func() {
				tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
				tx.TicketHash = util.StrToBytes32("ticket_hash")

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(nil)

				err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, error:ticket not found"))
			})
		})

		When("ticket is not delegated", func() {
			When("sender is not the ticket proposer", func() {
				BeforeEach(func() {
					key2 := crypto.NewKeyFromIntSeed(2)
					tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
					tx.TicketHash = util.StrToBytes32("ticket_hash")
					tx.SetSenderPubKey(key2.PubKey().MustBytes())

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &types.Ticket{ProposerPubKey: key.PubKey().MustBytes32()}
					mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

					err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:hash, error:sender not authorized to unbond this ticket"))
				})
			})
		})

		When("ticket is delegated", func() {
			When("sender is not the delegator", func() {
				BeforeEach(func() {
					key2 := crypto.NewKeyFromIntSeed(2)
					tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
					tx.TicketHash = util.StrToBytes32("ticket_hash")
					tx.SetSenderPubKey(key2.PubKey().MustBytes())

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						Delegator:      key.Addr().String(),
					}
					mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

					err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:hash, error:sender not authorized to unbond this ticket"))
				})
			})
		})

		When("ticket decay height is set and greater than current block height", func() {
			BeforeEach(func() {
				tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
				tx.TicketHash = util.StrToBytes32("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 50}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					DecayBy:        100,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, error:ticket is already decaying"))
			})
		})

		When("ticket decay height is set less than current block height", func() {
			BeforeEach(func() {
				tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
				tx.TicketHash = util.StrToBytes32("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					DecayBy:        100,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:hash, error:ticket has already decayed"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
				tx.TicketHash = util.StrToBytes32("ticket_hash")
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().MustBytes32(),
					DecayBy:        0,
				}
				mockTickMgr.EXPECT().GetByHash(tx.TicketHash).Return(ticket)

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					util.String("0"), tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxUnbondTicketConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxRepoCreateConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxRepoCreate()
				err = validators.CheckTxRepoCreateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("repo name is not unique", func() {
			BeforeEach(func() {
				tx := types.NewBareTxRepoCreate()
				tx.Name = "repo1"

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				repo := types.BareRepository()
				repo.AddOwner("some_address", &types.RepoOwner{})
				mockRepoKeeper.EXPECT().GetRepo(tx.Name).Return(repo)

				err = validators.CheckTxRepoCreateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:name is not available. choose another"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxRepoCreate()
				tx.Name = "repo1"
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				repo := types.BareRepository()
				mockRepoKeeper.EXPECT().GetRepo(tx.Name).Return(repo)

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxRepoCreateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxSetDelegateCommissionConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxSetDelegateCommission()
				err = validators.CheckTxSetDelegateCommissionConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxSetDelegateCommission()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					util.String("0"), tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxSetDelegateCommissionConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxAddGPGPubKeyConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxAddGPGPubKey()
				err = validators.CheckTxAddGPGPubKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("gpg public key is less than 2048 bits", func() {
			BeforeEach(func() {
				tx := types.NewBareTxAddGPGPubKey()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				var bz []byte
				bz, err = ioutil.ReadFile("./testdata/gpgkey1024.pub")
				Expect(err).To(BeNil())
				tx.PublicKey = string(bz)

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				err = validators.CheckTxAddGPGPubKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, error:gpg public key bit length must be at least 2048 bits"))
			})
		})

		When("gpg public key has already been registered", func() {
			BeforeEach(func() {
				tx := types.NewBareTxAddGPGPubKey()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				var bz []byte
				bz, err = ioutil.ReadFile("./testdata/gpgkey.pub")
				Expect(err).To(BeNil())
				tx.PublicKey = string(bz)

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				entity, _ := crypto.PGPEntityFromPubKey(tx.PublicKey)
				pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				mockGPGPubKeyKeeper.EXPECT().GetGPGPubKey(pkID).Return(&types.GPGPubKey{PubKey: tx.PublicKey})

				err = validators.CheckTxAddGPGPubKeyConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, error:gpg public key already registered"))
			})
		})

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxAddGPGPubKey()
				tx.SetSenderPubKey(key.PubKey().MustBytes())

				var bz []byte
				bz, err = ioutil.ReadFile("./testdata/gpgkey.pub")
				Expect(err).To(BeNil())
				tx.PublicKey = string(bz)

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				entity, _ := crypto.PGPEntityFromPubKey(tx.PublicKey)
				pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				mockGPGPubKeyKeeper.EXPECT().GetGPGPubKey(pkID).Return(&types.GPGPubKey{})

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					util.String("0"), tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxAddGPGPubKeyConsistency(tx, -1, mockLogic)

			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxNSAcquireConsistency", func() {

		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxNamespaceAcquire()
				err = validators.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("target namespace exist and not expired", func() {
			BeforeEach(func() {
				name := "name1"
				tx := types.NewBareTxNamespaceAcquire()
				tx.Name = name

				bi := &types.BlockInfo{Height: 9}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				mockNSKeeper.EXPECT().GetNamespace(tx.Name).Return(&types.Namespace{
					GraceEndAt: 10,
				})
				err = validators.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:name, error:chosen name is not currently available'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:chosen name is not currently available"))
			})
		})

		When("balance sufficiency dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxNamespaceAcquire()
				tx.Value = "10.2"
				tx.Name = "name1"
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())

				bi := &types.BlockInfo{Height: 10}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				mockNSKeeper.EXPECT().GetNamespace(tx.Name).Return(&types.Namespace{
					GraceEndAt: 9,
				})

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))
				err = validators.CheckTxNSAcquireConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxNamespaceDomainUpdateConsistency", func() {
		When("unable to get last block information", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := types.NewBareTxNamespaceDomainUpdate()
				err = validators.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("sender not owner of target namespace", func() {
			BeforeEach(func() {
				name := "name1"
				tx := types.NewBareTxNamespaceDomainUpdate()
				tx.Name = name
				tx.SenderPubKey = key.PubKey().MustBytes32()

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				key2 := crypto.NewKeyFromIntSeed(2)
				mockNSKeeper.EXPECT().GetNamespace(tx.Name).Return(&types.Namespace{
					GraceEndAt: 10,
					Owner:      key2.Addr().String(),
				})

				err = validators.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err='field:senderPubKey, error:sender not permitted to perform this operation'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender not permitted to perform this operation"))
			})
		})

		When("balance sufficiency dry-run fails", func() {
			BeforeEach(func() {
				name := "name1"
				tx := types.NewBareTxNamespaceDomainUpdate()
				tx.Name = name
				tx.SenderPubKey = key.PubKey().MustBytes32()

				bi := &types.BlockInfo{Height: 10}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)

				mockNSKeeper.EXPECT().GetNamespace(tx.Name).Return(&types.Namespace{
					GraceEndAt: 9,
					Owner:      key.Addr().String(),
				})

				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(), util.String("0"), tx.Fee,
					tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))
				err = validators.CheckTxNamespaceDomainUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})

	Describe(".CheckTxPushConsistency", func() {

		When("repository does not exist or retrieval failed", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
				})

				repoGetter := func(name string) (types.BareRepo, error) {
					return nil, fmt.Errorf("error")
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get repo: error"))
			})
		})

		When("unable to get top storers", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(nil, fmt.Errorf("error"))

				tx := types.NewBareTxPush()
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
				})

				repoGetter := func(name string) (types.BareRepo, error) {
					return nil, nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top storers: error"))
			})
		})

		When("a PushOK signer is not among the top storers", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10
				storers := []*types.SelectedTicket{
					&types.SelectedTicket{Ticket: &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
					}},
				}

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(storers, nil)

				tx := types.NewBareTxPush()
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
				})

				repoGetter := func(name string) (types.BareRepo, error) {
					return nil, nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.senderPubKey, error:sender public key does not belong to an active storer"))
			})
		})

		When("a PushOK has invalid BLS public key", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10
				storers := []*types.SelectedTicket{
					&types.SelectedTicket{Ticket: &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						BLSPubKey:      []byte("invalid"),
					}},
				}

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(storers, nil)

				tx := types.NewBareTxPush()
				tx.PushNote.References = append(tx.PushNote.References, &types.PushedReference{
					Name: "refs/heads/master",
				})
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				})

				repoGetter := func(name string) (types.BareRepo, error) {
					return mocks.NewMockBareRepo(ctrl), nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("unable to get reference tree", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10
				storers := []*types.SelectedTicket{
					&types.SelectedTicket{Ticket: &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						BLSPubKey:      key.PrivKey().BLSKey().Public().Bytes(),
					}},
				}

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(storers, nil)

				tx := types.NewBareTxPush()
				tx.PushNote.References = append(tx.PushNote.References, &types.PushedReference{
					Name: "refs/heads/master",
				})

				pok := &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				blsSig, _ := key.PrivKey().BLSKey().Sign(pok.BytesNoSig())
				pok.Sig = util.BytesToBytes64(blsSig)
				tx.PushOKs = append(tx.PushOKs, pok)

				mockBareRepo := mocks.NewMockBareRepo(ctrl)
				mockBareRepo.EXPECT().TreeRoot("refs/heads/master").Return(util.EmptyBytes32, fmt.Errorf("error"))
				repoGetter := func(name string) (types.BareRepo, error) {
					return mockBareRepo, nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get reference (refs/heads/master) tree root hash: error"))
			})
		})

		When("a PushOK reference has a hash that does not match the local reference hash", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10
				storers := []*types.SelectedTicket{
					&types.SelectedTicket{Ticket: &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						BLSPubKey:      key.PrivKey().BLSKey().Public().Bytes(),
					}},
				}

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(storers, nil)

				tx := types.NewBareTxPush()
				tx.PushNote.References = append(tx.PushNote.References, &types.PushedReference{
					Name: "refs/heads/master",
				})

				pok := &types.PushOK{
					PushNoteID:   util.StrToBytes32("pn1"),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				blsSig, _ := key.PrivKey().BLSKey().Sign(pok.BytesNoSig())
				pok.Sig = util.BytesToBytes64(blsSig)
				tx.PushOKs = append(tx.PushOKs, pok)

				mockBareRepo := mocks.NewMockBareRepo(ctrl)
				mockBareRepo.EXPECT().TreeRoot("refs/heads/master").Return(util.BytesToBytes32(util.RandBytes(32)), nil)
				repoGetter := func(name string) (types.BareRepo, error) {
					return mockBareRepo, nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.refsHash, error:wrong tree hash for reference (refs/heads/master)"))
			})
		})

		When("aggregated signature is invalid", func() {
			BeforeEach(func() {
				params.NumTopStorersLimit = 10
				storers := []*types.SelectedTicket{
					&types.SelectedTicket{Ticket: &types.Ticket{
						ProposerPubKey: key.PubKey().MustBytes32(),
						BLSPubKey:      key.PrivKey().BLSKey().Public().Bytes(),
					}},
				}

				mockTickMgr.EXPECT().GetTopStorers(params.NumTopStorersLimit).Return(storers, nil)

				tx := types.NewBareTxPush()

				pok := &types.PushOK{
					PushNoteID:     util.StrToBytes32("pn1"),
					SenderPubKey:   util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{},
				}
				pok.Sig = util.BytesToBytes64(util.RandBytes(64))
				tx.PushOKs = append(tx.PushOKs, pok)

				mockBareRepo := mocks.NewMockBareRepo(ctrl)
				repoGetter := func(name string) (types.BareRepo, error) {
					return mockBareRepo, nil
				}

				err = validators.CheckTxPushConsistency(tx, -1, mockLogic, repoGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("could not verify aggregated endorsers' signature"))
			})
		})

	})

	Describe(".CheckTxRepoProposalUpsertOwnerConsistency", func() {

		When("unable to get current block information", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpsertOwner()
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				err = validators.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("repo is unknown", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpsertOwner()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				repo := types.BareRepository()

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, error:repo not found"))
			})
		})

		When("proposal fee is less than repo minimum", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpsertOwner()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "10"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:value, error:proposal fee cannot be less than repo minimum"))
			})
		})

		When("sender is not one of the repo owners", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpsertOwner()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "101"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100
				repo.Config.Governace.ProposalProposee = types.ProposeeOwner

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:senderPubKey, error:sender is not one of the repo owners"))
			})
		})

		When("failed value transfer dry-run", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpsertOwner()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "101"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100
				repo.Config.Governace.ProposalProposee = types.ProposeeOwner
				repo.Owners[key.Addr().String()] = &types.RepoOwner{}

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxRepoProposalUpsertOwnerConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})
	})

	Describe(".CheckTxVoteConsistency", func() {
		When("repo is unknown", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				repo := types.BareRepository()
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)
				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, error:repo not found"))
			})
		})

		When("repo does not include the proposal", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.ProposalID = "proposal_xyz"
				tx.SenderPubKey = key.PubKey().MustBytes32()

				repo := types.BareRepository()
				repo.Proposals.Add("proposal1", &types.RepoProposal{})
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)
				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, error:proposal not found"))
			})
		})

		When("the proposal has been finalized/concluded", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.ProposalID = "proposal1"
				tx.SenderPubKey = key.PubKey().MustBytes32()

				repo := types.BareRepository()
				repo.Proposals.Add("proposal1", &types.RepoProposal{Outcome: 1})
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)
				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:id, error:proposal voting period has ended"))
			})
		})

		When("unable to get indexed proposal vote", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.ProposalID = "proposal1"

				repo := types.BareRepository()
				repo.Proposals.Add("proposal1", &types.RepoProposal{
					Config: repo.Config.Governace,
				})
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)

				mockRepoKeeper.EXPECT().GetProposalVote(tx.RepoName, tx.ProposalID,
					key.Addr().String()).Return(0, false, fmt.Errorf("error"))
				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to check proposal vote: error"))
			})
		})

		When("sender already voted on the proposal", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.ProposalID = "proposal1"

				repo := types.BareRepository()
				repo.Proposals.Add("proposal1", &types.RepoProposal{
					Config: repo.Config.Governace,
				})
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)

				mockRepoKeeper.EXPECT().GetProposalVote(tx.RepoName, tx.ProposalID,
					key.Addr().String()).Return(0, true, nil)
				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, error:vote already cast on the target proposal"))
			})
		})

		When("sender is not an owner of a repo whose proposal is targetted at repo owners", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalVote()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.ProposalID = "proposal1"

				repo := types.BareRepository()
				repo.Config.Governace.ProposalProposee = types.ProposeeOwner
				repo.Proposals.Add("proposal1", &types.RepoProposal{
					Config: repo.Config.Governace,
				})
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)

				err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender is not one of the repo owners"))
			})
		})

		When("sender is an owner of a repo whose proposal is targetted at repo owners", func() {
			When("sender has no veto right but votes NoWithVeto", func() {
				BeforeEach(func() {
					tx := types.NewBareRepoProposalVote()
					tx.RepoName = "repo1"
					tx.SenderPubKey = key.PubKey().MustBytes32()
					tx.ProposalID = "proposal1"
					tx.Vote = types.ProposalVoteNoWithVeto

					repo := types.BareRepository()
					repo.AddOwner(key.Addr().String(), &types.RepoOwner{})
					repo.Config.Governace.ProposalProposee = types.ProposeeOwner
					repo.Proposals.Add("proposal1", &types.RepoProposal{
						Config: repo.Config.Governace,
					})
					mockRepoKeeper.EXPECT().GetRepo(tx.RepoName).Return(repo)

					err = validators.CheckTxVoteConsistency(tx, -1, mockLogic)
				})

				It("should return err", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:senderPubKey, error:sender cannot vote 'no with veto' because they have no veto right"))
				})
			})
		})
	})

	Describe(".CheckTxRepoProposalUpdateConsistency", func() {

		When("unable to get current block information", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpdate()
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				err = validators.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("repo is unknown", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpdate()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				repo := types.BareRepository()

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:name, error:repo not found"))
			})
		})

		When("proposal fee is less than repo minimum", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpdate()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "10"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:value, error:proposal fee cannot be less than repo minimum"))
			})
		})

		When("sender is not one of the repo owners", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpdate()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "101"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100
				repo.Config.Governace.ProposalProposee = types.ProposeeOwner

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				err = validators.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:senderPubKey, error:sender is not one of the repo owners"))
			})
		})

		When("failed value transfer dry-run", func() {
			BeforeEach(func() {
				tx := types.NewBareRepoProposalUpdate()
				tx.RepoName = "repo1"
				tx.SenderPubKey = key.PubKey().MustBytes32()
				tx.Value = "101"
				repo := types.BareRepository()
				repo.Config.Governace.ProposalFee = 100
				repo.Config.Governace.ProposalProposee = types.ProposeeOwner
				repo.Owners[key.Addr().String()] = &types.RepoOwner{}

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
				mockRepoKeeper.EXPECT().GetRepo(tx.RepoName, uint64(bi.Height)).Return(repo)
				mockTxLogic.EXPECT().CanExecCoinTransfer(tx.Type, key.PubKey(),
					tx.Value, tx.Fee, tx.Nonce, uint64(bi.Height)).Return(fmt.Errorf("error"))

				err = validators.CheckTxRepoProposalUpdateConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})
	})
})
