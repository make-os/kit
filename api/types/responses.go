package types

type SendTxPayloadResponse struct {
	Hash string `json:"hash"`
}

// GetAccountNonceResponse is the response of GetAccountNonce endpoint
type GetAccountNonceResponse struct {
	Nonce string `json:"nonce"`
}
