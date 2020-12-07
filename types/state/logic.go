package state

import "github.com/make-os/kit/util"

// BlockInfo describes information about a block
type BlockInfo struct {
	AppHash         util.Bytes     `json:"appHash"`
	LastAppHash     util.Bytes     `json:"lastAppHash"`
	Hash            util.Bytes     `json:"hash"`
	Height          util.Int64     `json:"height"`
	ProposerAddress util.TMAddress `json:"proposerAddress"`
	Time            util.Int64     `json:"time"`
}
