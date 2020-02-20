package testutil

import (
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/storage"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"
	"gitlab.com/makeos/mosdef/util"

	path "path/filepath"

	"github.com/mitchellh/go-homedir"
	"gitlab.com/makeos/mosdef/config"
)

// GPGProgramPath is the path to the gpg program
const GPGProgramPath = "gpg"

// SetTestCfg prepare a config directory for tests
func SetTestCfg(opts ...string) (*config.AppConfig, error) {
	var dataDirName = util.RandString(5)
	if len(opts) > 0 {
		dataDirName = opts[0]
	}

	var err error
	dir, _ := homedir.Dir()
	dataDir := path.Join(dir, dataDirName)
	os.MkdirAll(dataDir, 0700)

	// Create test root command and
	// set required flags and values
	rootCmd := &cobra.Command{}
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set the network version")
	rootCmd.PersistentFlags().String("home", "", "Set configuration directory")
	rootCmd.PersistentFlags().Set("home", dataDir)
	rootCmd.PersistentFlags().Set("net", dataDir)
	viper.Set("net.version", 10000000)

	var cfg = &config.AppConfig{}
	var tmcfg = tmconfig.DefaultConfig()

	commands.SetLoggerToNoop()

	// Initialize the config using the test root command
	interrupt := util.Interrupt(make(chan struct{}))
	config.Configure(rootCmd, cfg, tmcfg, &interrupt)
	cfg.Node.Mode = config.ModeTest
	os.MkdirAll(path.Join(cfg.NetDataDir(), "repos"), 0700)
	cfg.SetRepoRoot(path.Join(cfg.NetDataDir(), "repos"))

	// Initialize the directory
	commands.SetConfig(tmcfg)
	commands.InitFilesCmd.RunE(nil, nil)
	tmconfig.EnsureRoot(tmcfg.RootDir)
	cfg.PrepareNodeValKeys(tmcfg.NodeKeyFile(), tmcfg.PrivValidatorKeyFile(),
		tmcfg.PrivValidatorStateFile())

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
func GetDBAtDir(cfg *config.AppConfig, dir string) *storage.Badger {
	db := storage.NewBadger()
	if err := db.Init(dir); err != nil {
		panic(err)
	}
	return db
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
	RepoManager     *mocks.MockRepoManager
	GPGPubKeyKeeper *mocks.MockGPGPubKeyKeeper
	NamespaceKeeper *mocks.MockNamespaceKeeper
	BlockGetter     *mocks.MockBlockGetter
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
	mo.TicketManager = mocks.NewMockTicketManager(ctrl)
	mo.StateTree = mocks.NewMockTree(ctrl)
	mo.RepoManager = mocks.NewMockRepoManager(ctrl)
	mo.GPGPubKeyKeeper = mocks.NewMockGPGPubKeyKeeper(ctrl)
	mo.NamespaceKeeper = mocks.NewMockNamespaceKeeper(ctrl)
	mo.BlockGetter = mocks.NewMockBlockGetter(ctrl)

	mo.Logic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.Logic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.Logic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.Logic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.Logic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.Logic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.Logic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.Logic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.Logic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.Logic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.Logic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)
	mo.Logic.EXPECT().GPGPubKeyKeeper().Return(mo.GPGPubKeyKeeper).MinTimes(0)
	mo.Logic.EXPECT().NamespaceKeeper().Return(mo.NamespaceKeeper).MinTimes(0)

	mo.AtomicLogic.EXPECT().Sys().Return(mo.Sys).MinTimes(0)
	mo.AtomicLogic.EXPECT().Tx().Return(mo.Tx).MinTimes(0)
	mo.AtomicLogic.EXPECT().Validator().Return(mo.Validator).MinTimes(0)
	mo.AtomicLogic.EXPECT().SysKeeper().Return(mo.SysKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().AccountKeeper().Return(mo.AccountKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().RepoKeeper().Return(mo.RepoKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().ValidatorKeeper().Return(mo.ValidatorKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().TxKeeper().Return(mo.TxKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetTicketManager().Return(mo.TicketManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().StateTree().Return(mo.StateTree).MinTimes(0)
	mo.AtomicLogic.EXPECT().GetRepoManager().Return(mo.RepoManager).MinTimes(0)
	mo.AtomicLogic.EXPECT().GPGPubKeyKeeper().Return(mo.GPGPubKeyKeeper).MinTimes(0)
	mo.AtomicLogic.EXPECT().NamespaceKeeper().Return(mo.NamespaceKeeper).MinTimes(0)

	return mo
}

// CreateGPGKey creates a GPG RSA key and returns the key id
func CreateGPGKey(gpgProgram, tempDir string) string {
	randStr := util.RandString(5)
	f, err := ioutil.TempFile(tempDir, "testkey")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(`%no-protection
Key-Type: RSA
Key-Length: 2048
Subkey-Type: 1
Subkey-Length: 2048
Name-Real: Root ` + randStr + `
Name-Email: ` + randStr + `@example.com
Expire-Date: 0`)
	args := []string{"--batch", "--gen-key", f.Name()}
	cmd := exec.Command(gpgProgram, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GNUPGHOME="+tempDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	reg := regexp.MustCompile("\\s([A-Z0-9]{16})\\s")
	match := reg.FindStringSubmatch(string(out))
	return match[1]
}
