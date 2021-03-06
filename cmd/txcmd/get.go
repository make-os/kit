package txcmd

import (
	"fmt"
	"io"

	types2 "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util/api"
	"github.com/make-os/kit/util/colorfmt"
	errors2 "github.com/make-os/kit/util/errors"
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
	RPCClient types.Client

	// GetTransaction is a function for getting a finalized transaction
	GetTransaction api.TxGetter

	Stdout io.Writer
}

// GetCmd gets a finalized transaction
func GetCmd(args *GetArgs) error {

	data, err := args.GetTransaction(args.Hash, args.RPCClient)
	if err != nil {
		if reqErr, ok := errors.Cause(err).(*errors2.ReqError); ok && reqErr.HttpCode == 404 {
			return fmt.Errorf("unknown transaction")
		}
		return errors.Wrap(err, "failed to get transaction")
	}

	if args.Stdout != nil {

		if args.Status {
			fmt.Print("Status: ")
			switch data.Status {
			case types2.TxStatusInBlock:
				fmt.Fprintln(args.Stdout, colorfmt.GreenString("Confirmed"))
			case types2.TxStatusInMempool:
				fmt.Fprintln(args.Stdout, colorfmt.YellowStringf("In Mempool"))
			case types2.TxStatusInPushpool:
				fmt.Fprintln(args.Stdout, colorfmt.YellowStringf("In Pushpool"))
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
