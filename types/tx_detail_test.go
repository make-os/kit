package types

import (
	"github.com/mr-tron/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("TxDetail", func() {
	Describe(".RemoveTxDetail", func() {
		It("should remove txDetail", func() {
			str := "This is a line\nThis is another line\ntx: args args"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxDetail(str)).To(Equal(expected))
		})

		It("should not remove txDetail if label is capitalized", func() {
			str := "This is a line\nThis is another line\nTX: args args"
			expected := "This is a line\nThis is another line\nTX: args args"
			Expect(RemoveTxDetail(str)).To(Equal(expected))
		})

		It("should return exact text when label is not present", func() {
			str := "This is a line\nThis is another line\n"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxDetail(str)).To(Equal(expected))
		})
	})

	Describe(".ExtractTxDetail", func() {
		When("message does not have a txDetail", func() {
			It("should return ErrTxDetailNotFound", func() {
				str := "This is a line\nThis is another line\n"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxDetailNotFound))
			})
		})

		When("message has a valid txDetail", func() {
			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				txDetail, err := ExtractTxDetail(str)
				Expect(err).To(BeNil())
				Expect(*txDetail).To(Equal(TxDetail{
					Fee:       "10",
					Nonce:     2,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
				}))
			})
		})

		When("txDetail has 'repo'", func() {
			When("it has no value", func() {
				It("should return error", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, repo"
					_, err := ExtractTxDetail(str)
					Expect(err).To(MatchError("field:repo, msg:target repo name is required"))
				})
			})

			When("it has a value", func() {
				It("should return error", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, repo=repo1"
					tp, err := ExtractTxDetail(str)
					Expect(err).To(BeNil())
					Expect(tp.RepoName).To(Equal("repo1"))
				})
			})
		})

		When("txDetail has 'repo'", func() {
			When("it has no value", func() {
				Specify("that RepoNamespace field is unset", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, namespace"
					tp, err := ExtractTxDetail(str)
					Expect(err).To(BeNil())
					Expect(tp.RepoNamespace).To(BeEmpty())
				})
			})

			When("with a value", func() {
				Specify("that RepoNamespace field is set", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, namespace=ns1"
					tp, err := ExtractTxDetail(str)
					Expect(err).To(BeNil())
					Expect(tp.RepoNamespace).To(Equal("ns1"))
				})
			})
		})

		When("txDetail has 'reference'", func() {
			When("has no value", func() {
				It("should return error", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, reference"
					_, err := ExtractTxDetail(str)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("field:reference, msg:target reference name is required"))
				})
			})

			When("has a value", func() {
				Specify("that Reference is set", func() {
					str := "tx: fee=10, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, reference=refs/heads/master"
					tp, err := ExtractTxDetail(str)
					Expect(err).To(BeNil())
					Expect(tp.Reference).To(Equal("refs/heads/master"))
				})
			})
		})

		When("txDetail has invalid nonce value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2a, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce must be an unsigned integer"))
			})
		})

		When("txDetail has invalid fee value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1a, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:fee must be numeric"))
			})
		})

		When("txDetail has signature field but no value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig="
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature value is required"))
			})
		})

		When("txDetail has signature value that does not a valid base58 encoding", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig=ab*c"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature format is not valid"))
			})
		})

		When("txDetail has valid signature format", func() {
			It("should not return error", func() {
				sig := base58.Encode([]byte("abc"))
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig=" + sig
				txDetail, err := ExtractTxDetail(str)
				Expect(err).To(BeNil())
				Expect(txDetail).To(Equal(&TxDetail{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					Signature: sig,
				}))
			})
		})

		When("txDetail contains a head option but it is not set", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, head"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:head, msg:value is required"))
			})
		})

		When("txDetail contains invalid head value", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, head=abc"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:head, msg:expect a valid object hash"))
			})
		})

		When("txDetail contains a merge option with no value", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id is required"))
			})
		})

		When("txDetail contains a merge option with invalid value format", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=abc12"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id must be numeric"))
			})
		})

		When("txDetail contains a merge option with length greater than 16", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=123456789"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id exceeded 8 bytes limit"))
			})
		})

		When("txDetail contains a merge option with valid value", func() {
			It("should return no err and set the Merge field to the value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=122"
				txDetail, err := ExtractTxDetail(str)
				Expect(err).To(BeNil())
				Expect(txDetail.MergeProposalID).To(Equal("122"))
			})
		})

		When("txDetail contains a pushKeyID != 44 characters long", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=0x9aed9d"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is invalid"))
			})
		})

		When("txDetail contains a pushKeyID does not begin with push", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=xas1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is invalid"))
			})
		})

		When("txDetail contains an unexpected key", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonze=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxDetail(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:nonze, msg:unknown field"))
			})
		})
	})

	Describe(".MakeTxDetail", func() {
		When("signature is nil", func() {
			It("should return expected string", func() {
				txDetail := MakeTxDetail("1", "1", "pushKeyID", nil)
				Expect(txDetail).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID"))
			})
		})

		When("signature is set", func() {
			It("should return expected string", func() {
				txDetail := MakeTxDetail("1", "1", "pushKeyID", []byte("abc"))
				Expect(txDetail).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, sig=ZiCa"))
			})
		})

		When("actions are set", func() {
			It("should return expected string", func() {
				txDetail := MakeTxDetail("1", "1", "pushKeyID", []byte("abc"), "removeRef", "checkRef")
				Expect(txDetail).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, removeRef, checkRef, sig=ZiCa"))
			})
		})

		When("option is set", func() {
			It("should return expected string", func() {
				txDetail := MakeTxDetail("1", "1", "pushKeyID", []byte("abc"), "deleteRef")
				Expect(txDetail).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, deleteRef, sig=ZiCa"))
				txDetail = MakeTxDetail("1", "1", "pushKeyID", []byte("abc"), "deleteRef", "mergeID=123")
				Expect(txDetail).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, deleteRef, mergeID=123, sig=ZiCa"))
			})
		})
	})

	Describe("TxDetail", func() {
		Describe(".String", func() {
			It("should return", func() {
				txDetail := &TxDetail{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					Signature: "abc",
				}
				expected := `tx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, sig=abc`
				Expect(txDetail.String()).To(Equal(expected))
			})
		})

		Describe(".GetNonceAsString", func() {
			It("should return nonce", func() {
				txDetail := &TxDetail{Fee: "1", Nonce: 100}
				Expect(txDetail.GetNonceAsString()).To(Equal("100"))
			})
		})
	})

	Describe(".MakeAndValidateTxDetail", func() {
		When("txparam is invalid", func() {
			It("should return err", func() {
				_, err := MakeAndValidateTxDetail("1", "1", "", nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is required"))
			})
		})

		When("txparam is valid", func() {
			It("should return txparam string", func() {
				tp, err := MakeAndValidateTxDetail("1", "1", "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p", nil)
				Expect(err).To(BeNil())
				Expect(tp.String()).To(Equal("tx: fee=1, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			})
		})
	})

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
				"pkID":      "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
				"fee":       "10",
			}
			txDetail, err := TxDetailFromPEMHeader(hdr)
			Expect(err).To(BeNil())
			Expect(txDetail.Nonce).To(Equal(uint64(1)))
			Expect(txDetail.RepoName).To(Equal("r1"))
			Expect(txDetail.RepoNamespace).To(Equal("ns1"))
			Expect(txDetail.Reference).To(Equal("refs/heads/master"))
			Expect(txDetail.PushKeyID).To(Equal("push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			Expect(txDetail.Fee).To(Equal(util.String("10")))
		})
	})
})
