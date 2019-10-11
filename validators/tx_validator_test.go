package validators_test

import (
	"fmt"
	"os"
	"time"

	"github.com/makeos/mosdef/params"

	drandmocks "github.com/makeos/mosdef/crypto/rand/mocks"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/validators"

	"github.com/makeos/mosdef/config"
	l "github.com/makeos/mosdef/logic"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
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
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var state *tree.SafeTree
	var logic *l.Logic
	var ctrl *gomock.Controller

	validEpochSecretTx := types.NewBareTx(types.TxTypeEpochSecret)
	validEpochSecretTx.SecretRound = 1000
	validEpochSecretTx.Secret = []uint8{
		0x3a, 0x06, 0x2b, 0xf4, 0xac, 0x34, 0x57, 0x06, 0xcd, 0x41, 0x62, 0xa7, 0x25, 0x39, 0xb8, 0x4a,
		0x73, 0xf7, 0xf4, 0x1e, 0x57, 0x89, 0x88, 0xdc, 0x9f, 0xef, 0xc2, 0xd4, 0x5f, 0x80, 0xe2, 0xec,
		0x64, 0x9e, 0xdc, 0x53, 0xb7, 0x26, 0x4b, 0x0c, 0xdf, 0x41, 0xe3, 0x63, 0xb1, 0xb9, 0xf4, 0xcd,
		0x73, 0x0c, 0x35, 0xd3, 0xf6, 0x31, 0x78, 0x14, 0x24, 0xef, 0xa4, 0x3a, 0x79, 0x63, 0xf1, 0x01,
	}
	validEpochSecretTx.PreviousSecret = []uint8{
		0x28, 0x18, 0x21, 0x0a, 0x81, 0xb6, 0x28, 0x88, 0xa9, 0x24, 0x29, 0x55, 0xf2, 0x01, 0x30, 0x80,
		0xa9, 0x7e, 0xa3, 0x55, 0x7c, 0x6d, 0xfe, 0x8a, 0x5d, 0x94, 0x0d, 0x8f, 0x65, 0x46, 0xdd, 0x99,
		0x69, 0xf2, 0xf9, 0x10, 0xd5, 0xcf, 0x15, 0xcc, 0x0e, 0x39, 0x17, 0xa8, 0xd9, 0x90, 0x21, 0x57,
		0x5e, 0x27, 0xdb, 0xfd, 0x25, 0x61, 0x54, 0xb1, 0x4d, 0xdc, 0xbf, 0xb1, 0xbf, 0xb4, 0x5e, 0x44,
	}

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = l.New(c, state, cfg)
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

	Describe(".ValidateTxSyntax", func() {

		var to = crypto.NewKeyFromIntSeed(1)
		var txMissingSignature = &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		var txInvalidSig = &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		txInvalidSig.Sig = []byte("invalid")

		var cases = []txCase{
			{tx: nil, desc: "nil is provided", err: fmt.Errorf("nil tx")},
			{tx: &types.Transaction{Type: 1000}, desc: "tx type is invalid", err: fmt.Errorf("field:type, error:unsupported transaction type")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: ""}, desc: "recipient not set", err: fmt.Errorf("field:to, error:recipient address is required")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: "abc"}, desc: "recipient not valid", err: fmt.Errorf("field:to, error:recipient address is not valid")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr()}, desc: "value not provided", err: fmt.Errorf("field:value, error:value is required")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "-1"}, desc: "value is negative", err: fmt.Errorf("field:value, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1"}, desc: "fee not provided", err: fmt.Errorf("field:fee, error:fee is required")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "-1"}, desc: "fee is negative", err: fmt.Errorf("field:fee, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "0.0000000001"}, desc: "fee lower than base price", err: fmt.Errorf("field:fee, error:fee cannot be lower than the base price of 0.0008")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1"}, desc: "timestamp not provided", err: fmt.Errorf("field:timestamp, error:timestamp is required")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix()}, desc: "sender pub key not provided", err: fmt.Errorf("field:senderPubKey, error:sender public key is required")},
			{tx: &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: "abc"}, desc: "sender pub key is not valid", err: fmt.Errorf("field:senderPubKey, error:sender public key is not valid")},
			{tx: txMissingSignature, desc: "signature not provided", err: fmt.Errorf("field:sig, error:signature is required")},
			{tx: txInvalidSig, desc: "signature not valid", err: fmt.Errorf("field:sig, error:signature is not valid")},
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
		to := crypto.NewKeyFromIntSeed(1)
		It("should return err='field:senderPubKey, error:invalid format: version and/or checksum bytes missing' when tx sender public key is not valid", func() {
			tx := &types.Transaction{Type: types.TxTypeTransferCoin, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: "abc"}
			err := validators.ValidateTxConsistency(tx, -1, nil)
			Expect(err.Error()).To(Equal("field:senderPubKey, error:invalid format: version and/or checksum bytes missing"))
		})
	})

	Describe(".ValidateEpochSecretTx", func() {
		When("secret is not set", func() {
			var err error
			BeforeEach(func() {
				tx := types.NewBareTx(types.TxTypeEpochSecret)
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
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(2)
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
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
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
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(2)
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
				tx := types.NewBareTx(types.TxTypeEpochSecret)
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
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
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
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
				mockLogic := mocks.NewMockLogic(ctrl)
				mockDrand := drandmocks.NewMockDRander(ctrl)
				mockDrand.EXPECT().Verify(validEpochSecretTx.Secret, validEpochSecretTx.PreviousSecret,
					validEpochSecretTx.SecretRound).Return(nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(0), fmt.Errorf("error"))
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().GetDRand().Return(mockDrand)

				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic)
				Expect(err).ToNot(BeNil())
			})

			It("should return err='failed to get highest drand round: error'", func() {
				Expect(err.Error()).To(Equal("failed to get highest drand round: error"))
			})
		})

		When("secret is valid but its round is not greater than the current highest round", func() {
			var err error
			BeforeEach(func() {
				mockLogic := mocks.NewMockLogic(ctrl)
				mockDrand := drandmocks.NewMockDRander(ctrl)
				mockDrand.EXPECT().Verify(validEpochSecretTx.Secret, validEpochSecretTx.PreviousSecret,
					validEpochSecretTx.SecretRound).Return(nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(1001), nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().GetDRand().Return(mockDrand)

				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic)
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
				mockLogic := mocks.NewMockLogic(ctrl)
				mockDrand := drandmocks.NewMockDRander(ctrl)
				mockDrand.EXPECT().Verify(validEpochSecretTx.Secret, validEpochSecretTx.PreviousSecret,
					validEpochSecretTx.SecretRound).Return(nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(999), nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().GetDRand().Return(mockDrand)

				params.NumBlocksPerEpoch = 120

				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic)
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
				mockLogic := mocks.NewMockLogic(ctrl)
				mockDrand := drandmocks.NewMockDRander(ctrl)
				mockDrand.EXPECT().Verify(validEpochSecretTx.Secret, validEpochSecretTx.PreviousSecret,
					validEpochSecretTx.SecretRound).Return(nil)
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetHighestDrandRound().Return(uint64(998), nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mockLogic.EXPECT().GetDRand().Return(mockDrand)

				params.NumBlocksPerEpoch = 120

				err = validators.ValidateEpochSecretTxConsistency(validEpochSecretTx, -1, mockLogic)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
