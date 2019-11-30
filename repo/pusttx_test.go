package repo

import (
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushTx", func() {
	var pushTx *PushTx

	BeforeEach(func() {
		pushTx = &PushTx{
			RepoName:    "repo",
			NodeSig:     []byte("node_signer_sig"),
			PusherKeyID: "pk_id",
			References: []*PushedReference{
				{
					Nonce:        1,
					NewObjectID:  "new_object_hash",
					Name:         "refs/heads/master",
					OldObjectID:  "old_object_hash",
					Sig:          "abc_xyz",
					Fee:          "0.2",
					AccountNonce: 2,
				},
			},
		}
	})

	Describe(".Bytes", func() {
		It("should return expected bytes", func() {
			Expect(pushTx.Bytes()).ToNot(HaveLen(0))
		})
	})

	Describe(".ID", func() {
		It("should return expected bytes", func() {
			Expect(len(pushTx.ID())).To(Equal(32))
		})
	})

	Describe(".TotalFee", func() {
		It("should return expected total fee", func() {
			Expect(pushTx.TotalFee()).To(Equal(util.String("0.2")))
		})

		It("should return expected total fee", func() {
			pushTx.References = append(pushTx.References, &PushedReference{
				Nonce:        1,
				NewObjectID:  "new_object_hash",
				Name:         "refs/heads/master",
				OldObjectID:  "old_object_hash",
				Sig:          "abc_xyz",
				Fee:          "0.2",
				AccountNonce: 2,
			})
			Expect(pushTx.TotalFee()).To(Equal(util.String("0.4")))
		})
	})

	Describe(".LenMinusFee", func() {
		It("should return expected length without the fee fields", func() {
			lenFee := len(util.ObjectToBytes(pushTx.References[0].Fee))
			lenWithFee := pushTx.Len()
			expected := lenWithFee - uint64(lenFee)
			Expect(pushTx.LenMinusFee()).To(Equal(expected))
		})
	})

	Describe(".Len", func() {
		It("should return expected length", func() {
			Expect(pushTx.Len()).To(Equal(uint64(128)))
		})
	})

	Describe(".TxSize", func() {
		It("should be non-zero", func() {
			Expect(pushTx.TxSize()).ToNot(Equal(0))
		})
	})

	Describe(".OverallSize", func() {
		It("should be non-zero", func() {
			Expect(pushTx.BillableSize()).ToNot(Equal(0))
		})
	})
})
