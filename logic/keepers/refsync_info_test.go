package keepers

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	state2 "github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("Tracklist", func() {
	var appDB storagetypes.Engine
	var state *tree.SafeTree
	var err error
	var cfg *config.AppConfig
	var keeper *RepoSyncInfoKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		dbTx := appDB.NewTx(true, true)
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		keeper = NewRepoSyncInfoKeeper(dbTx, state)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Track", func() {
		It("should add all repository targets if argument is a namespace with no domain", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Track("ns1/")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("abc"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
			rec, err = appDB.Get(MakeTrackedRepoKey("xyz"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add only repository of namespace target if namespace point to a repository target", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Track("ns1/domain2")
			Expect(err).To(BeNil())
			_, err = appDB.Get(MakeTrackedRepoKey("abc"))
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(storage.ErrRecordNotFound))
			rec, err := appDB.Get(MakeTrackedRepoKey("xyz"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should return error if namespace domain does not exist", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
			}})
			err := keeper.Track("ns1/domain2")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("namespace domain (domain2) not found"))
		})

		It("should add repo name", func() {
			err := keeper.Track("repo1")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add 2 repo names", func() {
			err := keeper.Track("repo1, repo2")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
			rec, err = appDB.Get(MakeTrackedRepoKey("repo2"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add repo name and set initial update height", func() {
			err := keeper.Track("repo1", 100)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			var tr core.TrackedRepo
			rec.Scan(&tr)
			Expect(tr.UpdatedAt.UInt64()).To(Equal(uint64(100)))
		})

		It("should re-add repo name and reset update height if it already exist", func() {
			err := keeper.Track("repo1", 100)
			Expect(err).To(BeNil())
			err = keeper.Track("repo1", 200)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			var tr core.TrackedRepo
			rec.Scan(&tr)
			Expect(tr.UpdatedAt.UInt64()).To(Equal(uint64(200)))
		})

		It("should return error when repo name is invalid", func() {
			err := keeper.Track("rep&%o1")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target (rep&%o1) is not a valid repo identifier"))
		})

		It("should return error when namespace does not exist", func() {
			err := keeper.Track("ns1/")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("namespace (ns1) not found"))
		})
	})

	Describe(".GetTracked", func() {
		It("should return nil if repo was not found", func() {
			Expect(keeper.GetTracked("repo1")).To(BeNil())
		})

		It("should return tracked repo if it exist", func() {
			err := keeper.Track("repo1", 200)
			Expect(err).To(BeNil())
			res := keeper.GetTracked("repo1")
			Expect(res).ToNot(BeNil())
			Expect(res.UpdatedAt.UInt64()).To(Equal(uint64(200)))
		})
	})

	Describe(".UnTrack", func() {
		It("should return nil if repo was not found", func() {
			Expect(keeper.UnTrack("repo1")).To(BeNil())
		})

		It("should remove tracked repo if it exist", func() {
			err := keeper.Track("repo1", 200)
			Expect(err).To(BeNil())
			Expect(keeper.UnTrack("repo1")).To(BeNil())
			res := keeper.GetTracked("repo1")
			Expect(res).To(BeNil())
		})

		It("should remove repository targets if argument is a namespace", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Track("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.GetTracked("abc")).ToNot(BeNil())
			Expect(keeper.GetTracked("xyz")).ToNot(BeNil())
			Expect(keeper.UnTrack("ns1/")).To(BeNil())
			Expect(keeper.GetTracked("abc")).To(BeNil())
			Expect(keeper.GetTracked("xyz")).To(BeNil())
		})

		It("should remove namespace target if namespace is whole", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Track("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.GetTracked("abc")).ToNot(BeNil())
			Expect(keeper.GetTracked("xyz")).ToNot(BeNil())
			Expect(keeper.UnTrack("ns1/domain2")).To(BeNil())
			Expect(keeper.GetTracked("abc")).ToNot(BeNil())
			Expect(keeper.GetTracked("xyz")).To(BeNil())
		})

		It("should return error if namespace domain does not exist", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
			}})
			err := keeper.Track("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.UnTrack("ns1/domain2")).To(MatchError("namespace domain (domain2) not found"))
		})
	})

	Describe(".Tracked", func() {
		It("should return map of tracked repo", func() {
			keeper.Track("repo1")
			err = keeper.Track("repo2", 1200)
			Expect(err).To(BeNil())
			res := keeper.Tracked()
			Expect(res).To(HaveKey("repo1"))
			Expect(res).To(HaveKey("repo2"))
			Expect(res["repo2"].UpdatedAt.UInt64()).To(Equal(uint64(1200)))
		})
	})

	Describe(".UpdateRefLastSyncHeight", func() {
		It("should update height", func() {
			err := keeper.UpdateRefLastSyncHeight("repo1", "ref/heads/master", 10)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeRepoRefLastSyncHeightKey("repo1", "ref/heads/master"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
			height := util.DecodeNumber(rec.Value)
			Expect(height).To(Equal(uint64(10)))
		})
	})

	Describe(".GetRefLastSyncHeight", func() {
		It("should update height", func() {
			err := keeper.UpdateRefLastSyncHeight("repo1", "ref/heads/master", 10)
			Expect(err).To(BeNil())
			height, err := keeper.GetRefLastSyncHeight("repo1", "ref/heads/master")
			Expect(err).To(BeNil())
			Expect(height).To(Equal(uint64(10)))
		})
	})
})
