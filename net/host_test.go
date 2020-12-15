package net_test

import (
	"context"
	"os"
	"testing"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/net"
	"github.com/make-os/kit/testutil"
	"github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Net Suite")
}

var _ = Describe("Host", func() {
	var cfg *config.AppConfig
	var err error

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".New", func() {
		It("should return error when address is invalid", func() {
			cfg.DHT.Address = "****"
			_, err := net.New(context.Background(), cfg)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid address: address ****: missing port in address"))
		})

		It("should return error when unable to start host", func() {
			cfg.DHT.Address = "12:00"
			_, err := net.New(context.Background(), cfg)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to create host"))
		})
	})

	Describe(".Get", func() {
		It("should return wrapped host", func() {
			cfg.DHT.Address = testutil.RandomAddr()
			host, err := net.New(context.Background(), cfg)
			Expect(err).To(BeNil())
			Expect(host.Get()).To(Not(BeNil()))
		})
	})

	Describe(".ID", func() {
		It("should return host ID", func() {
			cfg.DHT.Address = testutil.RandomAddr()
			host, err := net.New(context.Background(), cfg)
			Expect(err).To(BeNil())
			Expect(host.ID()).To(Not(BeEmpty()))
		})
	})

	Describe(".Addrs", func() {
		It("should return host listening addresses", func() {
			cfg.DHT.Address = testutil.RandomAddr()
			host, err := net.New(context.Background(), cfg)
			Expect(err).To(BeNil())
			Expect(host.Addrs()).To(HaveLen(1))
		})
	})

	Describe(".FullAddr", func() {
		It("should return host full address", func() {
			cfg.DHT.Address = testutil.RandomAddr()
			host, err := net.New(context.Background(), cfg)
			Expect(err).To(BeNil())
			addr := host.FullAddr()
			Expect(addr).To(Not(BeEmpty()))
			Expect(func() {
				multiaddr.StringCast(addr)
			}).ToNot(Panic())
		})
	})
})
