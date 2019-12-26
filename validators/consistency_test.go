package validators_test

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"

	randmocks "github.com/makeos/mosdef/crypto/rand/mocks"

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
	var mocRepoKeeper *mocks.MockRepoKeeper
	var mockDrand *randmocks.MockDRander
	var mockGPGPubKeyKeeper *mocks.MockGPGPubKeyKeeper

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
		mocRepoKeeper = mockObjects.RepoKeeper
		mockDrand = mockObjects.Drand
		mockGPGPubKeyKeeper = mockObjects.GPGPubKeyKeeper
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

		When("coin transfer dry-run fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxCoinTransfer()
				tx.Value = "10.2"
				tx.SenderPubKey = key.PubKey().Base58()
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
			When("unable to get active ticket of delegate", func() {
				BeforeEach(func() {
					tx := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
					tx.Value = "10.2"
					tx.SenderPubKey = key.PubKey().Base58()
					tx.Delegate = "delegate_pub_key"

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetActiveTicketsByProposer(tx.Delegate, tx.Type, false).
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
					tx.SenderPubKey = key.PubKey().Base58()
					tx.Delegate = "delegate_pub_key"

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetActiveTicketsByProposer(tx.Delegate, tx.Type, false).
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
					tx.SenderPubKey = key.PubKey().Base58()
					tx.Delegate = "delegate_pub_key"

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetActiveTicketsByProposer(tx.Delegate, tx.Type, false).
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
					tx.SenderPubKey = key.PubKey().Base58()
					tx.Delegate = "delegate_pub_key"

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					mockTickMgr.EXPECT().GetActiveTicketsByProposer(tx.Delegate, tx.Type, false).
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
				tx.TicketHash = "ticket_hash"

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
					tx.TicketHash = "ticket_hash"
					tx.SetSenderPubKey(key2.PubKey().Base58())

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &types.Ticket{ProposerPubKey: key.PubKey().Base58()}
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
					tx.TicketHash = "ticket_hash"
					tx.SetSenderPubKey(key2.PubKey().Base58())

					bi := &types.BlockInfo{Height: 1}
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
					ticket := &types.Ticket{
						ProposerPubKey: key.PubKey().Base58(),
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
				tx.TicketHash = "ticket_hash"
				tx.SetSenderPubKey(key.PubKey().Base58())

				bi := &types.BlockInfo{Height: 50}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().Base58(),
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
				tx.TicketHash = "ticket_hash"
				tx.SetSenderPubKey(key.PubKey().Base58())

				bi := &types.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().Base58(),
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
				tx.TicketHash = "ticket_hash"
				tx.SetSenderPubKey(key.PubKey().Base58())

				bi := &types.BlockInfo{Height: 101}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				ticket := &types.Ticket{
					ProposerPubKey: key.PubKey().Base58(),
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
				repo := &types.Repository{CreatorAddress: key.Addr()}
				mocRepoKeeper.EXPECT().GetRepo(tx.Name).Return(repo)

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
				tx.SetSenderPubKey(key.PubKey().Base58())

				bi := &types.BlockInfo{Height: 1}
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(bi, nil)
				repo := &types.Repository{}
				mocRepoKeeper.EXPECT().GetRepo(tx.Name).Return(repo)

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

	Describe(".CheckTxEpochSecretConsistency", func() {
		When("secret validation fail", func() {
			BeforeEach(func() {
				tx := types.NewBareTxEpochSecret()
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1000

				mockDrand.EXPECT().Verify(tx.Secret, tx.PreviousSecret,
					tx.SecretRound).Return(fmt.Errorf("error"))

				err = validators.CheckTxEpochSecretConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:secret, error:epoch secret is invalid"))
			})
		})

		When("unable to get highest drand round", func() {
			BeforeEach(func() {
				tx := types.NewBareTxEpochSecret()
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1000

				mockDrand.EXPECT().Verify(tx.Secret, tx.PreviousSecret,
					tx.SecretRound).Return(nil)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(0), fmt.Errorf("error"))

				err = validators.CheckTxEpochSecretConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get highest drand round: error"))
			})
		})

		When("secret round is not greater than highest drand round", func() {
			BeforeEach(func() {
				tx := types.NewBareTxEpochSecret()
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1000

				mockDrand.EXPECT().Verify(tx.Secret, tx.PreviousSecret,
					tx.SecretRound).Return(nil)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(1000), nil)

				err = validators.CheckTxEpochSecretConsistency(tx, -1, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:secretRound, error:must be greater than the previous round"))
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
				tx.SetSenderPubKey(key.PubKey().Base58())

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
				tx.SetSenderPubKey(key.PubKey().Base58())

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
				tx.SetSenderPubKey(key.PubKey().Base58())

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
				tx.SetSenderPubKey(key.PubKey().Base58())

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
})
