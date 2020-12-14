package startcmd

import (
	context2 "context"
	"os"
	"os/signal"
	"syscall"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/node"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func start(onStart func(n *node.Node)) {

	log := cfg.G().Log.Module("main")

	// Create the node
	n := node.NewNode(context2.Background(), cfg)

	// Start the node
	if err := n.Start(); err != nil {
		log.Fatal("Failed to prepare node", "Err", err)
	}

	// Once all processes have been started call the onStart callback
	// so the caller can perform other operations that rely on the already
	// started processes.
	if onStart != nil {
		onStart(n)
	}

	config.GetInterrupt().Wait()
	n.Stop()
}

func listenForInterrupt() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		config.GetInterrupt().Close()
	}()
}

// StartCmd represents the start command
var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the node to join the network.",
	Run: func(cmd *cobra.Command, args []string) {
		listenForInterrupt()
		start(nil)
	},
}

func setStartFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("node.address", config.DefaultNodeAddress, "Set the node's p2p listening address")
	f.Bool("rpc.on", false, "Start the RPC service")
	f.Bool("rpc.disableauth", false, "Disable RPC authentication")
	f.Bool("rpc.authpubmethod", false, "Enable RPC authentication for non-private methods")
	f.String("rpc.tmaddress", config.DefaultTMRPCAddress, "Set tendermint RPC listening address")
	f.Bool("node.validator", false, "Run the node in validator mode")
	f.String("dht.address", config.DefaultDHTAddress, "Set the DHT listening address")
	f.String("node.addpeer", "", "Connect to one or more persistent node")
	f.Bool("dht.on", true, "Run the DHT service and join the network")
	f.String("dht.addpeer", "", "Register bootstrap peers for joining the DHT network")
	f.StringSlice("node.exts", []string{}, "Specify an extension to run on startup")
	f.StringSliceP("repo.track", "t", []string{}, "Specify one or more repositories to track")
	f.StringSliceP("repo.untrack", "u", []string{}, "Untrack one or more repositories")
	f.BoolP("repo.untrackall", "x", false, "Untrack all previously tracked repositories")

	// Light node primary
	f.Bool("node.light", false, "Run the node in light mode")
	f.String("node.primary", "", "Set light node's primary node address")
	f.StringSlice("node.witaddress", nil,
		"Set the witnesses address for cross-checking a light node's primary")
	f.Int("node.maxopenconns", 900,
		"Maximum number of simultaneous connections to the light node's RPC proxy server")
	f.Duration("node.period", config.DefaultLightNodeTrustPeriod,
		"Light node trusting period within which an header can be verified")
	f.Int64("node.height", 0, "Light node's trusted header height")
	f.String("node.hash", "", "Light node's trusted header hash")
	f.String("node.trustlevel", "1/3", "Light node's trusted level. Must be between 1/3 and 3/3")
	f.Bool("node.sequential", false,
		"Let the light node use sequential verification to verify headers instead of skipping verification")

	extArgsMap := map[string]string{}
	f.StringToStringVar(&extArgsMap, "node.extsargs", map[string]string{}, "Specify arguments for extensions")
	viper.Set("node.extsargs", extArgsMap)

	if len(os.Args) > 1 && os.Args[1] == cmd.Name() {
		_ = viper.BindPFlags(cmd.Flags())
	}
}

func init() {
	setStartFlags(StartCmd)
}
