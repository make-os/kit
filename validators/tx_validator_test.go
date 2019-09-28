package validators_test

import (
	"fmt"
	"os"
	"time"

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

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = l.New(c, state, cfg)
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ValidateTxSyntax", func() {

		var to = crypto.NewKeyFromIntSeed(1)
		var txMissingSignature = &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		txMissingSignature.Hash = txMissingSignature.ComputeHash()
		var txInvalidSig = &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}
		txInvalidSig.Hash = txInvalidSig.ComputeHash()
		txInvalidSig.Sig = []byte("invalid")

		var cases = []txCase{
			{tx: nil, desc: "nil is provided", err: fmt.Errorf("nil tx")},
			{tx: &types.Transaction{Type: 1000}, desc: "tx type is invalid", err: fmt.Errorf("field:type, error:unsupported transaction type")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: ""}, desc: "recipient not set", err: fmt.Errorf("field:to, error:recipient address is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: "abc"}, desc: "recipient not valid", err: fmt.Errorf("field:to, error:recipient address is not valid")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr()}, desc: "value not provided", err: fmt.Errorf("field:value, error:value is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "-1"}, desc: "value is negative", err: fmt.Errorf("field:value, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1"}, desc: "fee not provided", err: fmt.Errorf("field:fee, error:fee is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "-1"}, desc: "fee is negative", err: fmt.Errorf("field:fee, error:negative figure not allowed")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "0.0000000001"}, desc: "fee lower than base price", err: fmt.Errorf("field:fee, error:fee cannot be lower than the base price of 0.0010")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1"}, desc: "timestamp not provided", err: fmt.Errorf("field:timestamp, error:timestamp is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix() + 10}, desc: "timestamp is a future time", err: fmt.Errorf("field:timestamp, error:timestamp cannot be a future time")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix()}, desc: "sender pub key not provided", err: fmt.Errorf("field:senderPubKey, error:sender public key is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: "abc"}, desc: "sender pub key is not valid", err: fmt.Errorf("field:senderPubKey, error:sender public key is not valid")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58())}, desc: "hash is not provided", err: fmt.Errorf("field:hash, error:hash is required")},
			{tx: &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: util.String(to.PubKey().Base58()), Hash: util.StrToHash("invalid")}, desc: "hash is not correct", err: fmt.Errorf("field:hash, error:hash is not correct")},
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
			tx := &types.Transaction{Type: types.TxTypeCoinTransfer, To: to.Addr(), Value: "1", Fee: "1", Timestamp: time.Now().Unix(), SenderPubKey: "abc"}
			err := validators.ValidateTxConsistency(tx, -1, nil)
			Expect(err.Error()).To(Equal("field:senderPubKey, error:invalid format: version and/or checksum bytes missing"))
		})
	})
})
