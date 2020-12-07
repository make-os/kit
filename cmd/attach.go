package cmd

import (
	"net"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/console"
	"github.com/make-os/kit/keystore"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/rpc/client"
	"github.com/make-os/kit/rpc/types"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// connectToServer creates an RPC client to configured remote server.
// It will test the connection by getting the RPC methods supported
// by the server. Returns both client and RPC methods on success.
func connectToServer(cfg *config.AppConfig) (types.Client, []rpc.MethodInfo, error) {
	host, port, err := net.SplitHostPort(cfg.Remote.Address)
	if err != nil {
		return nil, nil, err
	}

	cl := client.NewClient(&types.Options{
		Host:     host,
		Port:     cast.ToInt(port),
		HTTPS:    cfg.RPC.HTTPS,
		User:     cfg.RPC.User,
		Password: cfg.RPC.Password,
	})

	methods, err := cl.RPC().GetMethods()
	if err != nil {
		return nil, nil, err
	}

	return cl, methods, nil
}

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Start a JavaScript console attached to a node",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("attachmode", true)
		execCode, _ := cmd.Flags().GetString("exec")

		// Connect to the remote RPC server
		rpcClient, _, err := connectToServer(cfg)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "failed to connect to RPC server @ %s", cfg.Remote.Address).Error())
		}

		// Set up console
		console := console.New(cfg)
		ks := keystore.New(cfg.KeystoreDir())
		console.SetModulesHub(modules.NewAttachable(cfg, rpcClient, ks))
		console.OnStop(func() {
			itr.Close()
		})

		// Run the console
		go func() {
			if err := console.Run(execCode); err != nil {
				log.Fatal(err.Error())
			}
		}()

		itr.Wait()
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().String("exec", "", "Execute the given JavaScript code")
}
