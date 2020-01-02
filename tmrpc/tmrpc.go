package tmrpc

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/imroc/req"
	"github.com/makeos/mosdef/util"
)

// TMRPC provides convenience features that enables
// easy access to relevant tendermint RPC endpoints.
type TMRPC struct {
	req     *req.Req
	address string
}

// New creates an instance of TMRPC
func New(address string) *TMRPC {
	return &TMRPC{
		req:     req.New(),
		address: address,
	}
}

// SendTx broadcasts a transaction and returns the transaction
// hash after it must have been validated using CheckTx.
func (tm *TMRPC) SendTx(tx []byte) (util.Bytes32, error) {

	var hash util.Bytes32
	var resData map[string]interface{}

	// Hex encode the tx and broadcast
	txData := hex.EncodeToString(tx)
	endpoint := fmt.Sprintf(`http://%s/broadcast_tx_sync?tx="%s"`, tm.address, txData)
	resp, err := tm.req.Get(endpoint)
	if err != nil {
		return hash, errors.Wrap(err, "failed to broadcast tx")
	}

	if resp.Response().StatusCode == 500 {
		return hash, fmt.Errorf("failed to broadcast tx: server error")
	}

	// If error, decode and return a simple error
	_ = resp.ToJSON(&resData)
	if resData["error"] != nil {
		errMsg := resData["error"].(map[string]interface{})["message"]
		errData := resData["error"].(map[string]interface{})["data"]
		return hash, fmt.Errorf("failed to broadcast tx: %s - %s", errMsg, errData)
	}

	hashHex := resData["result"].(map[string]interface{})["data"].(string)
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return hash, errors.Wrap(err, "failed to decode broadcast response")
	}

	return util.BytesToHash(hashBytes), nil
}

// GetBlock returns the block at the given height
func (tm *TMRPC) GetBlock(height int64) (map[string]interface{}, error) {

	var endpoint = fmt.Sprintf(`http://%s/block`, tm.address)
	if height > 0 {
		endpoint = fmt.Sprintf(`http://%s/block?height="%d"`, tm.address, height)
	}

	resp, err := tm.req.Get(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}

	if resp.Response().StatusCode == 500 {
		return nil, fmt.Errorf("failed to get block: server error")
	}

	var resData map[string]interface{}
	_ = resp.ToJSON(&resData)

	// If error, decode and return a simple error
	if resData["error"] != nil {
		errMsg := resData["error"].(map[string]interface{})["message"]
		errData := resData["error"].(map[string]interface{})["data"]
		return nil, fmt.Errorf("failed to get block: %s - %s", errMsg, errData)
	}

	return resData, nil
}
