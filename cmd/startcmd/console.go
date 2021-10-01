package startcmd

import (
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/console"
	"github.com/make-os/kit/node"
	"github.com/spf13/cobra"
)

var (
	cfg = config.GetConfig()
	log = cfg.G().Log
)

// ConsoleCmd represents the console command
var ConsoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Start a JavaScript console and connect the node to the network",
	Run: func(cmd *cobra.Command, args []string) {
		listenForInterrupt()

		// Start the node and also start the console after the node has started
		start(func(n *node.Node) {
			console := console.New(cfg)

			// On stop, close the node and interrupt other processes
			console.OnStop(func() {
				n.Stop()
			})

			// Register JS module hub
			console.SetModulesHub(n.GetModulesHub())

			// Run the console
			go func() {
				if err := console.Run(); err != nil {
					log.Fatal(err.Error())
				}
			}()
		})
	},
}

func init() {
	setStartFlags(ConsoleCmd)
}
