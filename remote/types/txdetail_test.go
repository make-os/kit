package types

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			Expect(header).To(HaveLen(1))
			Expect(header).To(HaveKey("pkID"))
			Expect(header["pkID"]).To(Equal("pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
		})
	})

	Describe(".TxDetailFromGitSigPEMHeader", func() {
		It("should return err when pkID is not set", func() {
			hdr := map[string]string{}
			_, err := TxDetailFromGitSigPEMHeader(hdr)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("'pkID' is required"))
		})

		It("should successfully create TxDetail from PEM header", func() {
			hdr := map[string]string{
				"pkID": "pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
			}
			txDetail, err := TxDetailFromGitSigPEMHeader(hdr)
			Expect(err).To(BeNil())
			Expect(txDetail.PushKeyID).To(Equal("pk1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
		})
	})
})
