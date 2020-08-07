package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("TxDetail", func() {

	Describe(".GetGitSigPEMHeader", func() {
		It("should return expected header fields and their value", func() {
			detail := &TxDetail{
				RepoName:        "repo1",
				RepoNamespace:   "namespace",
				Reference:       "refs/heads/master",
				Fee:             "10.2",
				Value:           "12.3",
				Nonce:           1,
				PushKeyID:       "pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
				Signature:       "sig1",
				MergeProposalID: "1000",
			}
			header := detail.GetGitSigPEMHeader()
			Expect(header).ToNot(HaveKey("mergeID"))
			Expect(header).To(HaveKey("fee"))
			Expect(header["fee"]).To(Equal("10.2"))
			Expect(header).To(HaveKey("value"))
			Expect(header["value"]).To(Equal("12.3"))
			Expect(header).To(HaveKey("repo"))
			Expect(header["repo"]).To(Equal("repo1"))
			Expect(header).To(HaveKey("namespace"))
			Expect(header["namespace"]).To(Equal("namespace"))
			Expect(header).To(HaveKey("reference"))
			Expect(header["reference"]).To(Equal("refs/heads/master"))
			Expect(header).To(HaveKey("pkID"))
			Expect(header["pkID"]).To(Equal("pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			Expect(header).To(HaveKey("nonce"))
			Expect(header["nonce"]).To(Equal("1"))
		})
	})

	Describe(".TxDetailFromGitSigPEMHeader", func() {
		It("should return err when nonce is not a number", func() {
			hdr := map[string]string{"nonce": "0x"}
			_, err := TxDetailFromGitSigPEMHeader(hdr)
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
				"value":     "12.3",
			}
			txDetail, err := TxDetailFromGitSigPEMHeader(hdr)
			Expect(err).To(BeNil())
			Expect(txDetail.Nonce).To(Equal(uint64(1)))
			Expect(txDetail.RepoName).To(Equal("r1"))
			Expect(txDetail.RepoNamespace).To(Equal("ns1"))
			Expect(txDetail.Reference).To(Equal("refs/heads/master"))
			Expect(txDetail.PushKeyID).To(Equal("pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			Expect(txDetail.Fee).To(Equal(util.String("10")))
			Expect(txDetail.Value).To(Equal(util.String("12.3")))
		})
	})
})
