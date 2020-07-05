package modules

import (
	"math/big"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

type TestCase struct {
	Desc           string
	Obj            interface{}
	Expected       interface{}
	FieldsToIgnore []string
	ShouldPanic    bool
}

var _ = Describe("Common", func() {
	var ctrl *gomock.Controller
	var mockKeepers *mocks.MockKeepers
	var mockAcctKeeper *mocks.MockAccountKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".parseOptions", func() {
		It("should return no key and payloadOnly=false when options list contain 1 argument that is not a string or boolean", func() {
			key, payloadOnly := parseOptions(1)
			Expect(key).To(BeEmpty())
			Expect(payloadOnly).To(BeFalse())
		})

		It("should panic when options list contain 1 argument that is a string and failed key validation", func() {
			err := "private key is invalid: invalid format: version and/or checksum bytes missing"
			assert.PanicsWithError(GinkgoT(), err, func() { parseOptions("invalid_key") })
		})

		It("should return key when options list contain 1 argument that is a string and passed key validation", func() {
			pk := crypto.NewKeyFromIntSeed(1)
			key, payloadOnly := parseOptions(pk.PrivKey().Base58())
			Expect(payloadOnly).To(BeFalse())
			Expect(key).To(Equal(pk.PrivKey().Base58()))
		})

		It("should return payloadOnly=true when options list contain 1 argument that is a boolean (true)", func() {
			key, payloadOnly := parseOptions(true)
			Expect(key).To(BeEmpty())
			Expect(payloadOnly).To(BeTrue())
		})

		It("should panic when options list contain more than 1 arguments but arg=0 is not string", func() {
			err := "failed to decode argument.0 to string"
			assert.PanicsWithError(GinkgoT(), err, func() { parseOptions(1, "data") })
		})

		It("should panic when options list contain more than 1 arguments but arg=1 is not boolean", func() {
			err := "failed to decode argument.1 to bool"
			assert.PanicsWithError(GinkgoT(), err, func() { parseOptions("key", 123) })
		})
	})

	Describe(".finalizeTx", func() {
		It("should not sign the tx or set sender public key when key is not provided", func() {
			tx := txns.NewBareTxCoinTransfer()
			payloadOnly := finalizeTx(tx, mockKeepers)
			Expect(payloadOnly).To(BeFalse())
			Expect(tx.SenderPubKey.IsEmpty()).To(BeTrue())
			Expect(tx.Sig).To(BeEmpty())
		})

		It("should not set nonce when key is not provided", func() {
			tx := txns.NewBareTxCoinTransfer()
			finalizeTx(tx, mockKeepers)
			Expect(tx.Nonce).To(BeZero())
		})

		It("should set timestamp if not set", func() {
			tx := txns.NewBareTxCoinTransfer()
			Expect(tx.Timestamp).To(BeZero())
			finalizeTx(tx, mockKeepers)
			Expect(tx.Timestamp).ToNot(BeZero())
		})

		It("should sign the tx, set sender public key, sent nonce when key is provided", func() {
			key := crypto.NewKeyFromIntSeed(1)
			mockAcctKeeper.EXPECT().Get(key.Addr()).Return(&state.Account{Nonce: 1})

			tx := txns.NewBareTxCoinTransfer()
			payloadOnly := finalizeTx(tx, mockKeepers, key.PrivKey().Base58())
			Expect(payloadOnly).To(BeFalse())
			Expect(tx.SenderPubKey.IsEmpty()).To(BeFalse())
			Expect(tx.Sig).ToNot(BeEmpty())
			Expect(tx.Nonce).To(Equal(uint64(2)))
		})
	})

	Describe(".Normalize", func() {

		type test1 struct {
			Name string
			Desc []byte
		}

		type test2 struct {
			Age    int64
			Others test1
			More   []interface{} `json:",omitempty"`
		}

		type test3 struct {
			Sig util.Bytes32
		}

		type test4 struct {
			Num *big.Int
		}

		type test5 struct {
			Num util.BlockNonce
		}

		var t1 = test1{Name: "fred", Desc: []byte("i love games")}
		var cases = []TestCase{
			{
				Desc:        "should panic with non-map, non-slice map or struct",
				Obj:         []interface{}{1, 2},
				ShouldPanic: true,
			},
			{
				Desc:        "should panic with nil result",
				Obj:         nil,
				Expected:    []util.Map{},
				ShouldPanic: true,
			},
			{
				Desc:     "with a string and []byte field",
				Obj:      t1,
				Expected: util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
			},
			{
				Desc:     "with an integer field and a struct field (with string and []byte fields)",
				Obj:      test2{Age: 20, Others: t1},
				Expected: util.Map{"Age": "20", "Others": util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"}},
			},
			{
				Desc: "with an integer field, a slice of struct field and a struct field (with string and []byte fields)",
				Obj:  test2{Age: 20, Others: t1, More: []interface{}{t1}},
				Expected: util.Map{"Age": "20",
					"Others": util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
					"More":   []interface{}{util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"}}},
			},
			{
				Desc:     "with a byte slice field",
				Obj:      test3{Sig: util.StrToBytes32("fred")},
				Expected: util.Map{"Sig": "0x6672656400000000000000000000000000000000000000000000000000000000"},
			},
			{
				Desc:     "with a big.Int field",
				Obj:      test4{Num: new(big.Int).SetInt64(10)},
				Expected: util.Map{"Num": "10"},
			},
			{
				Desc:     "with a BlockNonce field",
				Obj:      test5{Num: util.EncodeNonce(10)},
				Expected: util.Map{"Num": "0x000000000000000a"},
			},
			{
				Desc:           "with fields to be ignored",
				Obj:            test2{Age: 30, Others: test1{Desc: []byte("i love games")}},
				FieldsToIgnore: []string{"Age"},
				Expected:       util.Map{"Age": int64(30), "Others": util.Map{"Name": "", "Desc": "0x69206c6f76652067616d6573"}},
			},
			{
				Desc: "with a slice of structs",
				Obj:  []interface{}{t1, t1},
				Expected: []util.Map{
					{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
					{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
				},
			},
			{
				Desc: "with a slice of map[string]int",
				Obj:  []interface{}{map[string]int{"age": 10}, map[string]int{"age": 1000}},
				Expected: []util.Map{
					{"age": 10},
					{"age": 1000},
				},
			},
			{
				Desc: "with a slice of map[string]float64",
				Obj:  []interface{}{map[string]float64{"age": 10.2}, map[string]float64{"age": 1000.3}},
				Expected: []util.Map{
					{"age": "10.2"},
					{"age": "1000.3"},
				},
			},
			{
				Desc:        "should panic with non-map, non-slice map or struct",
				Obj:         "string",
				Expected:    []util.Map{},
				ShouldPanic: true,
			},
		}

		for _, c := range cases {
			_c := c
			It(_c.Desc, func() {
				if !_c.ShouldPanic {
					result := Normalize(_c.Obj, _c.FieldsToIgnore...)
					Expect(result).To(Equal(_c.Expected))
				} else {
					Expect(func() {
						Normalize(_c.Obj, _c.FieldsToIgnore...)
					}).To(Panic())
				}
			})
		}
	})
})
