package crypto

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
)

var _ = Describe("WrappedPV", func() {
	Describe(".GetKey", func() {
		It("should return expected key and no error", func() {
			privKey := ed25519.GenPrivKeyFromSecret([]byte("abc"))
			wpv := WrappedPV{FilePV: &privval.FilePV{
				Key: privval.FilePVKey{
					PubKey:  privKey.PubKey(),
					PrivKey: privKey,
					Address: privKey.PubKey().Address(),
				},
			}}
			key, err := wpv.GetKey()
			Expect(err).To(BeNil())
			Expect(key.Addr().String()).To(Equal("eCDUbWW9prPkFL1aMTvJSAmYicpxHUkB21"))
		})
	})
})
