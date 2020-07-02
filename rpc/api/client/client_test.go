package client

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/rpc"
)

var _ = Describe("Client", func() {

	Describe(".NewClient", func() {
		When("host is not set", func() {
			It("should panic", func() {
				Expect(func() { NewClient(&Options{}) }).To(Panic())
			})
		})

		When("port is not set", func() {
			It("should panic", func() {
				Expect(func() { NewClient(&Options{Host: "127.0.0.1"}) }).To(Panic())
			})
		})

		When("host and port are set", func() {
			It("should not panic", func() {
				Expect(func() { NewClient(&Options{Host: "127.0.0.1", Port: 5000}) }).ToNot(Panic())
			})
		})
	})

	Describe(".Call", func() {
		It("should return error when options haven't been set", func() {
			c := RPCClient{opts: &Options{Host: "127.0.0.1"}}
			_, _, err := c.Call("", nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("http client and options not set"))
		})
	})

	Describe(".GetOptions", func() {
		It("should return options", func() {
			opts := &Options{Host: "hostA", Port: 9000}
			Expect(NewClient(opts).GetOptions()).To(Equal(opts))
		})
	})

	Describe(".makeClientStatusErr", func() {
		It("should return a StatusErr that describes a client error", func() {
			err := makeClientStatusErr("something bad on client: code %d", 11)
			Expect(err.Field).To(Equal(""))
			Expect(err.Code).To(Equal("client_error"))
			Expect(err.HttpCode).To(Equal(0))
			Expect(err.Msg).To(Equal("something bad on client: code 11"))
		})
	})

	Describe(".makeStatusErrorFromCallErr", func() {
		When("error does not contain a json object string", func() {
			It("should create unexpected_error", func() {
				err := makeStatusErrorFromCallErr(500, fmt.Errorf("some bad error"))
				Expect(err.HttpCode).To(Equal(500))
				Expect(err.Msg).To(Equal("some bad error"))
				Expect(err.Code).To(Equal(ErrCodeUnexpected))
				Expect(err.Field).To(Equal(""))
			})
		})

		When("status code is not 0 and error is json encoding of rpc.Response", func() {
			It("should return StatusErr populated with values from the encoded rpc.Response", func() {
				err := rpc.Response{Err: &rpc.Err{Code: "bad_code", Message: "we have a problem", Data: "bad_field"}}
				se := makeStatusErrorFromCallErr(500, fmt.Errorf(`%s`, err.ToJSON()))
				Expect(se.Code).To(Equal("bad_code"))
				Expect(se.HttpCode).To(Equal(500))
				Expect(se.Msg).To(Equal("we have a problem"))
				Expect(se.Field).To(Equal("bad_field"))
			})
		})
	})
})
