package client

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api/rest"
	apitypes "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// AccountGetNonce returns the nonce of the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *Client) AccountGetNonce(address string, blockHeight ...uint64) (*apitypes.AccountGetNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := rest.V1Path(constants.NamespaceUser, rest.MethodNameGetNonce)
	resp, err := c.get(path, M{"address": address, "blockHeight": height})
	if err != nil {
		return nil, err
	}

	var result apitypes.AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// AccountGet returns the account corresponding to the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *Client) AccountGet(address string, blockHeight ...uint64) (*state.Account, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := rest.V1Path(constants.NamespaceUser, rest.MethodNameGetAccount)
	resp, err := c.get(path, M{"address": address, "blockHeight": height})
	if err != nil {
		return nil, err
	}

	var acct, m = state.BareAccount(), util.Map{}
	_ = resp.ToJSON(&m)
	if err = acct.FromMap(m); err != nil {
		return nil, err
	}

	return acct, nil
}

// AccountGetNextNonceUsingClients gets the next nonce of an account by
// querying the given Remote API clients until one succeeds.
func AccountGetNextNonceUsingClients(clients []RestClient, address string) (string, error) {
	var errs []string
	for i, cl := range clients {
		resp, err := cl.AccountGetNonce(address)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "client[%d]", i).Error())
			continue
		}
		nonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
		return fmt.Sprintf("%d", nonce+1), nil
	}

	return "", fmt.Errorf("%s", strings.Join(errs, ", "))
}
