package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxParams", func() {
	Describe(".RemoveTxParams", func() {
		It("should remove txparams", func() {
			str := "This is a line\nThis is another line\ntx: args args"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxParams(str)).To(Equal(expected))
		})

		It("should not remove txparams if label is capitalized", func() {
			str := "This is a line\nThis is another line\nTX: args args"
			expected := "This is a line\nThis is another line\nTX: args args"
			Expect(RemoveTxParams(str)).To(Equal(expected))
		})

		It("should return exact text when label is not present", func() {
			str := "This is a line\nThis is another line\n"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxParams(str)).To(Equal(expected))
		})
	})

	Describe(".ExtractTxParams", func() {
		When("message does not have a txparams", func() {
			It("should return ErrTxParamsNotFound", func() {
				str := "This is a line\nThis is another line\n"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxParamsNotFound))
			})
		})

		When("message has a valid txparams", func() {
			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, deleteRef"
				txparams, err := ExtractTxParams(str)
				Expect(err).To(BeNil())
				Expect(*txparams).To(Equal(TxParams{
					Fee:       String("10"),
					Nonce:     2,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					DeleteRef: true,
				}))
			})

			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				txparams, err := ExtractTxParams(str)
				Expect(err).To(BeNil())
				Expect(*txparams).To(Equal(TxParams{
					Fee:       String("10"),
					Nonce:     2,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					DeleteRef: false,
				}))
			})
		})

		When("txparams has invalid nonce value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2a, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce must be an unsigned integer"))
			})
		})

		When("txparams has invalid fee value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1a, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:fee must be numeric"))
			})
		})

		When("txparams has signature field but no value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig="
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature value is required"))
			})
		})

		When("txparams has signature value that does not begin with 0x", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig=abc"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature format is not valid"))
			})
		})

		When("txparams has signature value that is not a valid hex", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig=0xabc"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature format is not valid"))
			})
		})

		When("txparams has signature format", func() {
			It("should not return error", func() {
				sigHex := ToHex([]byte("abc"))
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p sig=" + sigHex
				txParams, err := ExtractTxParams(str)
				Expect(err).To(BeNil())
				Expect(txParams).To(Equal(&TxParams{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					Signature: "abc",
				}))
			})
		})

		When("txparams contains a merge directive with no value", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id is required"))
			})
		})

		When("txparams contains a merge directive with invalid value format", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=abc12"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id must be numeric"))
			})
		})

		When("txparams contains a merge directive with length greater than 16", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=123456789"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:mergeID, msg:merge proposal id exceeded 8 bytes limit"))
			})
		})

		When("txparams contains a merge directive with valid value", func() {
			It("should return no err and set the Merge field to the value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, mergeID=122"
				txparams, err := ExtractTxParams(str)
				Expect(err).To(BeNil())
				Expect(txparams.MergeProposalID).To(Equal("122"))
			})
		})

		When("txparams contains a pushKeyID != 44 characters long", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=0x9aed9d"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is invalid"))
			})
		})

		When("txparams contains a pushKeyID does not begin with push", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkID=xas1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is invalid"))
			})
		})

		When("txparams contains an unexpected key", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonze=14, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"
				_, err := ExtractTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:nonze, msg:unknown field"))
			})
		})
	})

	Describe(".MakeTxParams", func() {
		When("signature is nil", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pushKeyID", nil)
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID"))
			})
		})

		When("signature is set", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pushKeyID", []byte("abc"))
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, sig=0x616263"))
			})
		})

		When("actions are set", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pushKeyID", []byte("abc"), "removeRef", "checkRef")
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, removeRef, checkRef, sig=0x616263"))
			})
		})

		When("directive is set", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pushKeyID", []byte("abc"), "deleteRef")
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, deleteRef, sig=0x616263"))
				txParams = MakeTxParams("1", "1", "pushKeyID", []byte("abc"), "deleteRef", "mergeID=123")
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkID=pushKeyID, deleteRef, mergeID=123, sig=0x616263"))
			})
		})
	})

	Describe("TxParams", func() {
		Describe(".String", func() {
			It("should return", func() {
				txParams := &TxParams{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PushKeyID: "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p",
					Signature: "abc",
				}
				expected := `tx: fee=1, nonce=2, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p, sig=0x616263`
				Expect(txParams.String()).To(Equal(expected))
			})
		})

		Describe(".GetNonceAsString", func() {
			It("should return nonce", func() {
				txParams := &TxParams{Fee: "1", Nonce: 100}
				Expect(txParams.GetNonceAsString()).To(Equal("100"))
			})
		})
	})

	Describe(".MakeAndValidateTxParams", func() {
		When("txparam is invalid", func() {
			It("should return err", func() {
				_, err := MakeAndValidateTxParams("1", "1", "", nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkID, msg:push key id is required"))
			})
		})

		When("txparam is valid", func() {
			It("should return txparam string", func() {
				tp, err := MakeAndValidateTxParams("1", "1", "push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p", nil)
				Expect(err).To(BeNil())
				Expect(tp.String()).To(Equal("tx: fee=1, nonce=1, pkID=push1y00fkeju2kdjefvwrlmads83uudjkahun3lj4p"))
			})
		})
	})

})
