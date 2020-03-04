package client

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api/rest"
	types2 "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// GPGGetNonceOfOwner returns the nonce of the gpg owner account
// Body:
// - gpgID <string>: The GPG public key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <AccountGetNonceResponse>
func (c *Client) GPGGetNonceOfOwner(gpgID string, blockHeight ...uint64) (*types2.AccountGetNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(rest.RestV1Path(types.NamespaceGPG, rest.MethodNameOwnerNonce), M{
		"id":          gpgID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var result types2.AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// GPGFind finds a GPG public key matching the given ID
// Body:
// - gpgID <string>: The GPG public key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.GPGPubKey>
func (c *Client) GPGFind(gpgID string, blockHeight ...uint64) (*state.GPGPubKey, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(rest.RestV1Path(types.NamespaceGPG, rest.MethodNameGPGFind), M{
		"id":          gpgID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var result state.GPGPubKey
	return &result, resp.ToJSON(&result)
}

// GPGGetNextNonceOfOwnerUsingClients gets the next account nonce of the owner of the
// gpg key by querying the given Remote API clients until one succeeds.
func GPGGetNextNonceOfOwnerUsingClients(clients []RestClient, gpgID string) (string, error) {
	var errs = []string{}
	for i, cl := range clients {
		var resp *types2.AccountGetNonceResponse
		resp, err := cl.GPGGetNonceOfOwner(gpgID)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "client[%d]", i).Error())
			continue
		}
		nonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
		return fmt.Sprintf("%d", nonce+1), nil
	}
	return "", fmt.Errorf("%s", strings.Join(errs, ", "))
}
