package announcer_test

import (
	"context"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/dht/announcer"
	"gitlab.com/makeos/mosdef/dht/server"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Announcer", func() {
	var err error
	var addr string
	var cfg, cfg2 *config.AppConfig
	var key = crypto.NewKeyFromIntSeed(1)
	var ctrl *gomock.Controller
	var key2 = crypto.NewKeyFromIntSeed(2)
	var dhtB *server.Server
	var dhtA *server.Server
	var ann *announcer.BasicAnnouncer

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		port := freeport.GetPort()
		addr = fmt.Sprintf("127.0.0.1:%d", port)

		dhtA, err = server.New(context.Background(), cfg, key.PrivKey().Key(), addr)
		Expect(err).To(BeNil())

		dhtB, err = server.New(context.Background(), cfg2, key2.PrivKey().Key(), testutil2.RandomAddr())
		Expect(err).To(BeNil())

		ann = announcer.NewBasicAnnouncer(dhtA.DHT(), 1, cfg.G().Log)
	})

	AfterEach(func() {
		ctrl.Finish()

		if dhtA != nil {
			dhtA.Stop()
		}

		if dhtB != nil {
			dhtB.Stop()
		}

		ann.Stop()

		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
		err = os.RemoveAll(cfg2.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".HasTask", func() {
		It("should return false if queue is empty", func() {
			Expect(ann.HasTask()).To(BeFalse())
		})
	})

	Describe(".Announce", func() {
		It("should add task to queue", func() {
			ann.Announce([]byte("key"), nil)
			Expect(ann.HasTask()).To(BeTrue())
		})
	})

	Describe(".IsRunning", func() {
		It("should return false if not started", func() {
			Expect(ann.IsRunning()).To(BeFalse())
			ann.Start()
			Expect(ann.IsRunning()).To(BeTrue())
		})
	})

	Describe(".Do", func() {
		var err error
		var key = []byte("key1")

		BeforeEach(func() {
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			err = ann.Do(0, &announcer.Task{Key: key, Done: func(err error) {}})
		})

		It("should return nil announcement succeeds", func() {
			Expect(err).To(BeNil())
		})

		Specify("that connected peer can find the key", func() {
			providers, err := dhtB.GetProviders(context.Background(), key)
			Expect(err).To(BeNil())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
		})

		Specify("that announcing peer can find the key", func() {
			providers, err := dhtA.GetProviders(context.Background(), key)
			Expect(err).To(BeNil())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
		})
	})
})
