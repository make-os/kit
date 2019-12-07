package dht

import (
	"context"
	"fmt"
	"os"

	"github.com/phayes/freeport"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("App", func() {
	var err error
	var addr string
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	BeforeEach(func() {
		port := freeport.GetPort()
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".New", func() {
		var key = crypto.NewKeyFromIntSeed(1)

		When("address format is not valid", func() {
			It("should return err", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Wrapped(), "invalid")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid address: address invalid: missing port in address"))
			})
		})

		When("unable to create host", func() {
			It("should return err", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Wrapped(), "0.1.1.1.0:999999")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to create host"))
			})
		})

		When("no problem", func() {
			It("should return nil", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Wrapped(), addr)
				Expect(err).To(BeNil())
			})
		})
	})
})
