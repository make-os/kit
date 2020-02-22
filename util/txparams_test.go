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

	Describe(".ParseTxParams", func() {
		When("message does not have a txparams", func() {
			It("should return ErrTxParamsNotFound", func() {
				str := "This is a line\nThis is another line\n"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrTxParamsNotFound))
			})
		})

		When("message has a valid txparams", func() {
			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, deleteRef"
				txparams, err := ParseTxParams(str)
				Expect(err).To(BeNil())
				Expect(*txparams).To(Equal(TxParams{
					Fee:       String("10"),
					Nonce:     2,
					PubKeyID:  "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd",
					DeleteRef: true,
				}))
			})

			It("should return no error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				txparams, err := ParseTxParams(str)
				Expect(err).To(BeNil())
				Expect(*txparams).To(Equal(TxParams{
					Fee:       String("10"),
					Nonce:     2,
					PubKeyID:  "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd",
					DeleteRef: false,
				}))
			})
		})

		When("txparams has invalid nonce value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=10, nonce=2a, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg: nonce must be an unsigned integer"))
			})
		})

		When("txparams has invalid fee value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1a, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg: fee must be numeric"))
			})
		})

		When("txparams has signature field but no value", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig="
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature value is required"))
			})
		})

		When("txparams has signature value that does not begin with 0x", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=abc"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature format is not valid"))
			})
		})

		When("txparams has signature value that is not a valid hex", func() {
			It("should return error", func() {
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=0xabc"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg: signature format is not valid"))
			})
		})

		When("txparams has signature format", func() {
			It("should not return error", func() {
				sigHex := ToHex([]byte("abc"))
				str := "This is a line\nThis is another line\ntx: fee=1, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd sig=" + sigHex
				txParams, err := ParseTxParams(str)
				Expect(err).To(BeNil())
				Expect(txParams).To(Equal(&TxParams{
					Fee:       "1",
					Nonce:     0x0000000000000002,
					PubKeyID:  "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd",
					Signature: "abc",
				}))
			})
		})

		When("txparams contains a merge directive with no value", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, mergeId"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge proposal id is required"))
			})
		})

		When("txparams contains a merge directive with invalid value format", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, mergeId=abc12"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge proposal id format is not valid"))
			})
		})

		When("txparams contains a merge directive with length greater than 16", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, mergeId=123456789"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("merge id limit of 8 bytes exceeded"))
			})
		})

		When("txparams contains a merge directive with valid value", func() {
			It("should return no err and set the Merge field to the value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, mergeId=122"
				txparams, err := ParseTxParams(str)
				Expect(err).To(BeNil())
				Expect(txparams.MergeProposalID).To(Equal("122"))
			})
		})

		When("txparams contains a pkId != 44 characters long", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=0x9aed9d"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkId, msg: public key id is invalid"))
			})
		})

		When("txparams contains a pkId does not begin with gpg", func() {
			It("should return err about missing value", func() {
				str := "tx: fee=0.2, nonce=14, pkId=xas1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd"
				_, err := ParseTxParams(str)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:pkId, msg: public key id is invalid"))
			})
		})
	})

	Describe(".MakeTxParams", func() {
		When("signature is nil", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pkID", nil)
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkId=pkID"))
			})
		})

		When("signature is set", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pkID", []byte("abc"))
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkId=pkID, sig=0x616263"))
			})
		})

		When("actions are set", func() {
			It("should return expected string", func() {
				txParams := MakeTxParams("1", "1", "pkID", []byte("abc"), "removeRef", "checkRef")
				Expect(txParams).To(Equal("tx: fee=1, nonce=1, pkId=pkID, removeRef, checkRef, sig=0x616263"))
			})
		})
	})

	Describe("TxParams.String", func() {
		It("should return", func() {
			txParams := &TxParams{
				Fee:       "1",
				Nonce:     0x0000000000000002,
				PubKeyID:  "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd",
				Signature: "abc",
			}
			expected := `tx: fee=1, nonce=2, pkId=gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd, sig=0x616263`
			Expect(txParams.String()).To(Equal(expected))
		})
	})
})
