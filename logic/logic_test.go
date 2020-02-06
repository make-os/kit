package logic_test

import (
	"os"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	l "github.com/makeos/mosdef/logic"
)

var _ = Describe("Logic", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *l.Logic

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = l.New(appDB, stateTreeDB, cfg)
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".WriteGenesisState", func() {
		var testGenData = []*config.GenDataEntry{
			&config.GenDataEntry{Type: "account", Address: "addr1", Balance: "100"},
			&config.GenDataEntry{Type: "account", Address: "addr2", Balance: "200"},
			&config.GenDataEntry{
				Type: "repo",
				Name: "my-repo",
				Owners: map[string]*config.RepoOwner{
					"addr": &config.RepoOwner{Creator: true, JoinedAt: 1, Veto: true},
				},
			},
		}

		BeforeEach(func() {
			cfg.GenesisFileEntries = testGenData
			for _, a := range testGenData {
				if a.Type == "account" {
					res := logic.AccountKeeper().GetAccount(util.String(a.Address))
					Expect(res.Balance).To(Equal(util.String("0")))
					Expect(res.Nonce).To(Equal(uint64(0)))
				}
			}
			err = logic.WriteGenesisState()
			Expect(err).To(BeNil())
		})

		It("should successfully add all accounts with expected balance", func() {
			addr1Res := logic.AccountKeeper().GetAccount(util.String(testGenData[0].Address))
			Expect(addr1Res.Balance).To(Equal(util.String("100")))
			addr2Res := logic.AccountKeeper().GetAccount(util.String(testGenData[1].Address))
			Expect(addr2Res.Balance).To(Equal(util.String("200")))
		})

		It("should successfully add all repos", func() {
			repo := logic.RepoKeeper().GetRepo("my-repo")
			Expect(repo.IsNil()).To(BeFalse())
			helmRepo, err := logic.SysKeeper().GetHelmRepo()
			Expect(err).To(BeNil())
			Expect(helmRepo).NotTo(Equal("my-repo"))
		})
	})
})
