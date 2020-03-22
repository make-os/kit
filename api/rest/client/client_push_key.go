package client

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api/rest"
	types2 "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
)

// PushKeyGetNonceOfOwner returns the nonce of the push key owner account
// Body:
// - pushKeyID <string>: The push key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <AccountGetNonceResponse>
func (c *Client) PushKeyGetNonceOfOwner(pushKeyID string, blockHeight ...uint64) (*types2.AccountGetNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(rest.RestV1Path(constants.NamespacePushKey, rest.MethodNameOwnerNonce), M{
		"id":          pushKeyID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var result types2.AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// PushKeyFind finds a push key by its ID
// Body:
// - pushKeyID <string>: The push key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.PushKey>
func (c *Client) PushKeyFind(pushKeyID string, blockHeight ...uint64) (*state.PushKey, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(rest.RestV1Path(constants.NamespacePushKey, rest.MethodNamePushKeyFind), M{
		"id":          pushKeyID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var result state.PushKey
	return &result, resp.ToJSON(&result)
}

// PushKeyGetNextNonceOfOwnerUsingClients gets the next account nonce of the owner of the
// push key by querying the given Remote API clients until one succeeds.
func PushKeyGetNextNonceOfOwnerUsingClients(clients []RestClient, pushKeyID string) (string, error) {
	var errs = []string{}
	for i, cl := range clients {
		var resp *types2.AccountGetNonceResponse
		resp, err := cl.PushKeyGetNonceOfOwner(pushKeyID)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "client[%d]", i).Error())
			continue
		}
		nonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
		return fmt.Sprintf("%d", nonce+1), nil
	}
	return "", fmt.Errorf("%s", strings.Join(errs, ", "))
}
