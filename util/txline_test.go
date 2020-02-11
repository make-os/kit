package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxLine", func() {
	Describe(".RemoveTxLine", func() {
		It("should remove tx line", func() {
			str := "This is a line\nThis is another line\ntx: args args"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxLine(str)).To(Equal(expected))
		})

		It("should not remove tx line if label is capitalized", func() {
			str := "This is a line\nThis is another line\nTX: args args"
			expected := "This is a line\nThis is another line\nTX: args args"
			Expect(RemoveTxLine(str)).To(Equal(expected))
		})

		It("should return exact text when label is not present", func() {
			str := "This is a line\nThis is another line\n"
			expected := "This is a line\nThis is another line\n"
			Expect(RemoveTxLine(str)).To(Equal(expected))
		})
	})

	Describe(".ParseTxLine", func() {
		When("message does not have a txline", func() {
			It("should return ErrTxLineNotFound", func() {
				str := "This is a line\nThis is another line\n"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxLineNotFound))
			})
		})

		When("message has a valid txline", func() {
			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00, deleteRef"
				txline, err := ParseTxLine(str)
				Expect(err).To(BeNil())
				Expect(*txline).To(Equal(TxLine{
					Fee:       String("10"),
					Nonce:     2,
					PubKeyID:  "0x9aed9dbda362c75e9feaa07241aac207d5ef4e00",
					DeleteRef: true,
				}))
			})

			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00"
				txline, err := ParseTxLine(str)
				Expect(err).To(BeNil())
				Expect(*txline).To(Equal(TxLine{
					Fee:       String("10"),
					Nonce:     2,
					PubKeyID:  "0x9aed9dbda362c75e9feaa07241aac207d5ef4e00",
					DeleteRef: false,
				}))
			})
		})

		When("txline has invalid nonce value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2a, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg: nonce must be an unsigned integer"))
			})
		})

		When("txline has invalid fee value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1a, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg: fee must be numeric"))
			})
		})

		When("txline has signature field but no value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00 sig="
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature value is required"))
			})
		})

		When("txline has signature value that does not begin with 0x", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00 sig=abc"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature format is not valid"))
			})
		})

		When("txline has signature value that is not a valid hex", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00 sig=0xabc"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature format is not valid"))
			})
		})

		When("txline has signature format", func() {
			It("should not return error", func() {
				sigHex := ToHex([]byte("abc"))
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00 sig=" + sigHex
				txLine, err := ParseTxLine(str)
				Expect(err).To(BeNil())
				Expect(txLine).To(Equal(&TxLine{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PubKeyID:  "0x9aed9dbda362c75e9feaa07241aac207d5ef4e00",
					Signature: "abc",
				}))
			})
		})

		When("txline contains a merge directive with no value", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00, merge"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target branch to merge is required"))
			})
		})

		When("txline contains a merge directive with invalid value format", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00, merge=repo&:branch"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("target branch format is not valid"))
			})
		})

		When("txline contains a merge directive with valid value", func() {
			It("should return no err and set the Merge field to the value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00, merge=repo:branch"
				txline, err := ParseTxLine(str)
				Expect(err).To(BeNil())
				Expect(txline.Merge).To(Equal("repo:branch"))
			})
		})
	})

	Describe(".MakeTxLine", func() {
		When("signature is nil", func() {
			It("should return expected string", func() {
				txLine := MakeTxLine("1", "1", "pkID", nil)
				Expect(txLine).To(Equal("tx: fee=1, nonce=1, pkId=pkID"))
			})
		})

		When("signature is set", func() {
			It("should return expected string", func() {
				txLine := MakeTxLine("1", "1", "pkID", []byte("abc"))
				Expect(txLine).To(Equal("tx: fee=1, nonce=1, pkId=pkID, sig=0x616263"))
			})
		})

		When("actions are set", func() {
			It("should return expected string", func() {
				txLine := MakeTxLine("1", "1", "pkID", []byte("abc"), "removeRef", "checkRef")
				Expect(txLine).To(Equal("tx: fee=1, nonce=1, pkId=pkID, removeRef, checkRef, sig=0x616263"))
			})
		})
	})

	Describe("TxLine.String", func() {
		It("should return", func() {
			txLine := &TxLine{
				Fee:       "1",
				Nonce:     0x0000000000000002,
				PubKeyID:  "0x9aed9dbda362c75e9feaa07241aac207d5ef4e00",
				Signature: "abc",
			}
			expected := `tx: fee=1, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00, sig=0x616263`
			Expect(txLine.String()).To(Equal(expected))
		})
	})
})
