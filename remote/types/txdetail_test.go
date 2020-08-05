package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("TxDetail", func() {

	Describe(".TxDetailFromPEMHeader", func() {
		It("should return err when nonce is not a number", func() {
			hdr := map[string]string{"nonce": "0x"}
			_, err := TxDetailFromPEMHeader(hdr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("nonce must be numeric"))
		})

		It("should successfully create TxDetail from PEM header", func() {
			hdr := map[string]string{
				"nonce":     "1",
				"repo":      "r1",
				"namespace": "ns1",
				"reference": "refs/heads/master",
				"pkID":      "pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
				"fee":       "10",
			}
			txDetail, err := TxDetailFromPEMHeader(hdr)
			Expect(err).To(BeNil())
			Expect(txDetail.Nonce).To(Equal(uint64(1)))
			Expect(txDetail.RepoName).To(Equal("r1"))
			Expect(txDetail.RepoNamespace).To(Equal("ns1"))
			Expect(txDetail.Reference).To(Equal("refs/heads/master"))
			Expect(txDetail.PushKeyID).To(Equal("pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			Expect(txDetail.Fee).To(Equal(util.String("10")))
		})
	})
})
