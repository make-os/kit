package pushtoken

import (
	"testing"

	"github.com/golang/mock/gomock"
	crypto2 "github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/types"
	"github.com/mr-tron/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPushToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation Suite")
}

var _ = Describe("TxDetail", func() {
	var ctrl *gomock.Controller
	var key = crypto2.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".DecodePushToken", func() {
		When("token is malformed (not base58 encoded)", func() {
			It("should return err", func() {
				_, err := DecodePushToken("invalid_token")
				Expect(err).To(MatchError("malformed token"))
			})
		})

		When("token is malformed (can't be deserialized to TxDetail)", func() {
			It("should return err", func() {
				_, err := DecodePushToken(base58.Encode([]byte("invalid data")))
				Expect(err).To(MatchError("malformed token"))
			})
		})

		When("token is valid", func() {
			It("should return no error and transaction detail object", func() {
				txDetail := &types.TxDetail{RepoName: "repo1"}
				token := base58.Encode(txDetail.Bytes())
				res, err := DecodePushToken(token)
				Expect(err).To(BeNil())
				Expect(res.Equal(txDetail)).To(BeTrue())
			})
		})
	})

	Describe(".MakePushToken", func() {
		var token string
		var txDetail *types.TxDetail

		BeforeEach(func() {
			txDetail = &types.TxDetail{RepoName: "repo1"}
			mockStoreKey := mocks.NewMockStoredKey(ctrl)
			mockStoreKey.EXPECT().GetKey().Return(key)
			token = MakePushToken(mockStoreKey, txDetail)
		})

		It("should return token", func() {
			Expect(token).ToNot(BeEmpty())
		})

		It("should decode token successfully", func() {
			txD, err := DecodePushToken(token)
			Expect(err).To(BeNil())
			Expect(txD.Equal(txDetail)).To(BeTrue())
		})
	})

	Describe(".IsValidPushToken", func() {
		It("should return false if token is invalid", func() {
			Expect(IsValidPushToken("invalid")).To(BeFalse())
		})

		It("should return true if token is invalid", func() {
			txDetail := &types.TxDetail{RepoName: "repo1"}
			token := base58.Encode(txDetail.Bytes())
			Expect(IsValidPushToken(token)).To(BeTrue())
		})
	})
})
