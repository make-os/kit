package types

import (
	"github.com/makeos/mosdef/storage"
)

// Keepers provides an interface that allows
// access to all known state keepers. Its the central
// point for accessing all kinds of app state.
type Keepers interface {
	GetBlockKeeper() BlockKeeper
	GetSystemKeeper() SystemKeeper
	GetDB() storage.Engine
}

// KeeperCommon describes a common interface for all keepers
type KeeperCommon interface {
}

// BlockKeeper provides an interface for storing blocks
// and other associated information
type BlockKeeper interface {
	KeeperCommon
}

// SystemKeeper manages general, non-specific states.
type SystemKeeper interface {
	KeeperCommon
}
