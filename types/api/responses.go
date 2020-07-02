package api

import "gitlab.com/makeos/mosdef/types/state"

// SendTxPayloadResponse is the response for a transaction
// payload successfully added to the mempool pool.
type SendTxPayloadResponse struct {
	Hash string `json:"hash"`
}

// GetAccountNonceResponse is the response of a request for an account's nonce.
type GetAccountNonceResponse struct {
	Nonce string `json:"nonce"`
}

// GetAccountResponse is the response of a request for an account.
type GetAccountResponse struct {
	*state.Account
}

// GetAccountResponse is the response of a request for a push key.
type GetPushKeyResponse struct {
	*state.PushKey
}

// CreateRepoResponse is the response of a request to create a repository
type CreateRepoResponse struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// GetRepoResponse is the response of a request to get a repository
type GetRepoResponse struct {
	*state.Repository
}
