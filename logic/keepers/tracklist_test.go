package keepers

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	state2 "github.com/make-os/lobe/types/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("Tracklist", func() {
	var appDB storage.Engine
	var state *tree.SafeTree
	var err error
	var cfg *config.AppConfig
	var keeper *TrackListKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
		dbTx := appDB.NewTx(true, true)
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		keeper = NewTrackListKeeper(dbTx, state)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Add", func() {
		It("should add repository targets if argument is a namespace", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update("ns1", &state2.Namespace{Domains: map[string]string{
				"stuff":  "r/abc",
				"stuff2": "r/xyz",
			}})
			err := keeper.Add("ns1/")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("abc"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
			rec, err = appDB.Get(MakeTrackedRepoKey("xyz"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add repo name", func() {
			err := keeper.Add("repo1")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should return error when repo name is invalid", func() {
			err := keeper.Add("rep&%o1")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("target (rep&%o1) is not a valid repo identifier"))
		})

		It("should return error when namespace does not exist", func() {
			err := keeper.Add("ns1/")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("namespace (ns1) not found"))
		})
	})

	Describe(".UpdateLastHeight", func() {
		It("should update last height of tracked repo", func() {
			err := keeper.Add("repo1")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			var ti core.TrackedRepo
			rec.Scan(&ti)
			Expect(ti.LastHeight).To(Equal(uint64(0)))

			err = keeper.UpdateLastHeight("repo1", 1200)
			Expect(err).To(BeNil())

			rec, err = appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			rec.Scan(&ti)
			Expect(ti.LastHeight).To(Equal(uint64(1200)))
		})

		It("should return error if repo is not tracked", func() {
			err = keeper.UpdateLastHeight("repo1", 1200)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("repo not tracked"))
		})
	})

	Describe(".Tracked", func() {
		It("should return map of tracked repo", func() {
			keeper.Add("repo1")
			keeper.Add("repo2")
			err = keeper.UpdateLastHeight("repo2", 1200)
			Expect(err).To(BeNil())
			res := keeper.Tracked()
			Expect(res).To(HaveKey("repo1"))
			Expect(res).To(HaveKey("repo2"))
			Expect(res["repo2"].LastHeight).To(Equal(uint64(1200)))
		})
	})
})
