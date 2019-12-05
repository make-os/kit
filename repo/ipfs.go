package repo

import (
	"context"
	"fmt"
	ma "github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	ipfscfg "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/k0kubun/pp"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
)

// Ipfs provides a data pinning layer for making git objects accessible to all
// nodes on the IPFS public network
type Ipfs struct {
	cfg  *config.EngineConfig
	log  logger.Logger
	core icore.CoreAPI
}

// newIpfs returns an instance of ipfs
func newIpfs(cfg *config.EngineConfig) *Ipfs {
	return &Ipfs{cfg: cfg, log: cfg.G().Log.Module("repo/ipfs")}
}

// Creates an IPFS node and returns its coreAPI
func (i *Ipfs) createNode(ctx context.Context, repoPath string) (icore.CoreAPI, error) {

	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	// Construct the node
	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	if err != nil {
		return nil, err
	}

	// Attach the Core API to the constructed node
	return coreapi.NewCoreAPI(node)
}

func (i *Ipfs) createRepo(ctx context.Context) (string, error) {

	repoPath := i.cfg.GetObjectStoreDir()

	// Create a config with default options and a 2048 bit key
	cfg, err := ipfscfg.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	cfg.Experimental.FilestoreEnabled = true

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init node: %s", err)
	}

	return repoPath, nil
}

func setupPlugins(externalPluginsPath string) error {

	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

// init creates an ipfs repo if not already created
func (i *Ipfs) init(ctx context.Context) error {
	i.log.Debug("Initializing IPFS repository")

	if err := setupPlugins(""); err != nil {
		return errors.Wrap(err, "failed to load plugins")
	}

	repoPath, err := i.createRepo(ctx)
	if err != nil {
		pp.Println(err)
		return errors.Wrap(err, "failed to create repo")
	}

	i.core, err = i.createNode(ctx, repoPath)
	if err != nil {
		return errors.Wrap(err, "failed to create node")
	}

	bootstrapNodes := []string{
		// IPFS Bootstrapper nodes.
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

		// IPFS Cluster Pinning nodes
		"/ip4/138.201.67.219/tcp/4001/ipfs/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		"/ip4/138.201.67.220/tcp/4001/ipfs/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		"/ip4/138.201.68.74/tcp/4001/ipfs/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		"/ip4/94.130.135.167/tcp/4001/ipfs/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",
	}

	err = connectToPeers(ctx, i.core, bootstrapNodes)
	if err != nil {
		pp.Println(err.Error())
	}
	pp.Println("INitialized")

	return nil
}

func connectToPeers(ctx context.Context, ipfs icore.CoreAPI, peers []string) error {
	var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peerstore.PeerInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peerstore.InfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peerstore.PeerInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peerstore.PeerInfo) {
			defer wg.Done()
			err := ipfs.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
			}
		}(peerInfo)
	}
	wg.Wait()
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

	cidFile, err := i.core.Unixfs().Add(
		ctx,
		rf,
		options.Unixfs.RawLeaves(true),
		options.Unixfs.Nocopy(true),
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to add file")
	}

	return cidFile.Cid().Hash().B58String(), nil
}

// func (i *Ipfs) Get(cid string) {
// node, err := i.core.Unixfs().Get(context.Background(), icorepath.New(cid))
// pp.Println(err)
// pp.Println(node.Size())
// }
