package announcer_test

import (
	"context"
	"os"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht/announcer"
	"github.com/make-os/lobe/dht/server"
	"github.com/make-os/lobe/mocks"
	testutil2 "github.com/make-os/lobe/remote/testutil"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Announcer", func() {
	var err error
	var cfg, cfg2 *config.AppConfig
	var ctrl *gomock.Controller
	var dhtA *server.Server
	var dhtB *server.Server
	var ann *announcer.Announcer
	var mockDHTKeeper *mocks.MockDHTKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		cfg.DHT.Address = testutil2.RandomAddr()
		dhtA, err = server.New(context.Background(), nil, cfg)
		Expect(err).To(BeNil())

		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2.DHT.Address = testutil2.RandomAddr()

		logic := testutil.MockLogic(ctrl)
		mockDHTKeeper = logic.DHTKeeper
		ann = announcer.New(dhtA.DHT(), logic.Logic, cfg.G().Log)
	})

	AfterEach(func() {
		ctrl.Finish()
		dhtA.Stop()
		if dhtB != nil {
			dhtB.Stop()
		}
		ann.Stop()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".HasTask", func() {
		It("should return false if queue is empty", func() {
			Expect(ann.HasTask()).To(BeFalse())
		})
	})

	Describe(".Announce", func() {
		It("should add task to queue", func() {
			ann.Announce(1, "", []byte("key"), nil)
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
		key := []byte("key")

		When("task.CheckExistence is true", func() {
			It("should return ErrDelisted and remove key from announce list if no checker for the key type is set", func() {
				mockDHTKeeper.EXPECT().RemoveFromAnnounceList(key)
				err := ann.Do(1, &announcer.Task{Key: key, CheckExistence: true, Done: nil})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(announcer.ErrDelisted))
			})

			It("should return ErrDelisted and remove key from announce list if checker returns false", func() {
				ann.RegisterChecker(1, func(repo string, k []byte) bool {
					Expect(key).To(Equal(k))
					return false
				})
				err := ann.Do(1, &announcer.Task{Key: key, Type: 1, CheckExistence: true, Done: nil})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(announcer.ErrDelisted))
			})
		})

		When("Connected to a DHT peer", func() {
			BeforeEach(func() {
				dhtB, err = server.New(context.Background(), nil, cfg2)
				Expect(err).To(BeNil())
				cfg.DHT.BootstrapPeers = dhtB.Addr()
				err = dhtA.Bootstrap()
				Expect(err).To(BeNil())
			})

			It("should re-add key to announce list and return nil error", func() {
				mockDHTKeeper.EXPECT().AddToAnnounceList(key, "repo1", 1, gomock.Any())
				err := ann.Do(1, &announcer.Task{Key: key, RepoName: "repo1", Type: 1, Done: func(err error) {}})
				Expect(err).To(BeNil())
			})
		})

		When("Not connected to a DHT peer", func() {
			It("should return error", func() {
				announcer.MaxRetry = 0
				err := ann.Do(1, &announcer.Task{Key: key, RepoName: "repo1", Type: 1, Done: func(err error) {}})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to find any peer in table"))
			})
		})
	})

	Describe(".Reannounce", func() {
		It("should re-add keys where NextTime equal to current time or is before current time", func() {
			now := time.Now()
			mockDHTKeeper.EXPECT().IterateAnnounceList(gomock.Any()).Do(func(it func(key []byte, entry *core.AnnounceListEntry)) {
				it([]byte("key"), &core.AnnounceListEntry{Type: 1, Repo: "repo1", NextTime: now.Add(-1 * time.Minute).Unix()})
				it([]byte("key2"), &core.AnnounceListEntry{Type: 1, Repo: "repo1", NextTime: now.Unix()})
				it([]byte("key3"), &core.AnnounceListEntry{Type: 1, Repo: "repo1", NextTime: now.Add(1 * time.Minute).Unix()})
			})
			ann.Reannounce()
			Expect(ann.QueueSize()).To(Equal(2))
		})
	})
})
