package testutil

import (
	"fmt"
	"io/ioutil"
	"os"
	path "path/filepath"

	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/lobe/mocks"
	"gitlab.com/makeos/lobe/pkgs/logger"
	"gitlab.com/makeos/lobe/storage"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"

	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"
	"gitlab.com/makeos/lobe/util"

	"gitlab.com/makeos/lobe/config"
)

// SetTestCfg prepare a config directory for tests
func SetTestCfg(opts ...string) (cfg *config.AppConfig, err error) {
	var dataDirName = "_test_" + util.RandString(5)
	if len(opts) > 0 {
		dataDirName = opts[0]
	}

	// Create test directory
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(fmt.Errorf("failed to create test directory"))
	}
	dataDir := path.Join(dir, dataDirName)
	os.MkdirAll(dataDir, 0700)

	// Set required viper keys
	viper.Set("net.version", 10000000)
	viper.Set("home", dataDir)

	commands.SetLoggerToNoop()

	// Initialize the config using the test root command
	var tmcfg = tmconfig.DefaultConfig()
	cfg = &config.AppConfig{}
	interrupt := util.Interrupt(make(chan struct{}))
	config.Configure(cfg, tmcfg, &interrupt)
	cfg.Node.Mode = config.ModeTest

	// Initialize the directory
	commands.SetConfig(tmcfg)
	tmconfig.EnsureRoot(tmcfg.RootDir)
	commands.InitFilesCmd.RunE(nil, nil)
	cfg.LoadKeys(tmcfg.NodeKeyFile(), tmcfg.PrivValidatorKeyFile(), tmcfg.PrivValidatorStateFile())

	// Replace logger with Noop logger
	cfg.G().Log = logger.NewLogrusNoOp()

	return cfg, err
}

// GetDB test databases
func GetDB(cfg *config.AppConfig) (appDB *storage.Badger, stateTreeDB *storage.Badger) {
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

// GetDBAtDir test databases at a directory

// MockObjects contains mocks for various structs
type MockObjects struct {
	Logic           *mocks.MockLogic
	AtomicLogic     *mocks.MockAtomicLogic
	Validator       *mocks.MockValidatorLogic
	SysKeeper       *mocks.MockSystemKeeper
	AccountKeeper   *mocks.MockAccountKeeper
	ValidatorKeeper *mocks.MockValidatorKeeper
	RepoKeeper      *mocks.MockRepoKeeper
	TxKeeper        *mocks.MockTxKeeper
	TicketManager   *mocks.MockTicketManager
	StateTree       *mocks.MockTree
	RemoteServer    *mocks.MockRemoteServer
	PushKeyKeeper   *mocks.MockPushKeyKeeper
	NamespaceKeeper *mocks.MockNamespaceKeeper
	BlockGetter     *mocks.MockBlockGetter
}

// MockLogic returns logic package mocks
func MockLogic(ctrl *gomock.Controller) *MockObjects {
	mo := &MockObjects{}
	mo.Logic = mocks.NewMockLogic(ctrl)
	mo.AtomicLogic = mocks.NewMockAtomicLogic(ctrl)

	mo.Validator = mocks.NewMockValidatorLogic(ctrl)
	mo.SysKeeper = mocks.NewMockSystemKeeper(ctrl)
	mo.AccountKeeper = mocks.NewMockAccountKeeper(ctrl)
	mo.RepoKeeper = mocks.NewMockRepoKeeper(ctrl)
	mo.ValidatorKeeper = mocks.NewMockValidatorKeeper(ctrl)
	mo.TxKeeper = mocks.NewMockTxKeeper(ctrl)
	mo.TicketManager = mocks.NewMockTicketManager(ctrl)
	mo.StateTree = mocks.NewMockTree(ctrl)
	mo.RemoteServer = mocks.NewMockRemoteServer(ctrl)
	mo.PushKeyKeeper = mocks.NewMockPushKeyKeeper(ctrl)
	mo.NamespaceKeeper = mocks.NewMockNamespaceKeeper(ctrl)
	mo.BlockGetter = mocks.NewMockBlockGetter(ctrl)

	mo.Logic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.Logic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.Logic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.Logic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.Logic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.Logic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.Logic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.Logic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.Logic.EXPECT().GetRemoteServer().Return(mo.RemoteServer).MinTimes(0)
	mo.Logic.EXPECT().PushKeyKeeper().Return(mo.PushKeyKeeper).MinTimes(0)
	mo.Logic.EXPECT().NamespaceKeeper().Return(mo.NamespaceKeeper).MinTimes(0)
	mo.Logic.EXPECT().NamespaceKeeper().Return(mo.NamespaceKeeper).MinTimes(0)

	mo.AtomicLogic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.AtomicLogic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetRemoteServer().Return(mo.RemoteServer).MinTimes(0)
	mo.AtomicLogic.EXPECT().PushKeyKeeper().Return(mo.PushKeyKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().NamespaceKeeper().Return(mo.NamespaceKeeper).MinTimes(0)

	return mo
}

// ReturnStringOnCallCount returns the element in ret that correspond with the callCount value.
// Use this in repeatedly called callback functions when you want to determine what value to
// return at a given call count.
func ReturnStringOnCallCount(callCount *int, ret ...string) string {
	str := ret[*callCount]
	*callCount++
	return str
}
