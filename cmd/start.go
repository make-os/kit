package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/node"
)

func start(onStart func(n *node.Node)) {

	log := cfg.G().Log.Module("main")

	// Create the node
	n := node.NewNode(cfg, tmconfig)

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

	itr.Wait()
	n.Stop()
}

func listenForInterrupt() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		itr.Close()
	}()
}

// startCmd represents the start command
var startCmd = &cobra.Command{
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
	f.String("rpc.user", "", "Set the RPC username")
	f.String("rpc.password", "", "Set the RPC password")
	f.Bool("rpc.disableauth", false, "Disable RPC authentication")
	f.Bool("rpc.authpubmethod", false, "Enable RPC authentication for non-private methods")
	f.Bool("node.validator", false, "Run the node in validator mode")
	f.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
	f.String("rpc.tmaddress", config.DefaultTMRPCAddress, "Set tendermint RPC listening address")
	f.String("dht.address", config.DefaultDHTAddress, "Set the DHT listening address")
	f.String("remote.address", config.DefaultRemoteServerAddress, "Set the remote server listening address")
	f.String("node.addpeer", "", "connect to one or more persistent node")
	f.Bool("dht.on", true, "Run the DHT service and join the network")
	f.String("dht.addpeer", "", "Register bootstrap peers for joining the DHT network")
	f.StringSlice("node.exts", []string{}, "Specify an extension to run on startup")
	extArgsMap := map[string]string{}
	f.StringToStringVar(&extArgsMap, "node.extsargs", map[string]string{}, "Specify arguments for extensions")
	viper.Set("node.extsargs", extArgsMap)
	viperBindFlagSet(cmd)
}

func init() {
	rootCmd.AddCommand(startCmd)
	setStartFlags(startCmd)
}
