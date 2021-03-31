package modules

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	mockrpc "github.com/make-os/kit/mocks/rpc"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

func TestModules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Modules Suite")
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
			Expect(key).To(BeNil())
			Expect(payloadOnly).To(BeFalse())
		})

		It("should panic when options list contain 1 argument that is a string and failed key validation", func() {
			err := "private key is invalid: invalid format: version and/or checksum bytes missing"
			assert.PanicsWithError(GinkgoT(), err, func() { parseOptions("invalid_key") })
		})

		It("should return key when options list contain 1 argument that is a string and passed key validation", func() {
			pk := ed25519.NewKeyFromIntSeed(1)
			key, payloadOnly := parseOptions(pk.PrivKey().Base58())
			Expect(payloadOnly).To(BeFalse())
			Expect(key.Base58()).To(Equal(pk.PrivKey().Base58()))
		})

		It("should return payloadOnly=true when options list contain 1 argument that is a boolean (true)", func() {
			key, payloadOnly := parseOptions(true)
			Expect(key).To(BeNil())
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
			payloadOnly, _ := finalizeTx(tx, mockKeepers, nil)
			Expect(payloadOnly).To(BeFalse())
			Expect(tx.SenderPubKey.IsEmpty()).To(BeTrue())
			Expect(tx.Sig).To(BeEmpty())
		})

		It("should not set nonce when key is not provided", func() {
			tx := txns.NewBareTxCoinTransfer()
			finalizeTx(tx, mockKeepers, nil)
			Expect(tx.Nonce).To(BeZero())
		})

		It("should set timestamp if not set", func() {
			tx := txns.NewBareTxCoinTransfer()
			Expect(tx.Timestamp).To(BeZero())
			finalizeTx(tx, mockKeepers, nil)
			Expect(tx.Timestamp).ToNot(BeZero())
		})

		It("should sign the tx, set sender public key, sent nonce when key is provided", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			mockAcctKeeper.EXPECT().Get(key.Addr()).Return(&state.Account{Nonce: 1})
			tx := txns.NewBareTxCoinTransfer()
			payloadOnly, pk := finalizeTx(tx, mockKeepers, nil, key.PrivKey().Base58())
			Expect(pk).ToNot(BeNil())
			Expect(pk.Base58()).To(Equal(key.PrivKey().Base58()))
			Expect(payloadOnly).To(BeFalse())
			Expect(tx.SenderPubKey.IsEmpty()).To(BeFalse())
			Expect(tx.Sig).ToNot(BeEmpty())
			Expect(tx.Nonce).To(Equal(uint64(2)))
		})

		It("should panic if account keeper returns empty account", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			mockAcctKeeper.EXPECT().Get(key.Addr()).Return(state.BareAccount())
			tx := txns.NewBareTxCoinTransfer()
			Expect(func() {
				finalizeTx(tx, mockKeepers, nil, key.PrivKey().Base58())
			}).To(Panic())
		})

		When("rpc client is set and keeper is not set", func() {
			It("should use rpc client to get nonce", func() {
				mockRPCClient := mockrpc.NewMockClient(ctrl)
				mockUserClient := mockrpc.NewMockUser(ctrl)
				mockRPCClient.EXPECT().User().Return(mockUserClient)

				key := ed25519.NewKeyFromIntSeed(1)
				tx := txns.NewBareTxCoinTransfer()
				mockUserClient.EXPECT().Get(key.Addr().String()).Return(&api.ResultAccount{Account: &state.Account{Nonce: 1}}, nil)

				payloadOnly, pk := finalizeTx(tx, nil, mockRPCClient, key.PrivKey().Base58())
				Expect(pk).ToNot(BeNil())
				Expect(pk.Base58()).To(Equal(key.PrivKey().Base58()))
				Expect(payloadOnly).To(BeFalse())
				Expect(tx.SenderPubKey.IsEmpty()).To(BeFalse())
				Expect(tx.Nonce).To(Equal(uint64(2)))
			})

			It("should not sign the tx", func() {
				mockRPCClient := mockrpc.NewMockClient(ctrl)
				mockUserClient := mockrpc.NewMockUser(ctrl)
				mockRPCClient.EXPECT().User().Return(mockUserClient)

				key := ed25519.NewKeyFromIntSeed(1)
				tx := txns.NewBareTxCoinTransfer()
				mockUserClient.EXPECT().Get(key.Addr().String()).Return(&api.ResultAccount{Account: &state.Account{Nonce: 1}}, nil)

				finalizeTx(tx, nil, mockRPCClient, key.PrivKey().Base58())
				Expect(tx.Sig).To(BeEmpty())
			})
		})

		It("should panic if rpc client returns error", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			tx := txns.NewBareTxCoinTransfer()
			mockRPCClient := mockrpc.NewMockClient(ctrl)
			mockUserClient := mockrpc.NewMockUser(ctrl)
			mockRPCClient.EXPECT().User().Return(mockUserClient)
			mockUserClient.EXPECT().Get(key.Addr().String()).Return(nil, fmt.Errorf("error"))

			Expect(func() {
				finalizeTx(tx, nil, mockRPCClient, key.PrivKey().Base58())
			}).To(Panic())
		})
	})

	Describe(".Select", func() {
		It("should select correctly from a 1-level JSON string", func() {
			m := map[string]interface{}{"age": 100}
			out, err := Select(util.MustToJSON(m), "age")
			Expect(err).To(BeNil())
			Expect(out).To(Equal(map[string]interface{}{
				"age": float64(100),
			}))
		})

		It("should select correctly from a 2-level JSON string", func() {
			m := map[string]interface{}{"person": map[string]interface{}{"age": 100, "gender": "f"}}
			out, err := Select(util.MustToJSON(m), "person.age")
			Expect(err).To(BeNil())
			Expect(out).To(Equal(map[string]interface{}{
				"person": map[string]interface{}{
					"age": float64(100),
				},
			}))
		})

		It("should select correctly from a 2-level JSON string with 2 selectors", func() {
			m := map[string]interface{}{"person": map[string]interface{}{"age": 100, "gender": "f"}}
			out, err := Select(util.MustToJSON(m), "person.age", "person.gender")
			Expect(err).To(BeNil())
			Expect(out).To(Equal(map[string]interface{}{
				"person": map[string]interface{}{
					"age":    float64(100),
					"gender": "f",
				},
			}))
		})

		It("should return error if selector format is malformed", func() {
			m := map[string]interface{}{"person": map[string]interface{}{"age": 100, "gender": "f"}}
			_, err := Select(util.MustToJSON(m), "person.age", "person*gender")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("selector at index=1 is malformed"))
		})
	})
})
