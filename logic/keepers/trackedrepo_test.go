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
	"github.com/make-os/lobe/util/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("Tracklist", func() {
	var appDB storage.Engine
	var state *tree.SafeTree
	var err error
	var cfg *config.AppConfig
	var keeper *TrackedRepoKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
		dbTx := appDB.NewTx(true, true)
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		keeper = NewTrackedRepoKeeper(dbTx, state)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Add", func() {
		It("should add all repository targets if argument is a namespace with no domain", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
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

		It("should add only repository of namespace target if namespace point to a repository target", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Add("ns1/domain2")
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
			err := keeper.Add("ns1/domain2")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("namespace domain (domain2) not found"))
		})

		It("should add repo name", func() {
			err := keeper.Add("repo1")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add 2 repo names", func() {
			err := keeper.Add("repo1, repo2")
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
			rec, err = appDB.Get(MakeTrackedRepoKey("repo2"))
			Expect(err).To(BeNil())
			Expect(rec).ToNot(BeNil())
		})

		It("should add repo name and set initial update height", func() {
			err := keeper.Add("repo1", 100)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			var tr core.TrackedRepo
			rec.Scan(&tr)
			Expect(tr.LastHeight.UInt64()).To(Equal(uint64(100)))
		})

		It("should re-add repo name and reset update height if it already exist", func() {
			err := keeper.Add("repo1", 100)
			Expect(err).To(BeNil())
			err = keeper.Add("repo1", 200)
			Expect(err).To(BeNil())
			rec, err := appDB.Get(MakeTrackedRepoKey("repo1"))
			Expect(err).To(BeNil())
			var tr core.TrackedRepo
			rec.Scan(&tr)
			Expect(tr.LastHeight.UInt64()).To(Equal(uint64(200)))
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

	Describe(".Get", func() {
		It("should return nil if repo was not found", func() {
			Expect(keeper.Get("repo1")).To(BeNil())
		})

		It("should return tracked repo if it exist", func() {
			err := keeper.Add("repo1", 200)
			Expect(err).To(BeNil())
			res := keeper.Get("repo1")
			Expect(res).ToNot(BeNil())
			Expect(res.LastHeight.UInt64()).To(Equal(uint64(200)))
		})
	})

	Describe(".Remove", func() {
		It("should return nil if repo was not found", func() {
			Expect(keeper.Remove("repo1")).To(BeNil())
		})

		It("should remove tracked repo if it exist", func() {
			err := keeper.Add("repo1", 200)
			Expect(err).To(BeNil())
			Expect(keeper.Remove("repo1")).To(BeNil())
			res := keeper.Get("repo1")
			Expect(res).To(BeNil())
		})

		It("should remove repository targets if argument is a namespace", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Add("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.Get("abc")).ToNot(BeNil())
			Expect(keeper.Get("xyz")).ToNot(BeNil())
			Expect(keeper.Remove("ns1/")).To(BeNil())
			Expect(keeper.Get("abc")).To(BeNil())
			Expect(keeper.Get("xyz")).To(BeNil())
		})

		It("should remove namespace target if namespace is whole", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
				"domain2": "r/xyz",
			}})
			err := keeper.Add("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.Get("abc")).ToNot(BeNil())
			Expect(keeper.Get("xyz")).ToNot(BeNil())
			Expect(keeper.Remove("ns1/domain2")).To(BeNil())
			Expect(keeper.Get("abc")).ToNot(BeNil())
			Expect(keeper.Get("xyz")).To(BeNil())
		})

		It("should return error if namespace domain does not exist", func() {
			nsKeeper := NewNamespaceKeeper(state)
			nsKeeper.Update(crypto.MakeNamespaceHash("ns1"), &state2.Namespace{Domains: map[string]string{
				"domain1": "r/abc",
			}})
			err := keeper.Add("ns1/")
			Expect(err).To(BeNil())
			Expect(keeper.Remove("ns1/domain2")).To(MatchError("namespace domain (domain2) not found"))
		})
	})

	Describe(".Tracked", func() {
		It("should return map of tracked repo", func() {
			keeper.Add("repo1")
			err = keeper.Add("repo2", 1200)
			Expect(err).To(BeNil())
			res := keeper.Tracked()
			Expect(res).To(HaveKey("repo1"))
			Expect(res).To(HaveKey("repo2"))
			Expect(res["repo2"].LastHeight.UInt64()).To(Equal(uint64(1200)))
		})
	})
})
