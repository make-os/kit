package testutil

import (
	"os"

	drandmocks "github.com/makeos/mosdef/crypto/rand/mocks"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util/logger"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"

	"github.com/makeos/mosdef/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"

	path "path/filepath"

	"github.com/makeos/mosdef/config"
	"github.com/mitchellh/go-homedir"
)

// SetTestCfg prepare a config directory for tests
func SetTestCfg() (*config.EngineConfig, error) {
	var err error
	dir, _ := homedir.Dir()
	dataDir := path.Join(dir, util.RandString(5))
	os.MkdirAll(dataDir, 0700)

	// Create test root command and
	// set required flags and values
	rootCmd := &cobra.Command{}
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set the network version")
	rootCmd.PersistentFlags().String("home", "", "Set configuration directory")
	rootCmd.PersistentFlags().Set("home", dataDir)
	rootCmd.PersistentFlags().Set("net", dataDir)
	viper.Set("net.version", 10000000)

	var cfg = &config.EngineConfig{}
	var tmcfg = tmconfig.DefaultConfig()

	commands.SetLoggerToNoop()

	// Initialize the config using the test root command
	config.Configure(rootCmd, cfg, tmcfg)
	cfg.Node.Mode = config.ModeTest

	// Initialize the directory
	commands.SetConfig(tmcfg)
	commands.InitFilesCmd.RunE(nil, nil)
	tmconfig.EnsureRoot(tmcfg.RootDir)

	// Replace logger with Noop logger
	cfg.G().Log = logger.NewLogrusNoOp()

	return cfg, err
}

// GetDB test databases
func GetDB(cfg *config.EngineConfig) (appDB *storage.Badger, stateTreeDB *storage.Badger) {
	appDB = storage.NewBadger()
	if err := appDB.Init(cfg.GetAppDBDir()); err != nil {
		panic(err)
	}
	stateTreeDB = storage.NewBadger()
	if err := stateTreeDB.Init(cfg.GetStateTreeDBDir()); err != nil {
		panic(err)
	}
	return appDB, stateTreeDB
}

// MockObjects contains mocks for various structs
type MockObjects struct {
	Logic           *mocks.MockLogic
	AtomicLogic     *mocks.MockAtomicLogic
	Sys             *mocks.MockSysLogic
	Tx              *mocks.MockTxLogic
	Validator       *mocks.MockValidatorLogic
	SysKeeper       *mocks.MockSystemKeeper
	AccountKeeper   *mocks.MockAccountKeeper
	ValidatorKeeper *mocks.MockValidatorKeeper
	RepoKeeper      *mocks.MockRepoKeeper
	TxKeeper        *mocks.MockTxKeeper
	TicketManager   *mocks.MockTicketManager
	StateTree       *mocks.MockTree
	Drand           *drandmocks.MockDRander
	RepoManager     *mocks.MockRepoManager
}

// MockLogic returns logic package mocks
func MockLogic(ctrl *gomock.Controller) *MockObjects {
	mo := &MockObjects{}
	mo.Logic = mocks.NewMockLogic(ctrl)
	mo.AtomicLogic = mocks.NewMockAtomicLogic(ctrl)

	mo.Sys = mocks.NewMockSysLogic(ctrl)
	mo.Tx = mocks.NewMockTxLogic(ctrl)
	mo.Validator = mocks.NewMockValidatorLogic(ctrl)
	mo.SysKeeper = mocks.NewMockSystemKeeper(ctrl)
	mo.AccountKeeper = mocks.NewMockAccountKeeper(ctrl)
	mo.RepoKeeper = mocks.NewMockRepoKeeper(ctrl)
	mo.ValidatorKeeper = mocks.NewMockValidatorKeeper(ctrl)
	mo.TxKeeper = mocks.NewMockTxKeeper(ctrl)
	mo.Logic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.TicketManager = mocks.NewMockTicketManager(ctrl)
	mo.StateTree = mocks.NewMockTree(ctrl)
	mo.Drand = drandmocks.NewMockDRander(ctrl)
	mo.RepoManager = mocks.NewMockRepoManager(ctrl)

	mo.Logic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.Logic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.Logic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.Logic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.Logic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.Logic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.Logic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.Logic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.Logic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.Logic.EXPECT().GetDRand().Return(mo.Drand).MinTimes(0)
	mo.Logic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.AtomicLogic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.AtomicLogic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.AtomicLogic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetDRand().Return(mo.Drand).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)

	return mo
}
