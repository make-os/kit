package client

import (
	"time"

	"github.com/make-os/lobe/ticket/types"
	types2 "github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// TicketAPI provides access to ticket-related RPC methods
type TicketAPI struct {
	c *RPCClient
}

// Buy creates a transaction to buy a validator ticket
func (t *TicketAPI) Buy(body *api.BodyBuyTicket) (*api.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Delegate = body.Delegate
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, status, err := t.c.call("ticket_buy", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r api.ResultHash
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// BuyHost creates a transaction to buy a host ticket
func (t *TicketAPI) BuyHost(body *api.BodyBuyTicket) (*api.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Delegate = body.Delegate
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	// Set BLS public key. If not provided, derive a new one from the private key.
	tx.BLSPubKey = body.BLSPubKey
	if len(tx.BLSPubKey) == 0 {
		tx.BLSPubKey = body.SigningKey.PrivKey().BLSKey().Public().Bytes()
	}

	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, status, err := t.c.call("ticket_buyHost", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r api.ResultHash
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// List returns active validator tickets associated with a public key
func (t *TicketAPI) List(body *api.BodyTicketQuery) (res []*api.ResultTicket, err error) {

	q := util.Map{"proposer": body.ProposerPubKey, "queryOpts": util.ToMap(body.QueryOption)}
	resp, status, err := t.c.call("ticket_list", q)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r = objx.New(map[string]interface{}(resp))
	for _, t := range r.Get("tickets").InterSlice() {
		tm := objx.New(t)
		ticket := &types.Ticket{
			Type:           types2.TxCode(cast.ToInt(tm.Get("type").Inter())),
			BLSPubKey:      util.ToByteSlice(cast.ToIntSlice(tm.Get("blsPubKey").InterSlice())),
			CommissionRate: cast.ToFloat64(tm.Get("commissionRate").Inter()),
			Delegator:      cast.ToString(tm.Get("delegator").Inter()),
			ExpireBy:       cast.ToUint64(tm.Get("expiryBy").Inter()),
			Height:         cast.ToUint64(tm.Get("height").Inter()),
			Index:          cast.ToInt(tm.Get("index").Inter()),
			MatureBy:       cast.ToUint64(tm.Get("matureBy").Inter()),
			ProposerPubKey: util.BytesToBytes32(util.ToByteSlice(cast.ToIntSlice(tm.Get("proposerPubKey").
				InterSlice()))),
			Value: util.String(cast.ToString(tm.Get("value").Inter())),
		}
		ticket.Hash, _ = util.FromHex(cast.ToString(tm.Get("hash").Inter()))
		res = append(res, &api.ResultTicket{ticket})
	}
	return
}

// ListHost returns active hosts tickets associated with a public key
func (t *TicketAPI) ListHost(body *api.BodyTicketQuery) (res []*api.ResultTicket, err error) {

	q := util.Map{"proposer": body.ProposerPubKey, "queryOpts": util.ToMap(body.QueryOption)}
	resp, status, err := t.c.call("ticket_listHost", q)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r = objx.New(map[string]interface{}(resp))
	for _, t := range r.Get("tickets").InterSlice() {
		tm := objx.New(t)
		ticket := &types.Ticket{
			Type:           types2.TxCode(cast.ToInt(tm.Get("type").Inter())),
			BLSPubKey:      util.ToByteSlice(cast.ToIntSlice(tm.Get("blsPubKey").InterSlice())),
			CommissionRate: cast.ToFloat64(tm.Get("commissionRate").Inter()),
			Delegator:      cast.ToString(tm.Get("delegator").Inter()),
			ExpireBy:       cast.ToUint64(tm.Get("expiryBy").Inter()),
			Height:         cast.ToUint64(tm.Get("height").Inter()),
			Index:          cast.ToInt(tm.Get("index").Inter()),
			MatureBy:       cast.ToUint64(tm.Get("matureBy").Inter()),
			ProposerPubKey: util.BytesToBytes32(util.ToByteSlice(cast.ToIntSlice(tm.Get("proposerPubKey").
				InterSlice()))),
			Value: util.String(cast.ToString(tm.Get("value").Inter())),
		}
		ticket.Hash, _ = util.FromHex(cast.ToString(tm.Get("hash").Inter()))
		res = append(res, &api.ResultTicket{ticket})
	}
	return
}
