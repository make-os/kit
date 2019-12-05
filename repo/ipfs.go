package repo

import (
	"context"
	"os"

	files "github.com/ipfs/go-ipfs-files"
	client "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/util/logger"
)

// Ipfs provides a data pinning layer for making git objects accessible to all
// nodes on the IPFS public network
type Ipfs struct {
	cfg *config.EngineConfig
	log logger.Logger
	api *client.HttpApi
}

// newIpfs returns an instance of ipfs
func newIpfs(cfg *config.EngineConfig) *Ipfs {
	return &Ipfs{cfg: cfg, log: cfg.G().Log.Module("repo/ipfs")}
}

// init creates an ipfs repo if not already created
func (i *Ipfs) init(ctx context.Context) error {
	i.log.Debug("Initializing IPFS repository")
	api, err := client.NewLocalApi()
	if err != nil {
		return err
	}
	i.api = api
	return nil
}

// AddFile adds a file to the object store
func (i *Ipfs) AddFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}

	rf, err := files.NewReaderPathFile(path, f, st)
	if err != nil {
		return "", err
	}

	res, err := i.api.Unixfs().Add(
		ctx,
		rf,
		options.Unixfs.RawLeaves(true),
		options.Unixfs.Nocopy(true))
	if err != nil {
		return "", err
	}

	if err := i.api.Pin().Add(ctx, res); err != nil {
		return "", err
	}

	return res.Cid().Hash().B58String(), nil
}
