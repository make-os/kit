package types

type TxSendPayloadResponse struct {
	Hash string `json:"hash"`
}

// AccountGetNonceResponse is the response of AccountGetNonce endpoint
type AccountGetNonceResponse struct {
	Nonce string `json:"nonce"`
}
