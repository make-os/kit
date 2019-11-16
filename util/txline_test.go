package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("TxLine", func() {
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

		When("txline is malformed", func() {
			It("should return ErrTxLineMalformed", func() {
				str := "This is a line\nThis is another line\ntx: fee10"
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxLineMalformed))
			})

			It("should return ErrTxLineMalformed", func() {
				str := "This is a line\nThis is another line\ntx:fee=10 - "
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxLineMalformed))
			})

			It("should return ErrTxLineMalformed", func() {
				str := "This is a line\nThis is another line\ntx: fee=10/nonce=2 "
				_, err := ParseTxLine(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxLineMalformed))
			})
		})

		When("message has a valid txline", func() {
			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkId=0x9aed9dbda362c75e9feaa07241aac207d5ef4e00"
				txline, err := ParseTxLine(str)
				Expect(err).To(BeNil())
				Expect(*txline).To(Equal(TxLine{
					Fee:      String("10"),
					Nonce:    2,
					PubKeyID: "0x9aed9dbda362c75e9feaa07241aac207d5ef4e00",
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
	})
})
