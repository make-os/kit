package client

import (
	"testing"

	"gitlab.com/makeos/mosdef/rpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = Describe("Client", func() {

	Describe(".NewClient", func() {
		It("should panic when option.host is not set", func() {
			Expect(func() {
				NewClient(nil)
			}).To(Panic())
		})

		It("should panic when option.port is not set", func() {
			Expect(func() {
				opt := rpc.Options{Host: "127.0.0.1"}
				NewClient(&opt)
			}).To(Panic())
		})
	})

	Describe(".Call", func() {
		It("should return error when options haven't been set", func() {
			c := RPCClient{opts: &rpc.Options{Host: "127.0.0.1"}}
			_, err := c.Call("", nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("http client and options not set"))
		})
	})

	Describe(".GetOptions", func() {
		It("should return options", func() {
			opts := &rpc.Options{Host: "hostA", Port: 9000}
			Expect(NewClient(opts).GetOptions()).To(Equal(opts))
		})
	})
})
