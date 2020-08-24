package txcmd

import (
	"fmt"
	"io"

	restclient "github.com/make-os/lobe/api/remote/client"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/api/utils"
	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/colorfmt"
	"github.com/ncodes/go-prettyjson"
	"github.com/pkg/errors"
)

// GetArgs contains arguments for GetCmd.
type GetArgs struct {

	// Hash is the transaction hash
	Hash string

	// Status indicates that only the status info is requested
	Status bool

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// GetTransaction is a function for getting a finalized transaction
	GetTransaction utils.TxGetter

	Stdout io.Writer
}

// GetCmd gets a finalized transaction
func GetCmd(args *GetArgs) error {

	data, err := args.GetTransaction(args.Hash, args.RPCClient, args.RemoteClients)
	if err != nil {
		if reqErr, ok := errors.Cause(err).(*util.ReqError); ok && reqErr.HttpCode == 404 {
			return fmt.Errorf("unknown transaction")
		}
		return errors.Wrap(err, "failed to get transaction")
	}

	if args.Stdout != nil {

		if args.Status {
			fmt.Print("Status: ")
			switch data.Status {
			case modules.TxStatusInBlock:
				fmt.Fprintln(args.Stdout, colorfmt.GreenString("Confirmed"))
			case modules.TxStatusInMempool:
				fmt.Fprintln(args.Stdout, colorfmt.YellowString("In Mempool"))
			case modules.TxStatusInPushpool:
				fmt.Fprintln(args.Stdout, colorfmt.YellowString("In Pushpool"))
			}
			return nil
		}

		f := prettyjson.NewFormatter()
		f.NewlineArray = ""
		bz, _ := f.Marshal(data)
		fmt.Fprint(args.Stdout, string(bz))
	}

	return nil
}
