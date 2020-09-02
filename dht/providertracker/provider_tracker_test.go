package providertracker_test

import (
	"os"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht/providertracker"
	"github.com/make-os/lobe/dht/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProviderTracker", func() {
	var err error
	var cfg *config.AppConfig
	var tracker *providertracker.ProviderTracker

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		tracker = providertracker.New()
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Register", func() {
		It("should add addresses if not already registered", func() {
			addrs := []peer.AddrInfo{
				{ID: peer.ID("peer1")},
				{ID: peer.ID("peer2")},
			}
			tracker.Register(addrs...)
			Expect(tracker.NumProviders()).To(Equal(2))
		})
	})

	Describe(".Get", func() {
		It("should pass provider info to callback if provider exist", func() {
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			var found *types.ProviderInfo
			retval := tracker.Get(peerID, func(info *types.ProviderInfo) {
				found = info
			})
			Expect(retval).To(Equal(found))
			Expect(found).ToNot(BeNil())
			Expect(found.Addr.ID).To(Equal(peerID))
			Expect(found.LastFailure).To(BeZero())
			Expect(found.Failed).To(BeZero())
			Expect(found.LastSeen).ToNot(BeZero())
		})

		It("should return nil if provider does not exist", func() {
			peerID := peer.ID("peer1")
			var found *types.ProviderInfo
			retval := tracker.Get(peerID, func(info *types.ProviderInfo) {
				found = info
			})
			Expect(retval).To(BeNil())
			Expect(found).To(BeNil())
		})
	})

	Describe(".Ban", func() {
		It("should add peer to ban cache if not already added", func() {
			peerID := peer.ID("peer1")
			tracker.Ban(peerID, 10*time.Second)
			expTime := tracker.BanCache().Get(peerID.Pretty())
			Expect(expTime).ToNot(BeNil())
			Expect(expTime.(*time.Time).Sub(time.Now()).Seconds() > 9).To(BeTrue())
		})

		It("should add duration to a peer's expiry time if peer already exist in the ban cache", func() {
			peerID := peer.ID("peer1")
			tracker.Ban(peerID, 10*time.Second)
			expTime := tracker.BanCache().Get(peerID.Pretty())

			tracker.Ban(peerID, 10*time.Second)
			expTime = tracker.BanCache().Get(peerID.Pretty())

			Expect(expTime).ToNot(BeNil())
			Expect(expTime.(*time.Time).Sub(time.Now()).Seconds() > 19).To(BeTrue())
		})
	})

	Describe(".MarkFailure", func() {
		It("should increment peer's failure count", func() {
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.MarkFailure(peerID)
			info := tracker.Get(peerID, nil)
			Expect(info.Failed).To(Equal(1))
			Expect(info.LastFailure).ToNot(BeZero())
		})

		It("should ban peer when its failure count reached MaxFailureBeforeBan", func() {
			providertracker.MaxFailureBeforeBan = 2
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.MarkFailure(peerID)
			tracker.MarkFailure(peerID)
			info := tracker.Get(peerID, nil)
			Expect(info.Failed).To(Equal(2))
			expTime := tracker.BanCache().Get(peerID.Pretty())
			Expect(expTime).ToNot(BeNil())
		})
	})

	Describe(".MarkSeen", func() {
		It("should reset peer's fail count", func() {
			providertracker.MaxFailureBeforeBan = 2
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.MarkFailure(peerID)
			info := tracker.Get(peerID, nil)
			Expect(info.Failed).To(Equal(1))

			info2 := tracker.Get(peerID, nil)
			tracker.MarkSeen(peerID)
			Expect(info2.Failed).To(Equal(0))
			Expect(info2).ToNot(Equal(info.LastSeen))
		})
	})

	Describe(".IsGood", func() {
		It("should return true if peer is not banned", func() {
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			good := tracker.IsGood(peerID)
			Expect(good).To(BeTrue())
		})

		It("should return false if peer is banned", func() {
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.Ban(peerID, 10*time.Second)
			good := tracker.IsGood(peerID)
			Expect(good).To(BeFalse())
		})

		It("should return true if peer is not registered", func() {
			peerID := peer.ID("peer1")
			good := tracker.IsGood(peerID)
			Expect(good).To(BeTrue())
		})

		It("should return false if peer is registered and last failure is within back off duration", func() {
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.MarkFailure(peerID)
			good := tracker.IsGood(peerID)
			Expect(good).To(BeFalse())
		})

		It("should return true if peer is registered and last failure is not within back off duration", func() {
			providertracker.BackOffDurAfterFailure = 1 * time.Millisecond
			peerID := peer.ID("peer1")
			addrs := []peer.AddrInfo{{ID: peerID}}
			tracker.Register(addrs...)
			tracker.MarkFailure(peerID)
			time.Sleep(2 * time.Millisecond)
			good := tracker.IsGood(peerID)
			Expect(good).To(BeTrue())
		})
	})

	Describe(".PeerSentNope & .DidPeerSendNope", func() {
		key := util.RandBytes(5)
		peerID := peer.ID("peer1")

		BeforeEach(func() {
			Expect(tracker.DidPeerSendNope(peerID, key)).To(BeFalse())
			tracker.PeerSentNope(peerID, key)
		})

		It("should return true if peer+key entry exist", func() {
			Expect(tracker.DidPeerSendNope(peerID, key)).To(BeTrue())
		})
	})
})
