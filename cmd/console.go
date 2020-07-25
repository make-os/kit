package cmd

import (
	"github.com/spf13/cobra"
	"gitlab.com/makeos/lobe/console"
	"gitlab.com/makeos/lobe/node"
)

// consoleCmd represents the console command
var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Start a JavaScript console mode and start the node",
	Run: func(cmd *cobra.Command, args []string) {

		// Start the node and also start the console after the node has started
		start(func(n *node.Node) {
			console := console.New(cfg)

			// On stop, close the node and interrupt other processes
			console.OnStop(func() {
				itr.Close()
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
	rootCmd.AddCommand(consoleCmd)
	setStartFlags(consoleCmd)
}
