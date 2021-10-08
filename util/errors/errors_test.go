package errors

import (
	"testing"

	"github.com/k0kubun/pp"
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
			Expect(err).To(MatchError("field:'name', msg:'something bad here', httpCode:'100', code:'bad_thing'"))
		})
	})

	Describe(".ReqErrorFromStr", func() {
		It("should convert a ReqError.Error output back to same ReqError", func() {
			err := ReqErr(100, "bad_thing", "name", "something bad here")
			out := err.Error()
			err2 := ReqErrorFromStr(out)
			Expect(err).To(Equal(err2))
		})

		When("msg contains a field error format: `msg:'field:id, msg:proposal id has been used, choose "+
			"another', httpCode:'400', code:'err_mempool'`", func() {
			It("should convert successfully without altering the field error", func() {
				out := `msg:'field:id, msg:proposal id has been used, choose another', httpCode:'400', code:'err_mempool'`
				err := ReqErrorFromStr(out)
				Expect(err).To(Equal(&ReqError{
					Code:     "err_mempool",
					HttpCode: 400,
					Msg:      "field:id, msg:proposal id has been used, choose another",
					Field:    "",
				}))
			})
		})

		When("msg contains a field error format: `msg:'field:id, msg:user's name is required', httpCode:'400', code:'err_mempool'`", func() {
			It("should convert successfully without altering the field error", func() {
				out := `msg:'field:id, msg:user's name is required', httpCode:'400', code:'err_mempool'`
				err := ReqErrorFromStr(out)
				Expect(err).To(Equal(&ReqError{
					Code:     "err_mempool",
					HttpCode: 400,
					Msg:      "field:id, msg:user's name is required",
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

	FDescribe(".BadFieldErrorFromStr", func() {
		When("case 1", func() {
			str := `field:id, msg:proposal id has been used, choose another, index:1`
			It("should return field='id', msg='proposal id has been used, choose another' and index='1'", func() {
				bfe := BadFieldErrorFromStr(str)
				Expect(bfe.Index).To(Equal(1))
				Expect(bfe.Field).To(Equal("id"))
				Expect(bfe.Msg).To(Equal("proposal id has been used, choose another"))
			})
		})

		FWhen("case 2", func() {
			It("should return field='id', msg='when proposer type is not `ProposerOwner`, tally methods `CoinWeighted` and `Identity` are not allowed'", func() {
				str := "field:config, msg:when proposer type is not `ProposerOwner`, tally methods `CoinWeighted` and `Identity` are not allowed"
				bfe := BadFieldErrorFromStr(str)
				pp.Println(bfe)
				// Expect(bfe.Index).To(Equal(1))
				// Expect(bfe.Field).To(Equal("id"))
				// Expect(bfe.Msg).To(Equal("proposal id has been used, choose another"))
			})
		})
	})
})
