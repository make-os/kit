package announcer_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	mocks2 "github.com/make-os/kit/mocks/net"
	"github.com/make-os/kit/net"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/net/dht/announcer"
	"github.com/make-os/kit/net/dht/server"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAnnouncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Announcer Suite")
}

var _ = Describe("Announcer", func() {
	var err error
	var cfg, cfg2 *config.AppConfig
	var ctrl *gomock.Controller
	var dhtA *server.Server
	var dhtB *server.Server
	var ann *announcer.Announcer
	var mockDHTKeeper *mocks.MockDHTKeeper
	var mockObjects *testutil.MockObjects

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		cfg.DHT.Address = testutil.RandomAddr()

		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2.DHT.Address = testutil.RandomAddr()

		mockObjects = testutil.Mocks(ctrl)
		mockDHTKeeper = mockObjects.DHTKeeper

		_, dhtA = makePeer(ctrl, cfg, nil)
		ann = announcer.New(cfg, dhtA.DHT(), mockObjects.Logic)
	})

	AfterEach(func() {
		ctrl.Finish()
		_ = dhtA.Stop()
		if dhtB != nil {
			_ = dhtB.Stop()
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
			queued := ann.Announce(1, "", []byte("key"), nil)
			Expect(ann.HasTask()).To(BeTrue())
			Expect(queued).To(BeTrue())
		})

		It("should return false if already queued", func() {
			queued := ann.Announce(1, "", []byte("key"), nil)
			Expect(queued).To(BeTrue())
			queued = ann.Announce(1, "", []byte("key"), nil)
			Expect(queued).To(BeFalse())
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
				err := ann.Do(&announcer.Task{Key: key, CheckExistence: true, Done: nil})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(announcer.ErrDelisted))
			})

			It("should return ErrDelisted and remove key from announce list if checker returns false", func() {
				ann.RegisterChecker(1, func(repo string, k []byte) bool {
					Expect(key).To(Equal(k))
					return false
				})
				err := ann.Do(&announcer.Task{Key: key, Type: 1, CheckExistence: true, Done: nil})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(announcer.ErrDelisted))
			})
		})

		When("announcement is successful", func() {
			task := &announcer.Task{Key: key, RepoName: "repo1", Type: 1, Done: func(err error) {}}
			BeforeEach(func() {
				_, dhtB := makePeer(ctrl, cfg2, nil)
				cfg.DHT.BootstrapPeers = dhtB.Addr()
				err = dhtA.Bootstrap()
				Expect(err).To(BeNil())
			})

			It("should re-add key to announce list and return nil error", func() {
				mockDHTKeeper.EXPECT().AddToAnnounceList(key, "repo1", 1, gomock.Any())
				err := ann.Do(task)
				Expect(err).To(BeNil())
			})

			It("should remove task from queued index", func() {
				mockDHTKeeper.EXPECT().AddToAnnounceList(key, "repo1", 1, gomock.Any())
				ann.Announce(task.Type, task.RepoName, task.Key, task.Done)
				Expect(ann.GetQueued()).To(HaveKey(task.GetID()))
				err := ann.Do(task)
				Expect(err).To(BeNil())
				Expect(ann.GetQueued()).ToNot(HaveKey(task.GetID()))
			})
		})

		When("Not connected to a DHT peer", func() {
			It("should return error", func() {
				announcer.MaxRetry = 0
				err := ann.Do(&announcer.Task{Key: key, RepoName: "repo1", Type: 1, Done: func(err error) {}})
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

var _ = Describe("Session", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockAnn *mocks.MockAnnouncer
	var ses *announcer.Session

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		mockAnn = mocks.NewMockAnnouncer(ctrl)
		ses = announcer.NewSession(mockAnn)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Announce", func() {
		It("should call Announce internally", func() {
			mockAnn.EXPECT().Announce(1, "repo1", []byte("abc"), gomock.Any())
			ses.Announce(1, "repo1", []byte("abc"))
		})
	})

	Describe(".Announce", func() {
		It("should call callback with 0 error count when announcement succeeded", func() {
			mockAnn.EXPECT().Announce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(objType int, repo string, key []byte, doneCB func(error)) {
				doneCB(nil)
			}).Return(true)
			ses.Announce(1, "repo1", []byte("abc"))
			ses.OnDone(func(errCount int) {
				Expect(errCount).To(Equal(0))
			})
		})

		It("should call callback with 1 error count when announcement failed", func() {
			mockAnn.EXPECT().Announce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(objType int, repo string, key []byte, doneCB func(error)) {
				doneCB(fmt.Errorf("error"))
			}).Return(true)
			ses.Announce(1, "repo1", []byte("abc"))
			ses.OnDone(func(errCount int) {
				Expect(errCount).To(Equal(1))
			})
		})

		It("should call callback with 0 error count when announcement was not queued", func() {
			mockAnn.EXPECT().Announce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false)
			ses.Announce(1, "repo1", []byte("abc"))
			ses.OnDone(func(errCount int) {
				Expect(errCount).To(Equal(0))
			})
		})
	})
})

func makePeer(ctrl *gomock.Controller, cfg *config.AppConfig, keepers core.Keepers) (*mocks2.MockHost, *server.Server) {
	cfg.DHT.Address = testutil.RandomAddr()
	host, err := net.New(context.Background(), cfg)
	Expect(err).To(BeNil())
	mockHost := mocks2.NewMockHost(ctrl)
	mockHost.EXPECT().Get().Return(host.Get()).AnyTimes()
	mockHost.EXPECT().ID().Return(peer.ID(util.RandString(5))).AnyTimes()
	mockHost.EXPECT().Addrs().Return(host.Get().Addrs()).AnyTimes()
	svr, err := server.New(context.Background(), mockHost, keepers, cfg)
	Expect(err).To(BeNil())
	svr.DHT().Validator.(record.NamespacedValidator)[dht2.ObjectNamespace] = okValidator{}
	return mockHost, svr
}

type okValidator struct{ err error }

func (v okValidator) Validate(key string, value []byte) error         { return nil }
func (v okValidator) Select(key string, values [][]byte) (int, error) { return 0, nil }
