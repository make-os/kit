package errors

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestErrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Errors Suite")
}

var _ = Describe("Errors", func() {

	Describe(".ReqError.Error", func() {
		It("should return expected err", func() {
			err := ReqErr(100, "bad_thing", "name", "something bad here")
			Expect(err).To(MatchError(`"code":"bad_thing","field":"name","httpCode":"100","msg":"something bad here"`))
		})
	})

	Describe("ReqError.IsSet", func() {
		It("should return correct result", func() {
			err := ReqErr(100, "bad_thing", "name", "something bad here")
			Expect(err.IsSet()).To(BeTrue())

			empty := &ReqError{}
			Expect(empty.IsSet()).To(BeFalse())
		})
	})

	Describe(".ReqErrorFromStr", func() {
		It("should convert a ReqError.Error output back to same ReqError", func() {
			err := ReqErr(100, "bad_thing", "name", "something bad here")
			out := err.Error()
			Expect(out).To(Equal(err.String()))
			err2 := ReqErrorFromStr(out)
			Expect(err).To(Equal(err2))
		})

		When("param is malformed", func() {
			It("should return a ReqErr with message set to the malformed message", func() {
				param := "field:a, some:thing"
				Expect(ReqErrorFromStr(param).Msg).To(Equal(param))
			})
		})
	})

	Describe(".BadFieldErrorFromStr", func() {
		It("should inflate and return BadFieldError", func() {
			bfe := FieldErrorWithIndex(-1, "a_field", "a message", "some_data")
			bfe2 := BadFieldErrorFromStr(bfe.Error())
			bfe2.Data = "some_data"
			Expect(bfe).To(Equal(bfe2))

			bfe = FieldErrorWithIndex(10, "a_field", "a message")
			bfe2 = BadFieldErrorFromStr(bfe.Error())
			Expect(bfe).To(Equal(bfe2))

			Expect(bfe2.Is(&BadFieldError{})).To(BeTrue())
		})

		When("param is malformed", func() {
			It("should return a BadFieldError with message set to the malformed message", func() {
				param := "field:a, some:thing"
				Expect(BadFieldErrorFromStr(param).Msg).To(Equal(param))
			})
		})
	})

	Describe(".CallIfNil", func() {
		It("should have expected behaviour", func() {
			var n = 0
			err := CallIfNil(nil, func() error {
				n++
				return nil
			})
			Expect(err).To(BeNil())
			Expect(n).To(Equal(1))

			n = 0
			param := fmt.Errorf("err")
			err = CallIfNil(param, func() error {
				n++
				return nil
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(param))
			Expect(n).To(Equal(0))
		})
	})

	Describe(".FieldError", func() {
		It("should return correct error", func() {
			err := FieldError("a_field", "message")
			err2 := BadFieldErrorFromStr(err.Error())
			Expect(err).To(Equal(err2))
		})
	})
})
