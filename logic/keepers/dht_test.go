package keepers

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/storage/mocks"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DHT", func() {
	var appDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var keeper *DHTKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		dbTx := appDB.NewTx(true, true)
		keeper = NewDHTKeyKeeper(dbTx)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".AddToAnnounceList", func() {
		It("should add an entry", func() {
			err := keeper.AddToAnnounceList([]byte("key1"), "repo1", 1, 100000)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeAnnounceListKey([]byte("key1"), 100000))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should remove existing entry before adding an entry with matching key", func() {
			err := keeper.AddToAnnounceList([]byte("key1"), "repo1", 1, 100000)
			Expect(err).To(BeNil())
			err = keeper.AddToAnnounceList([]byte("key1"), "repo1", 1, 200000)
			Expect(err).To(BeNil())

			rec, err := appDB.Get(MakeAnnounceListKey([]byte("key1"), 200000))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())

			rec, err = appDB.Get(MakeAnnounceListKey([]byte("key1"), 100000))
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(storage.ErrRecordNotFound))
		})

		It("should return error on failure", func() {
			mockDBTx := mocks.NewMockTx(ctrl)
			mockDBTx.EXPECT().NewTx(true, true).Return(mockDBTx)
			keeper.db = mockDBTx
			mockDBTx.EXPECT().Put(gomock.Any()).Return(fmt.Errorf("error"))
			mockDBTx.EXPECT().Iterate(gomock.Any(), gomock.Any(), gomock.Any())
			err := keeper.AddToAnnounceList([]byte("key1"), "", 1, 100000)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})

	Describe(".RemoveFromAnnounceList", func() {
		It("should remove an entry from the announcement list", func() {
			err := keeper.AddToAnnounceList([]byte("key1"), "repo1", 1, 100000)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeAnnounceListKey([]byte("key1"), 100000))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())

			keeper.RemoveFromAnnounceList([]byte("key1"))
			_, err = appDB.Get(MakeAnnounceListKey([]byte("key1"), 100000))
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(storage.ErrRecordNotFound))
		})
	})

	Describe(".IterateAnnounceList", func() {
		It("should walk through the list", func() {
			err := keeper.AddToAnnounceList([]byte("key1"), "repo1", 1, 100000)
			Expect(err).To(BeNil())
			err = keeper.AddToAnnounceList([]byte("key2"), "repo2", 2, 200000)
			Expect(err).To(BeNil())

			var res = map[string]*core.AnnounceListEntry{}
			keeper.IterateAnnounceList(func(key []byte, entry *core.AnnounceListEntry) {
				res[string(key)] = entry
			})
			Expect(res).To(HaveLen(2))
			Expect(res["key1"]).ToNot(BeNil())
			Expect(res["key2"]).ToNot(BeNil())
			Expect(res["key1"].NextTime).To(Equal(int64(100000)))
			Expect(res["key1"].Repo).To(Equal("repo1"))
			Expect(res["key1"].Type).To(Equal(1))
			Expect(res["key2"].NextTime).To(Equal(int64(200000)))
			Expect(res["key2"].Repo).To(Equal("repo2"))
			Expect(res["key2"].Type).To(Equal(2))
		})
	})
})
