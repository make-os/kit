package logic_test

import (
	"encoding/json"
	"os"
	"testing"

	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	l "github.com/make-os/lobe/logic"
)

func TestLogic(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logic Suite")
}

var _ = Describe("Logic", func() {
	var appDB, stateTreeDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var logic *l.Logic

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = l.New(appDB, stateTreeDB, cfg)
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ApplyGenesisState", func() {
		var testGenData = []*config.GenDataEntry{
			{Type: config.GenDataTypeAccount, Address: "addr1", Balance: "100"},
			{Type: config.GenDataTypeAccount, Address: "addr2", Balance: "200"},
			{
				Type: "repo",
				Name: "my-repo",
				Owners: map[string]*config.RepoOwner{
					"addr": {Creator: true, JoinedAt: 1, Veto: true},
				},
			},
		}

		When("genesis state is provided via config", func() {
			BeforeEach(func() {
				cfg.GenesisFileEntries = testGenData
				for _, a := range testGenData {
					if a.Type == config.GenDataTypeAccount {
						res := logic.AccountKeeper().Get(identifier.Address(a.Address))
						Expect(res.Balance).To(Equal(util.String("0")))
						Expect(res.Nonce.UInt64()).To(Equal(uint64(0)))
					}
				}
				err = logic.ApplyGenesisState(nil)
				Expect(err).To(BeNil())
			})

			It("should successfully add all accounts with expected balance", func() {
				addr1Res := logic.AccountKeeper().Get(identifier.Address(testGenData[0].Address))
				Expect(addr1Res.Balance).To(Equal(util.String("100")))
				addr2Res := logic.AccountKeeper().Get(identifier.Address(testGenData[1].Address))
				Expect(addr2Res.Balance).To(Equal(util.String("200")))
			})

			It("should successfully add all repos", func() {
				repo := logic.RepoKeeper().Get("my-repo")
				Expect(repo.IsNil()).To(BeFalse())
				helmRepo, err := logic.SysKeeper().GetHelmRepo()
				Expect(err).To(BeNil())
				Expect(helmRepo).NotTo(Equal("my-repo"))
			})
		})

		When("genesis state is provided as argument", func() {
			BeforeEach(func() {
				for _, a := range testGenData {
					if a.Type == config.GenDataTypeAccount {
						res := logic.AccountKeeper().Get(identifier.Address(a.Address))
						Expect(res.Balance).To(Equal(util.String("0")))
						Expect(res.Nonce.UInt64()).To(Equal(uint64(0)))
					}
				}
				rawState, _ := json.Marshal(testGenData)
				err = logic.ApplyGenesisState(rawState)
				Expect(err).To(BeNil())
			})

			It("should successfully add all accounts with expected balance", func() {
				addr1Res := logic.AccountKeeper().Get(identifier.Address(testGenData[0].Address))
				Expect(addr1Res.Balance).To(Equal(util.String("100")))
				addr2Res := logic.AccountKeeper().Get(identifier.Address(testGenData[1].Address))
				Expect(addr2Res.Balance).To(Equal(util.String("200")))
			})

			It("should successfully add all repos", func() {
				repo := logic.RepoKeeper().Get("my-repo")
				Expect(repo.IsNil()).To(BeFalse())
				helmRepo, err := logic.SysKeeper().GetHelmRepo()
				Expect(err).To(BeNil())
				Expect(helmRepo).NotTo(Equal("my-repo"))
			})
		})
	})
})
