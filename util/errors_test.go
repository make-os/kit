package util

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {

	Describe(".StatusError.Error", func() {
		It("should return expected err", func() {
			err := NewStatusError(100, "bad_thing", "name", "something bad here")
			Expect(err).To(MatchError("field:'name', msg:'something bad here', httpCode:'100', code:'bad_thing'"))
		})
	})

	Describe(".StatusErrorFromStr", func() {
		It("should convert a StatusError.Error output back to same StatusError", func() {
			err := NewStatusError(100, "bad_thing", "name", "something bad here")
			out := err.Error()
			err2 := StatusErrorFromStr(out)
			Expect(err).To(Equal(err2))
		})

		When("msg contains a field error format: `msg:'field:id, msg:proposal id has been used, choose "+
			"another', httpCode:'400', code:'mempool_add_fail'`", func() {
			It("should convert successfully without altering the field error", func() {
				out := `msg:'field:id, msg:proposal id has been used, choose another', httpCode:'400', code:'mempool_add_fail'`
				err := StatusErrorFromStr(out)
				Expect(err).To(Equal(&StatusError{
					Code:     "mempool_add_fail",
					HttpCode: 400,
					Msg:      "field:id, msg:proposal id has been used, choose another",
					Field:    "",
				}))
			})
		})
	})

	Describe(".getKeyFromFieldErrOutput", func() {
		When("field error is `field:id, msg:proposal id has been used, choose another, index:1`", func() {
			str := `field:id, msg:proposal id has been used, choose another, index:1`

			It("should return field=id", func() {
				Expect(getKeyFromFieldErrOutput(str, "field")).To(Equal("id"))
			})

			It("should return msg=proposal id has been used, choose another", func() {
				Expect(getKeyFromFieldErrOutput(str, "msg")).To(Equal("proposal id has been used, choose another"))
			})

			It("should return index=1", func() {
				Expect(getKeyFromFieldErrOutput(str, "index")).To(Equal("1"))
			})
		})
	})

	Describe(".BadFieldErrorFromStr", func() {
		When("field error is `field:id, msg:proposal id has been used, choose another, index:1`", func() {
			str := `field:id, msg:proposal id has been used, choose another, index:1`
			It("should return field='id', msg='proposal id has been used, choose another' and index='1'", func() {
				bfe := BadFieldErrorFromStr(str)
				Expect(bfe.Index).To(Equal(1))
				Expect(bfe.Field).To(Equal("id"))
				Expect(bfe.Msg).To(Equal("proposal id has been used, choose another"))
			})
		})
	})
})
