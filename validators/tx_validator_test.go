package validators_test

import (
	"fmt"
	"os"
	"time"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/params"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/validators"

	"github.com/makeos/mosdef/config"
	l "github.com/makeos/mosdef/logic"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/types"
)

type txCase struct {
	tx   *types.Transaction
	err  error
	desc string
}

var _ = Describe("TxValidator", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.EngineConfig
	var logic *l.Logic
	var ctrl *gomock.Controller
	var mockLogic *testutil.MockObjects

	validEpochSecretTx := types.NewBareTx(types.TxTypeEpochSecret)
	validEpochSecretTx.EpochSecret = &types.EpochSecret{}
	validEpochSecretTx.EpochSecret.SecretRound = 1000
	validEpochSecretTx.EpochSecret.Secret = []uint8{
		0x3a, 0x06, 0x2b, 0xf4, 0xac, 0x34, 0x57, 0x06, 0xcd, 0x41, 0x62, 0xa7, 0x25, 0x39, 0xb8, 0x4a,
		0x73, 0xf7, 0xf4, 0x1e, 0x57, 0x89, 0x88, 0xdc, 0x9f, 0xef, 0xc2, 0xd4, 0x5f, 0x80, 0xe2, 0xec,
		0x64, 0x9e, 0xdc, 0x53, 0xb7, 0x26, 0x4b, 0x0c, 0xdf, 0x41, 0xe3, 0x63, 0xb1, 0xb9, 0xf4, 0xcd,
		0x73, 0x0c, 0x35, 0xd3, 0xf6, 0x31, 0x78, 0x14, 0x24, 0xef, 0xa4, 0x3a, 0x79, 0x63, 0xf1, 0x01,
	}
	validEpochSecretTx.EpochSecret.PreviousSecret = []uint8{
		0x28, 0x18, 0x21, 0x0a, 0x81, 0xb6, 0x28, 0x88, 0xa9, 0x24, 0x29, 0x55, 0xf2, 0x01, 0x30, 0x80,
		0xa9, 0x7e, 0xa3, 0x55, 0x7c, 0x6d, 0xfe, 0x8a, 0x5d, 0x94, 0x0d, 0x8f, 0x65, 0x46, 0xdd, 0x99,
		0x69, 0xf2, 0xf9, 0x10, 0xd5, 0xcf, 0x15, 0xcc, 0x0e, 0x39, 0x17, 0xa8, 0xd9, 0x90, 0x21, 0x57,
		0x5e, 0x27, 0xdb, 0xfd, 0x25, 0x61, 0x54, 0xb1, 0x4d, 0xdc, 0xbf, 0xb1, 0xbf, 0xb4, 0x5e, 0x44,
	}

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = l.New(appDB, stateTreeDB, cfg)
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ValidateTxSyntax", func() {
		params.MinDelegatorCommission = decimal.NewFromFloat(10)
		var to = crypto.NewKeyFromIntSeed(1)
		var txMissingSignature = &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		var txInvalidSig = &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		txInvalidSig.Sig = []byte("invalid")

		var cases = []txCase{
			{tx: nil, desc: "nil is provided", err: fmt.Errorf("nil tx")},
			{tx: &types.Transaction{Type: 1000}, desc: "tx type is invalid", err: fmt.Errorf("field:type, error:unsupported transaction type")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, EpochSecret: &types.EpochSecret{}}, desc: "unexpected field `epochSecret` is set", err: fmt.Errorf("field:epochSecret, error:unexpected field")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: ""}, desc: "recipient not set", err: fmt.Errorf("field:to, error:recipient address is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: "abc"}, desc: "recipient not valid", err: fmt.Errorf("field:to, error:recipient address is not valid")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr()}, desc: "value not provided", err: fmt.Errorf("field:value, error:value is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "-1"}, desc: "value is negative", err: fmt.Errorf("field:value, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1"}, desc: "fee not provided", err: fmt.Errorf("field:fee, error:fee is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "-1"}, desc: "fee is negative", err: fmt.Errorf("field:fee, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "0.0000000001"}, desc: "fee lower than base price", err: fmt.Errorf("field:fee, error:fee cannot be lower than the base price of 0.0007")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1"}, desc: "timestamp not provided", err: fmt.Errorf("field:timestamp, error:timestamp is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix()}, desc: "sender pub key not provided", err: fmt.Errorf("field:senderPubKey, error:sender public key is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: "abc"}, desc: "sender pub key is not valid", err: fmt.Errorf("field:senderPubKey, error:sender public key is not valid")},
			{tx: txMissingSignature, desc: "signature not provided", err: fmt.Errorf("field:sig, error:signature is required")},
			{tx: txInvalidSig, desc: "signature not valid", err: fmt.Errorf("field:sig, error:signature is not valid")},

			// TxTypeValidatorTicket specific case
			{tx: &types.Transaction{Type: types.TxTypeValidatorTicket, To: "abc"}, desc: "recipient not a valid public key", err: fmt.Errorf("field:to, error:requires a valid public key to delegate to")},

			// TxTypeStorerTicket specific case
			{tx: &types.Transaction{Type: types.TxTypeStorerTicket, To: "abc"}, desc: "recipient not a valid public key", err: fmt.Errorf("field:to, error:requires a valid public key to delegate to")},

			// TxTypeSetDelegatorCommission specific cases
			{tx: &types.Transaction{Type: types.TxTypeSetDelegatorCommission, To: "abc"}, desc: "unexpected field `to` is set", err: fmt.Errorf("field:to, error:unexpected field")},
			{tx: &types.Transaction{Type: types.TxTypeSetDelegatorCommission, Value: "101"}, desc: "exceeded commission rate", err: fmt.Errorf("field:value, error:commission rate cannot exceed 100%%%%")},
			{tx: &types.Transaction{Type: types.TxTypeSetDelegatorCommission, Value: "1"}, desc: "below commission rate", err: fmt.Errorf("field:value, error:commission rate cannot be below the minimum (10%%%%)")},

			// TxTypeUnbondStorerTicket specific cases
			{tx: &types.Transaction{Type: types.TxTypeUnbondStorerTicket, Fee: "1", To: "addr"}, desc: "unexpected field `to` is set", err: fmt.Errorf("field:to, error:unexpected field")},
			{tx: &types.Transaction{Type: types.TxTypeUnbondStorerTicket, Fee: "1", Timestamp: 1}, desc: "ticket id not provided", err: fmt.Errorf("field:ticket, error:ticket id is required")},
		}

		for _, c := range cases {
			_c := c
			It(fmt.Sprintf("should return err=%s, when %s", _c.err.Error(), _c.desc), func() {
				err := validators.ValidateTxSyntax(_c.tx, -1)
				if err != nil {
					Expect(err.Error()).To(Equal(_c.err.Error()))
				} else {
					Expect(_c.err).To(BeNil())
				}
			})
		}
	})

	Describe(".ValidateTxs", func() {
		var txs = []*types.Transaction{
			&types.Transaction{Type: 1000},
		}

		It("should return err='index:0, field:type, error:unsupported transaction type' when tx at index:0 is invalid", func() {
			err := validators.ValidateTxs(txs, logic)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:type, error:unsupported transaction type"))
		})
	})

	Describe(".ValidateTxConsistency", func() {
		var key = crypto.NewKeyFromIntSeed(1)
		var key2 = crypto.NewKeyFromIntSeed(2)
		var err error

		BeforeEach(func() {
			params.MinStorerStake = decimal.NewFromFloat(1)
		})

		When("error occurred when getting current block height", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("bad error"))
				tx := &types.Transaction{Type: types.TxTypeCoinTransfer, To: key.Addr(),
					Value: "1", Fee: "1", Timestamp: time.Now().Unix(),
					SenderPubKey: util.String(key.PubKey().Base58())}
				err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: bad error"))
			})
		})

		When("tx type is TxTypeCoinTransfer", func() {
			When("an error occurred when performing coin transfer check", func() {
				BeforeEach(func() {
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.Tx.EXPECT().CanExecCoinTransfer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(fmt.Errorf("bad error"))

					tx := &types.Transaction{Type: types.TxTypeCoinTransfer, To: key.Addr(),
						Value: "1", Fee: "1", Timestamp: time.Now().Unix(),
						SenderPubKey: util.String(key.PubKey().Base58())}

					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("failed to check balance sufficiency: bad error"))
				})
			})
		})

		When("unable to get current block info", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				tx := &types.Transaction{}
				err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
			})

			It("should return err='failed to fetch current block info: error'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch current block info: error"))
			})
		})

		When("tx type is TxTypeSetDelegatorCommission", func() {

			When("balance check return error", func() {
				var err error
				BeforeEach(func() {
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.Tx.EXPECT().CanExecCoinTransfer(gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any()).Return(fmt.Errorf("error"))

					tx := &types.Transaction{Type: types.TxTypeSetDelegatorCommission}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return err='failed to check balance sufficiency: error'", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("failed to check balance sufficiency: error"))
				})
			})

			When("balance check returns nil", func() {
				var err error
				BeforeEach(func() {
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.Tx.EXPECT().CanExecCoinTransfer(gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any()).Return(nil)
					tx := &types.Transaction{Type: types.TxTypeSetDelegatorCommission}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return nil", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		When("tx type is TxTypeValidatorTicket", func() {
			When("failed to get list of active ticket by the tx target proposer/delegate", func() {
				BeforeEach(func() {
					params.MinStorerStake = decimal.NewFromFloat(1)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.TicketManager.EXPECT().GetActiveTicketsByProposer(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
					tx := &types.Transaction{Type: types.TxTypeValidatorTicket, To: key.Addr(), Value: "10", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return error='failed to get active delegate tickets: error'", func() {
					Expect(err.Error()).To(Equal("failed to get active delegate tickets: error"))
				})
			})

			When("target proposer/delegate has no active tickets", func() {
				BeforeEach(func() {
					tickets := []*types.Ticket{}
					mockLogic.TicketManager.EXPECT().GetActiveTicketsByProposer(gomock.Any(), gomock.Any(), gomock.Any()).Return(tickets, nil)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					tx := &types.Transaction{Type: types.TxTypeValidatorTicket, To: key.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return error", func() {
					Expect(err.Error()).To(Equal("field:to, error:the delegate is not active"))
				})
			})

			When("value is lower than the current validator ticket price", func() {
				BeforeEach(func() {
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.Sys.EXPECT().GetCurValidatorTicketPrice().Return(20.5)
					tx := &types.Transaction{Type: types.TxTypeValidatorTicket, To: "", Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return err='field:value, error:value is lower than the minimum ticket price (20.500000)'", func() {
					Expect(err.Error()).To(Equal("field:value, error:value is lower than the minimum ticket price (20.500000)"))
				})
			})
		})

		When("tx type is TxTypeStorerTicket", func() {
			When("tx value is less than the minimum ticket stake", func() {
				BeforeEach(func() {
					params.MinStorerStake = decimal.NewFromFloat(100)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, nil)
					tx := &types.Transaction{Type: types.TxTypeStorerTicket, Value: "0"}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return err='field:value, error:value is lower than minimum storer stake'", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:value, error:value is lower than minimum storer stake"))
				})
			})

			When("failed to get list of active ticket by the tx target proposer/delegate", func() {
				BeforeEach(func() {
					params.MinStorerStake = decimal.NewFromFloat(1)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					mockLogic.TicketManager.EXPECT().GetActiveTicketsByProposer(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
					tx := &types.Transaction{Type: types.TxTypeStorerTicket, To: key.Addr(), Value: "10", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return error='failed to get active delegate tickets: error'", func() {
					Expect(err.Error()).To(Equal("failed to get active delegate tickets: error"))
				})
			})

			When("target proposer/delegate has no active tickets", func() {
				BeforeEach(func() {
					tickets := []*types.Ticket{}
					mockLogic.TicketManager.EXPECT().GetActiveTicketsByProposer(gomock.Any(), gomock.Any(), gomock.Any()).Return(tickets, nil)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					tx := &types.Transaction{Type: types.TxTypeStorerTicket, To: key.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return error", func() {
					Expect(err.Error()).To(Equal("field:to, error:the delegate is not active"))
				})
			})

			When("target proposer/delegate has active tickets", func() {
				BeforeEach(func() {
					params.MinStorerStake = decimal.NewFromFloat(1)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					tickets := []*types.Ticket{&types.Ticket{Hash: "h1"}}
					mockLogic.TicketManager.EXPECT().GetActiveTicketsByProposer(gomock.Any(), gomock.Any(), gomock.Any()).Return(tickets, nil)
					mockLogic.Tx.EXPECT().CanExecCoinTransfer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
					tx := &types.Transaction{Type: types.TxTypeStorerTicket, To: key.Addr(), Value: "2", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(key.PubKey().Base58())}
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return nil", func() {
					Expect(err).To(BeNil())
				})
			})
		})

		When("tx type is TxTypeUnbondStorerTicket", func() {
			When("ticket is unknown or not found", func() {
				var err error
				BeforeEach(func() {
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
					tx := &types.Transaction{
						Type: types.TxTypeUnbondStorerTicket,
						Fee:  "1", Timestamp: time.Now().Unix(),
						UnbondTicket: &types.UnbondTicket{
							TicketID: []byte("ticket_id"),
						},
						SenderPubKey: util.String(key.PubKey().Base58()),
					}
					mockLogic.TicketManager.EXPECT().GetByHash(string(tx.UnbondTicket.TicketID)).Return(nil)
					err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
				})

				It("should return err='field:ticketID, error:ticket not found'", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:ticketID, error:ticket not found"))
				})
			})

			Context("for non-delegated ticket", func() {
				When("tx sender is not the proposer of the ticket", func() {
					var err error
					BeforeEach(func() {
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
						tx := &types.Transaction{
							Type: types.TxTypeUnbondStorerTicket,
							Fee:  "1", Timestamp: time.Now().Unix(),
							UnbondTicket: &types.UnbondTicket{
								TicketID: []byte("ticket_id"),
							},
							SenderPubKey: util.String(key.PubKey().Base58()),
						}
						returnTicket := &types.Ticket{Hash: string(tx.UnbondTicket.TicketID), ProposerPubKey: key2.PubKey().Base58()}
						mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)
						err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
					})

					It("should return err='field:ticketID, error:sender not authorized to unbond this ticket'", func() {
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(Equal("field:ticketID, error:sender not authorized to unbond this ticket"))
					})
				})
			})

			Context("for delegated ticket", func() {
				When("tx sender is not the delegator of the ticket", func() {
					var err error
					BeforeEach(func() {
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
						tx := &types.Transaction{
							Type:         types.TxTypeUnbondStorerTicket,
							Fee:          "1",
							Timestamp:    time.Now().Unix(),
							UnbondTicket: &types.UnbondTicket{TicketID: []byte("ticket_id")},
							SenderPubKey: util.String(key2.PubKey().Base58()),
						}
						returnTicket := &types.Ticket{Hash: string(tx.UnbondTicket.TicketID), ProposerPubKey: key2.PubKey().Base58(), Delegator: key.Addr().String()}
						mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)
						err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
					})

					It("should return err='field:ticketID, error:sender not authorized to unbond this ticket'", func() {
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(Equal("field:ticketID, error:sender not authorized to unbond this ticket"))
					})
				})

				When("ticket decay height is 0", func() {
					var err error
					BeforeEach(func() {
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 1}, nil)
						tx := &types.Transaction{
							Type:         types.TxTypeUnbondStorerTicket,
							Fee:          "1",
							Timestamp:    time.Now().Unix(),
							UnbondTicket: &types.UnbondTicket{TicketID: []byte("ticket_id")},
							SenderPubKey: util.String(key.PubKey().Base58()),
						}
						returnTicket := &types.Ticket{Hash: string(tx.UnbondTicket.TicketID), ProposerPubKey: key.PubKey().Base58(), DecayBy: 0}
						mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)
						mockLogic.Tx.EXPECT().CanExecCoinTransfer(gomock.Any(), gomock.Any(),
							gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
						err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
					})

					It("should return nil", func() {
						Expect(err).To(BeNil())
					})
				})

				When("ticket decay height is greater than 0 but less than current block height", func() {
					var err error
					BeforeEach(func() {
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						tx := &types.Transaction{
							Type:         types.TxTypeUnbondStorerTicket,
							Fee:          "1",
							Timestamp:    time.Now().Unix(),
							UnbondTicket: &types.UnbondTicket{TicketID: []byte("ticket_id")},
							SenderPubKey: util.String(key.PubKey().Base58()),
						}
						returnTicket := &types.Ticket{Hash: string(tx.UnbondTicket.TicketID), ProposerPubKey: key.PubKey().Base58(), DecayBy: 5}
						mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)
						err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
					})

					It("should return err='field:ticketID, error:ticket has already decayed'", func() {
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(Equal("field:ticketID, error:ticket has already decayed"))
					})
				})

				When("ticket decay height is greater than 0 but greater than current block height", func() {
					var err error
					BeforeEach(func() {
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 3}, nil)
						tx := &types.Transaction{
							Type:         types.TxTypeUnbondStorerTicket,
							Fee:          "1",
							Timestamp:    time.Now().Unix(),
							UnbondTicket: &types.UnbondTicket{TicketID: []byte("ticket_id")},
							SenderPubKey: util.String(key.PubKey().Base58()),
						}
						returnTicket := &types.Ticket{Hash: string(tx.UnbondTicket.TicketID), ProposerPubKey: key.PubKey().Base58(), DecayBy: 5}
						mockLogic.TicketManager.EXPECT().GetByHash(returnTicket.Hash).Return(returnTicket)
						err = validators.ValidateTxConsistency(tx, -1, mockLogic.Logic)
					})

					It("should return err='field:ticketID, error:ticket has already decayed'", func() {
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(Equal("field:ticketID, error:ticket is already decaying"))
					})
				})
			})
		})
	})

	Describe(".IsSet", func() {
		var cases = [][]interface{}{
			[]interface{}{map[string]interface{}{}, false},
			[]interface{}{map[string]interface{}{"a": 1}, true},
			[]interface{}{nil, false},
			[]interface{}{1, true},
			[]interface{}{-1, true},
			[]interface{}{0, false},
			[]interface{}{"", false},
			[]interface{}{"a", true},
			[]interface{}{[]byte{1}, true},
			[]interface{}{[]byte{}, false},
		}

		for _, c := range cases {
			It(fmt.Sprintf("should return %v for %v", c[0], c[1]), func() {
				Expect(validators.IsSet(c[0])).To(Equal(c[1]))
			})
		}
	})

	Describe(".CheckUnexpectedFields", func() {
		When("check TxTypeValidatorTicket", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeValidatorTicket}
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `epochSecret` field", func() {
				tx.EpochSecret = &types.EpochSecret{}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:epochSecret, error:unexpected field"))
			})

			It("should not accept a set `unbondTicket` field", func() {
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte{1, 2}}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:unbondTicket, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.RepoCreate = &types.RepoCreate{Name: "repo"}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoCreate, error:unexpected field"))
			})
		})

		When("check TxTypeEpochSecret", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeEpochSecret}
				tx.Timestamp = 0
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `nonce` field", func() {
				tx.Nonce = 1
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:unexpected field"))
			})

			It("should not accept a set `to` field", func() {
				tx.To = "address"
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:unexpected field"))
			})

			It("should not accept a set `senderPubKey` field", func() {
				tx.SenderPubKey = "pub_key"
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:unexpected field"))
			})

			It("should not accept a set `value` field", func() {
				tx.Value = "10"
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:unexpected field"))
			})

			It("should not accept a set `timestamp` field", func() {
				tx.Timestamp = 100000
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:unexpected field"))
			})

			It("should not accept a set `fee` field", func() {
				tx.Fee = "10"
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:unexpected field"))
			})

			It("should not accept a set `sig` field", func() {
				tx.Sig = []byte{1, 2}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:unexpected field"))
			})

			It("should not accept a set `unbondTicket` field", func() {
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte{1, 2}}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:unbondTicket, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.RepoCreate = &types.RepoCreate{Name: "repo"}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoCreate, error:unexpected field"))
			})
		})

		When("check TxTypeSetDelegatorCommission", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeSetDelegatorCommission}
				tx.Timestamp = 0
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `epochSecret` field", func() {
				tx.EpochSecret = &types.EpochSecret{}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:epochSecret, error:unexpected field"))
			})

			It("should not accept a set `to` field", func() {
				tx.To = util.String("address")
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:unexpected field"))
			})

			It("should not accept a set `unbondTicket` field", func() {
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte{1, 2}}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:unbondTicket, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.RepoCreate = &types.RepoCreate{Name: "repo"}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoCreate, error:unexpected field"))
			})
		})

		When("check TxTypeStorerTicket", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeStorerTicket}
				tx.Timestamp = 0
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `epochSecret` field", func() {
				tx.EpochSecret = &types.EpochSecret{}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:epochSecret, error:unexpected field"))
			})

			It("should not accept a set `unbondTicket` field", func() {
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte{1, 2}}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:unbondTicket, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.RepoCreate = &types.RepoCreate{Name: "repo"}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoCreate, error:unexpected field"))
			})
		})

		When("check TxTypeUnbondStorerTicket", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeUnbondStorerTicket}
				tx.Timestamp = 0
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `to` field", func() {
				tx.To = util.String("address")
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:unexpected field"))
			})

			It("should not accept a set `value` field", func() {
				tx.Value = util.String("10")
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:unexpected field"))
			})

			It("should not accept a set `epochSecret` field", func() {
				tx.EpochSecret = &types.EpochSecret{}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:epochSecret, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.RepoCreate = &types.RepoCreate{Name: "repo"}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoCreate, error:unexpected field"))
			})
		})

		When("check TxTypeRepoCreate", func() {
			var tx *types.Transaction

			BeforeEach(func() {
				tx = &types.Transaction{Type: types.TxTypeRepoCreate}
				tx.Timestamp = 0
			})

			It("should not accept a set `meta` field", func() {
				tx.SetMeta(map[string]interface{}{"a": 2})
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:meta, error:unexpected field"))
			})

			It("should not accept a set `to` field", func() {
				tx.To = util.String("address")
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:unexpected field"))
			})

			It("should not accept a set `value` field", func() {
				tx.Value = util.String("10")
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:unexpected field"))
			})

			It("should not accept a set `epochSecret` field", func() {
				tx.EpochSecret = &types.EpochSecret{}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:epochSecret, error:unexpected field"))
			})

			It("should not accept a set `repoCreate` field", func() {
				tx.UnbondTicket = &types.UnbondTicket{TicketID: []byte("repo")}
				err := validators.CheckUnexpectedFields(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:unbondTicket, error:unexpected field"))
			})
		})
	})

	Describe(".ValidateEpochSecretTx", func() {

		When("unexpected field is set", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret}
				tx.Value = "1"
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:value, error:unexpected field'", func() {
				Expect(err.Error()).To(Equal("field:value, error:unexpected field"))
			})
		})

		When("secret is not set", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret, EpochSecret: &types.EpochSecret{}}
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:secret, error:secret is required'", func() {
				Expect(err.Error()).To(Equal("field:secret, error:secret is required"))
			})
		})

		When("secret length is not 64", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret, EpochSecret: &types.EpochSecret{}}
				tx.EpochSecret.Secret = util.RandBytes(2)
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:secret, error:invalid length; expected 64 bytes'", func() {
				Expect(err.Error()).To(Equal("field:secret, error:invalid length; expected 64 bytes"))
			})
		})

		When("previous secret is not set", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret, EpochSecret: &types.EpochSecret{}}
				tx.EpochSecret.Secret = util.RandBytes(64)
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:previousSecret, error:previous secret is required'", func() {
				Expect(err.Error()).To(Equal("field:previousSecret, error:previous secret is required"))
			})
		})

		When("previous secret length is not 64", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret, EpochSecret: &types.EpochSecret{}}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(2)
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:previousSecret, error:invalid length; expected 64 bytes'", func() {
				Expect(err.Error()).To(Equal("field:previousSecret, error:invalid length; expected 64 bytes"))
			})
		})

		When("secret round is 0", func() {
			var err error
			BeforeEach(func() {
				tx := &types.Transaction{Type: types.TxTypeEpochSecret, EpochSecret: &types.EpochSecret{}}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				err = validators.ValidateEpochSecretTx(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:secretRound, error:secret round is required'", func() {
				Expect(err.Error()).To(Equal("field:secretRound, error:secret round is required"))
			})
		})
	})

	Describe(".ValidateEpochSecretTxConsistency", func() {
		When("secret is not valid", func() {
			var err error
			BeforeEach(func() {
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.EpochSecret = &types.EpochSecret{}
				tx.EpochSecret.Secret = util.RandBytes(64)
				tx.EpochSecret.PreviousSecret = util.RandBytes(64)
				err = validators.ValidateEpochSecretTxConsistency(tx, -1, logic)
				Expect(err).ToNot(BeNil())
			})

			It("should err='field:secret, error:epoch secret is invalid'", func() {
				Expect(err.Error()).To(Equal("field:secret, error:epoch secret is invalid"))
			})
		})

		When("secret is valid but failed to get highest drand round", func() {
			var err error
			BeforeEach(func() {
				mockLogic.Drand.EXPECT().Verify(validEpochSecretTx.EpochSecret.Secret, validEpochSecretTx.EpochSecret.PreviousSecret,
					validEpochSecretTx.EpochSecret.SecretRound).Return(nil)
				mockLogic.SysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(0), fmt.Errorf("error"))
				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic.Logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='failed to get highest drand round: error'", func() {
				Expect(err.Error()).To(Equal("failed to get highest drand round: error"))
			})
		})

		When("secret is valid but its round is not greater than the current highest round", func() {
			var err error
			BeforeEach(func() {
				mockLogic.Drand.EXPECT().Verify(validEpochSecretTx.EpochSecret.Secret, validEpochSecretTx.EpochSecret.PreviousSecret,
					validEpochSecretTx.EpochSecret.SecretRound).Return(nil)
				mockLogic.SysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(1001), nil)
				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic.Logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:secretRound, error:must be greater than the previous round'", func() {
				Expect(err.Error()).To(Equal("field:secretRound, error:must be greater than the previous round"))
			})
		})

		When("secret is valid but its round was created too early (less than the expected round)", func() {
			var err error

			// Highest Round = 999
			// Expected Round = 1001
			// Tx Round = 1000
			BeforeEach(func() {
				mockLogic.Drand.EXPECT().Verify(validEpochSecretTx.EpochSecret.Secret, validEpochSecretTx.EpochSecret.PreviousSecret,
					validEpochSecretTx.EpochSecret.SecretRound).Return(nil)
				mockLogic.SysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(999), nil)
				params.NumBlocksPerEpoch = 120
				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic.Logic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='field:secretRound, error:round was generated too early'", func() {
				Expect(err.Error()).To(Equal("field:secretRound, error:round was generated too early"))
			})
		})

		When("secret is valid and its round is the expected round", func() {
			var err error

			// Highest Round = 998
			// Expected Round = 1000
			// Tx Round = 1000
			BeforeEach(func() {
				mockLogic.Drand.EXPECT().Verify(validEpochSecretTx.EpochSecret.Secret, validEpochSecretTx.EpochSecret.PreviousSecret,
					validEpochSecretTx.EpochSecret.SecretRound).Return(nil)
				mockLogic.SysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(998), nil)
				params.BlockTime = 1
				params.NumBlocksPerEpoch = 120
				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic.Logic)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
