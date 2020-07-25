package txcmd

import (
	"fmt"
	"io"

	"github.com/ncodes/go-prettyjson"
	"github.com/pkg/errors"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
)

// GetArgs contains arguments for GetCmd.
type GetArgs struct {

	// Hash is the transaction hash
	Hash string

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
		return errors.Wrap(err, "failed to get transaction")
	}

	if args.Stdout != nil {
		f := prettyjson.NewFormatter()
		f.NewlineArray = ""
		bz, _ := f.Marshal(data)
		fmt.Fprint(args.Stdout, string(bz))
	}

	return nil
}
