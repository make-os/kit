package api

// SendTxPayloadResponse is the response for a transaction
// payload successfully added to the mempool pool.
type SendTxPayloadResponse struct {
	Hash string `json:"hash"`
}

// GetAccountNonceResponse is the response of a request for an account's nonce.
type GetAccountNonceResponse struct {
	Nonce string `json:"nonce"`
}
