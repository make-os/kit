package cmd

import (
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gen2brain/beeep"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/gitcmd"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/colorfmt"
	"github.com/thoas/go-funk"

	"github.com/make-os/lobe/config"
	tmcfg "github.com/tendermint/tendermint/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// BuildVersion is the build version set by goreleaser
	BuildVersion = ""

	// BuildCommit is the git hash of the build. It is set by goreleaser
	BuildCommit = ""

	// BuildDate is the date the build was created. Its is set by goreleaser
	BuildDate = ""

	// GoVersion is the version of go used to build the client
	GoVersion = "go1.13"
)

var (
	log logger.Logger

	// cfg is the application config
	cfg = config.EmptyAppConfig()

	// Get a reference to tendermint's config object
	tmconfig = tmcfg.DefaultConfig()

	// itr is used to inform the stoppage of all modules
	itr = util.Interrupt(make(chan struct{}))
)

// Execute the root command or fallback command when command is unknown.
func Execute() {

	// When command is unknown, run the root command PersistentPreRun
	// then run the fallback command
	_, _, err := rootCmd.Find(os.Args[1:])
	if err != nil && strings.Index(err.Error(), "unknown command") != -1 {
		rootCmd.PersistentPreRun(fallbackCmd, os.Args)
		fallbackCmd.Run(&cobra.Command{}, os.Args)
		return
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "lob",
	Short: "Lobe is the official client for the MakeOS network",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		curCmd := cmd.CalledAs()

		// Configure the node's home directory
		config.Configure(cfg, tmconfig, &itr)
		log = cfg.G().Log

		// Set version information
		cfg.VersionInfo = &config.VersionInfo{}
		cfg.VersionInfo.BuildCommit = BuildCommit
		cfg.VersionInfo.BuildDate = BuildDate
		cfg.VersionInfo.GoVersion = GoVersion
		cfg.VersionInfo.BuildVersion = BuildVersion

		// Load keys in the config object
		if curCmd != "init" {
			cfg.LoadKeys(tmconfig.NodeKeyFile(), tmconfig.PrivValidatorKeyFile(), tmconfig.PrivValidatorStateFile())
		}

		// Skip git exec check for certain commands
		if !funk.ContainsString([]string{
			"init",
			"start",
			"console",
			"sign",
			"attach",
			"config"}, curCmd) {
			return
		}

		if yes, version := util.IsGitInstalled(cfg.Node.GitBinPath); yes {
			if semver.New(version).LessThan(*semver.New("2.11.0")) {
				log.Fatal(colorfmt.YellowString(`Git version is outdated. Please update git executable.` +
					`Visit https://git-scm.com/downloads to download and install the latest version.`,
				))
			}
		} else {
			log.Fatal(colorfmt.YellowString(`Git executable was not found.` +
				`If you already have Git installed, provide the executable's location using --gitpath, otherwise ` +
				`visit https://git-scm.com/downloads to download and install it.`,
			))
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		version, _ := cmd.Flags().GetBool("version")
		if version {
			fmt.Println("Client:", BuildVersion)
			fmt.Println("Build:", BuildCommit)
			fmt.Println("Go:", GoVersion)
			fmt.Println("NodeID:", cfg.G().NodeKey.ID())
			return
		}

		cmd.Help()
	},
}

// isGitSignRequest checks whether the program arguments
// indicate a request from git to sign a message
func isGitSignRequest(args []string) bool {
	return len(args) == 4 && args[1] == "--status-fd=2" && args[2] == "-bsau"
}

// isGitVerifyRequest checks whether the program arguments
// indicate a request from git to verify a signature
func isGitVerifyRequest(args []string) bool {
	return len(args) == 6 && funk.ContainsString(args, "--verify")
}

// fallbackCmd is called any time an unknown command is executed
var fallbackCmd = &cobra.Command{
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {

		if isGitSignRequest(args) {
			if err := gitcmd.GitSignCmd(cfg, os.Stdin, &gitcmd.GitSignArgs{
				Args:            os.Args,
				RepoGetter:      repo.Get,
				PushKeyUnlocker: common.UnlockKey,
				StdErr:          os.Stderr,
				StdOut:          os.Stdout,
			}); err != nil {
				if cfg.IsDev() {
					beeep.Alert("ERROR", err.Error(), "")
				}
				log.Fatal(err.Error())
			}
			os.Exit(0)
		}

		if isGitVerifyRequest(args) {
			if err := gitcmd.GitVerifyCmd(cfg, &gitcmd.GitVerifyArgs{
				Args:            args,
				RepoGetter:      repo.Get,
				PushKeyUnlocker: common.UnlockKey,
				PemDecoder:      pem.Decode,
				StdOut:          os.Stdout,
				StdErr:          os.Stderr,
				StdIn:           os.Stdin,
			}); err != nil {
				log.Fatal(err.Error())
			}
			os.Exit(0)
		}

		fmt.Print("Unknown command. Use --help to see commands.\n")
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(fallbackCmd)
	rootCmd.Flags().SortFlags = false

	// Register flags
	rootCmd.PersistentFlags().String("home", config.DefaultDataDir, "Set the path to the home directory")
	rootCmd.PersistentFlags().String("home.prefix", "", "Adds a prefix to the home directory in dev mode")
	rootCmd.PersistentFlags().String("gitpath", "git", "Set path to git executable")
	rootCmd.PersistentFlags().Bool("dev", false, "Enables development mode")
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set network/chain ID")
	rootCmd.PersistentFlags().Bool("no-log", false, "Disables loggers")
	rootCmd.PersistentFlags().Bool("no-colors", false, "Disables output colors")
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
	rootCmd.PersistentFlags().StringToString("loglevel", map[string]string{}, "Set log level for modules")

	// Remote API connection flags
	rootCmd.PersistentFlags().String("rpc.user", "", "Set the RPC username")
	rootCmd.PersistentFlags().String("rpc.password", "", "Set the RPC password")
	rootCmd.PersistentFlags().String("remote.address", config.DefaultRemoteServerAddress, "Set the RPC listening address")
	rootCmd.PersistentFlags().Bool("rpc.https", false, "Force the client to use https:// protocol")

	// Hidden flags relevant to git gpg interface conformance
	rootCmd.PersistentFlags().String("keyid-format", "", "")
	rootCmd.PersistentFlags().MarkHidden("keyid-format")
	rootCmd.PersistentFlags().String("status-fd", "", "")
	rootCmd.PersistentFlags().MarkHidden("status-fd")
	rootCmd.PersistentFlags().Bool("verify", false, "")
	rootCmd.PersistentFlags().MarkHidden("verify")

	// Viper bindings
	viper.BindPFlag("node.gitpath", rootCmd.PersistentFlags().Lookup("gitpath"))
	viper.BindPFlag("net.version", rootCmd.PersistentFlags().Lookup("net"))
	viper.BindPFlag("dev", rootCmd.PersistentFlags().Lookup("dev"))
	viper.BindPFlag("home", rootCmd.PersistentFlags().Lookup("home"))
	viper.BindPFlag("home.prefix", rootCmd.PersistentFlags().Lookup("home.prefix"))
	viper.BindPFlag("no-log", rootCmd.PersistentFlags().Lookup("no-log"))
	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))
	viper.BindPFlag("no-colors", rootCmd.PersistentFlags().Lookup("no-colors"))
	viper.BindPFlag("rpc.user", rootCmd.PersistentFlags().Lookup("rpc.user"))
	viper.BindPFlag("rpc.password", rootCmd.PersistentFlags().Lookup("rpc.password"))
	viper.BindPFlag("remote.address", rootCmd.PersistentFlags().Lookup("remote.address"))
	viper.BindPFlag("rpc.https", rootCmd.PersistentFlags().Lookup("rpc.https"))
}
