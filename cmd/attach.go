package cmd

import (
	"net"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gitlab.com/makeos/lobe/api/rpc/client"
	apitypes "gitlab.com/makeos/lobe/api/types"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/console"
	"gitlab.com/makeos/lobe/keystore"
	"gitlab.com/makeos/lobe/modules"
)

// connectToServer creates an RPC client to configured remote server.
// It will test the connection by getting the RPC methods supported
// by the server. Returns both client and RPC methods on success.
func connectToServer(cfg *config.AppConfig) (client.Client, *apitypes.GetMethodResponse, error) {
	host, port, err := net.SplitHostPort(cfg.RPC.Address)
	if err != nil {
		return nil, nil, err
	}

	cl := client.NewClient(&client.Options{
		Host:     host,
		Port:     cast.ToInt(port),
		HTTPS:    cfg.RPC.HTTPS,
		User:     cfg.RPC.User,
		Password: cfg.RPC.Password,
	})

	methods, err := cl.GetMethods()
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
			log.Fatal(errors.Wrapf(err, "failed to connect to RPC server @ %s", cfg.RPC.Address).Error())
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

	f := attachCmd.Flags()
	f.String("rpc.address", config.DefaultRPCAddress, "Set the RPC server address to connect to")
	f.Bool("rpc.https", false, "Connect using HTTPS protocol")
	f.String("rpc.user", "", "Set the RPC username")
	f.String("rpc.password", "", "Set the RPC password")
	f.String("exec", "", "Execute the given JavaScript code")
	viperBindFlagSet(attachCmd)
}
