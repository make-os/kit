package streamer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/make-os/lobe/config"
	dht2 "github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/dht/providertracker"
	types3 "github.com/make-os/lobe/dht/streamer/types"
	types4 "github.com/make-os/lobe/dht/types"
	"github.com/make-os/lobe/pkgs/cache"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/remote/types"
	types2 "github.com/make-os/lobe/types"
	"github.com/make-os/lobe/util/io"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	plumb "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	ErrNoProviderFound        = fmt.Errorf("no provider found")
	ErrEndObjMustExistLocally = fmt.Errorf("end object must already exist in the local repo")
)

var (
	ObjectStreamerProtocolID = protocol.ID("/object/1.0")
)

// MakeHaveCacheKey returns a key for storing HaveCache entries.
func MakeHaveCacheKey(repoName string, hash plumb.Hash) string {
	return repoName + hash.String()
}

// HaveCache describes a module for keeping track of objects known to peers
type HaveCache interface {
	GetCache(peerID string) *cache.Cache
}

// BasicHaveCache implements HaveCache. It is a LRU cache for keeping
// track of objects known to peers
type BasicHaveCache struct {
	cache *cache.Cache
}

// newHaveCache creates an instance of BasicHaveCache
func newHaveCache(cap int) *BasicHaveCache {
	return &BasicHaveCache{cache: cache.NewCacheWithExpiringEntry(cap)}
}

// GetCache a cache by peerID. If peer has no cache, one is created and returned.
// A peer cache will have a expiry time after which it is removed.
func (c *BasicHaveCache) GetCache(peerID string) *cache.Cache {
	peerCache := c.cache.Get(peerID)
	if peerCache == nil {
		newCache := cache.NewCache(100000)
		c.cache.Add(peerID, newCache, time.Now().Add(15*time.Minute))
		return newCache
	}
	return peerCache.(*cache.Cache)
}

// BasicObjectStreamer implements ObjectStreamer. It provides a mechanism for
// announcing or transferring repository objects to/from the DHT.
type BasicObjectStreamer struct {
	dht              types4.DHT
	log              logger.Logger
	reposDir         string
	gitBinPath       string
	tracker          types4.ProviderTracker
	OnWantHandler    WantSendHandler
	OnSendHandler    WantSendHandler
	HaveCache        HaveCache
	RepoGetter       repo.GetLocalRepoFunc
	PackObject       plumbing.CommitPacker
	MakeRequester    MakeObjectRequester
	PackObjectGetter plumbing.PackObjectFinder
}

// NewObjectStreamer creates an instance of BasicObjectStreamer
func NewObjectStreamer(dht types4.DHT, cfg *config.AppConfig) *BasicObjectStreamer {
	ce := &BasicObjectStreamer{
		dht:              dht,
		reposDir:         cfg.GetRepoRoot(),
		log:              cfg.G().Log.Module("object-streamer"),
		gitBinPath:       cfg.Node.GitBinPath,
		tracker:          providertracker.New(),
		HaveCache:        newHaveCache(1000),
		RepoGetter:       repo.GetWithLiteGit,
		PackObject:       plumbing.PackObject,
		PackObjectGetter: plumbing.GetObjectFromPack,
	}

	// Hook concrete functions to function type fields
	ce.OnWantHandler = ce.OnWantRequest
	ce.OnSendHandler = ce.OnSendRequest
	ce.MakeRequester = makeRequester

	dht.Host().SetStreamHandler(ObjectStreamerProtocolID, ce.Handler)
	return ce
}

// SetProviderTracker overwrites the default provider tracker.
func (c *BasicObjectStreamer) SetProviderTracker(t types4.ProviderTracker) {
	c.tracker = t
}

// GetProviders find providers that may be able to provide an object.
//
// It finds providers that have announced their ability to provide an object.
// It also finds providers that have announce their ability to provide a
// repository - these providers are used as fallback in cases where an object
// may exist in a repository but not announced.
//
// TODO: In the future, we should sort the providers by rank such that hosts,
//  popular remotes and good-behaved providers are prioritized.
func (c *BasicObjectStreamer) GetProviders(ctx context.Context, repoName string, objKey []byte) ([]peer.AddrInfo, error) {

	// First, get providers that can provide the target object
	objProviders, err := c.dht.GetProviders(ctx, objKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get providers of target object")
	}

	// Get providers that can provide the repository.
	// When an error occurred, only return it if no providers so far.
	repoProviders, err := c.dht.GetProviders(ctx, []byte(repoName))
	if err != nil && len(objProviders) == 0 {
		return nil, errors.Wrap(err, "failed to get providers of target repo")
	}

	// Add unique repo provider to the object providers list
	for _, rp := range repoProviders {
		found := false
		for _, op := range objProviders {
			if rp.ID.Pretty() == op.ID.Pretty() {
				found = true
				break
			}
		}
		if !found {
			objProviders = append(objProviders, rp)
		}
	}

	return objProviders, nil
}

// GetCommit gets a single commit by hash.
// It returns the packfile, the commit object and error.
func (c *BasicObjectStreamer) GetCommit(
	ctx context.Context,
	repoName string,
	hash []byte) (io.ReadSeekerCloser, *object.Commit, error) {

	// Find providers of the object
	providers, err := c.GetProviders(ctx, repoName, hash)
	if err != nil {
		return nil, nil, err
	}

	// Remove banned providers or providers that have recently sent NOPE as
	// response to previous request for the key
	providers = funk.Filter(providers, func(p peer.AddrInfo) bool {
		return c.tracker.IsGood(p.ID) && !c.tracker.DidPeerSendNope(p.ID, hash)
	}).([]peer.AddrInfo)

	// Return immediate with error if no provider was found
	if len(providers) == 0 {
		return nil, nil, ErrNoProviderFound
	}

	// Register the providers we can track its behaviour over time.
	c.tracker.Register(providers...)

	// Start request session
	req := c.MakeRequester(RequestArgs{
		Providers:       providers,
		RepoName:        repoName,
		Key:             hash,
		Host:            c.dht.Host(),
		Log:             c.log,
		ReposDir:        c.reposDir,
		ProviderTracker: c.tracker,
	})

	// Do the request
	res, err := req.Do(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "request failed")
	}

	// Get the commit from the packfile
	commit, err := c.PackObjectGetter(res.Pack, plumbing.BytesToHex(hash))
	if err != nil {
		c.tracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, errors.Wrap(err, "failed to get target commit from packfile")
	}

	// Ensure the commit exist in the packfile.
	if commit == nil {
		c.tracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, fmt.Errorf("target commit not found in the packfile")
	}

	c.log.Debug("New object downloaded", "Hash", commit.ID().String(), "Repo", repoName)

	return res.Pack, commit.(*object.Commit), nil
}

// GetCommitWithAncestors gets a commit and its ancestors that do not exist in the local repository.
//
// It stops fetching ancestors when it finds an ancestor matching the given end commit hash.
// If EndHash is true, it is expected that the EndHash commit exist locally.
// It will skip the start commit if it exist locally but try to add its parent to the internal wantlist allowing
// it to find ancestors that may have not been fetched before.
// Packfiles returned are expected to be closed by the caller.
// If ResultCB is set, packfiles will be passed to the callback and not returned.
// If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
// with a nil error.
func GetCommitWithAncestors(
	ctx context.Context,
	c types3.ObjectStreamer,
	repoGetter repo.GetLocalRepoFunc,
	args types3.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {

	// Get the target repo
	var r types.LocalRepo
	r, err = repoGetter(args.GitBinPath, filepath.Join(args.ReposDir, args.RepoName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get repo")
	}

	// If end commit hash is specified, ensure it exists locally.
	endCommitHash := plumbing.BytesToHex(args.EndHash)
	if endCommitHash != "" {
		if !r.ObjectExist(endCommitHash) {
			return nil, ErrEndObjMustExistLocally
		}
	}

	// Maintain a wantlist containing commit objects to fetch, starting with the start commit.
	var wantlist = [][]byte{args.StartHash}
	var fetched = map[string]struct{}{}
	var endCommitSeen bool
	for len(wantlist) > 0 {

		// Get a hash
		target := wantlist[0]
		wantlist = wantlist[1:]

		// Skip if already fetched
		targetHash := plumbing.BytesToHex(target)
		if _, ok := fetched[targetHash]; ok {
			continue
		}

		var fetchedCommit *object.Commit
		var pack io.ReadSeekerCloser

		// Check if the start commit exist locally. If it does, we don't need to re-fetch it from the network.
		// Instead, use the local version as the fetched commit and skip to processing.
		if bytes.Equal(args.StartHash, target) {
			startObj, err := r.CommitObject(plumb.NewHash(targetHash))
			if err != nil && err != plumb.ErrObjectNotFound {
				return packfiles, err
			} else if startObj != nil {
				fetchedCommit = startObj
				goto processFetched
			}
		}

		// Attempt to get the packfile of the commit from the network
		pack, fetchedCommit, err = c.GetCommit(ctx, args.RepoName, target)
		if err != nil {
			return packfiles, err
		}

	processFetched:

		// Cache successfully fetched commits.
		fetched[targetHash] = struct{}{}

		// Collect packfile if the fetched commit is not the end commit and ExcludeEndCommit is false
		fetchedHash := fetchedCommit.ID().String()
		if fetchedHash != endCommitHash || fetchedHash == endCommitHash && !args.ExcludeEndCommit {
			// Pass packfile to callback if provided, otherwise, use the regular packfiles result slice
			// Return immediately if the callback returns an error.
			// If the callback returns an error, return nil if err is an ErrExit.
			if args.ResultCB != nil && pack != nil {
				if err := args.ResultCB(pack, fetchedHash); err != nil {
					if err == types2.ErrExit {
						return packfiles, nil
					}
					return packfiles, err
				}
			} else if pack != nil {
				packfiles = append(packfiles, pack)
			}
		}

		// Skip immediately if fetched commit is the end commit
		if fetchedHash == endCommitHash {
			continue
		}

		// At this point, the fetched commit is not the end commit.
		// We need to add its parents that are currently unknown and un-fetched into the wantlist.
		_, endCommitFetched := fetched[endCommitHash]
		for len(fetchedCommit.ParentHashes) > 0 {

			parent := fetchedCommit.ParentHashes[0]
			fetchedCommit.ParentHashes = fetchedCommit.ParentHashes[1:]
			parentIsEndCommit := parent.String() == endCommitHash

			// If current parent is the end commit, set endCommitSeen flag to true.
			// Also skip the parent if ExcludeEndCommit is true.
			if parentIsEndCommit {
				endCommitSeen = true
				if args.ExcludeEndCommit {
					continue
				}
			}

			// If current parent is not the end commit, check whether it exist locally.
			// If it does, add its parent to the parents list of the fetched commit so that
			// they are processed in this loop as though they are parents of the fetched commit
			if !parentIsEndCommit {
				parentObj, err := r.CommitObject(parent)
				if err != nil && err != plumb.ErrObjectNotFound {
					return packfiles, err
				}
				if parentObj != nil {
					fetchedCommit.ParentHashes = append(fetchedCommit.ParentHashes, parentObj.ParentHashes...)
					continue
				}
			}

			// At this point, if the parent is not the end commit and the end commit
			// has been seen or fetched, we need to ensure the current parent is
			// not an ancestor of the end commit to avoid trying to fetch commits
			// that may already be available locally
			if !parentIsEndCommit && (endCommitFetched || endCommitSeen) {

				// If this parent is an ancestor of the end commit, it means
				// we already have the parent since it is already part of the
				// end commit history, as such, we skip t
				err := r.IsAncestor(parent.String(), endCommitHash)

				// However, if we do not have the parent commit locally
				// or it is not an ancestor of the end commit, it is okay
				// to add it to the wantlist.
				if err == plumb.ErrObjectNotFound || err == repo.ErrNotAnAncestor {
					goto add
				}

				// Return immediately if an unexpected error occurred.
				if err != nil && err != repo.ErrNotAnAncestor {
					return packfiles, errors.Wrap(err, "failed to perform ancestor check")
				}
				continue
			}

		add:
			if _, ok := fetched[parent.String()]; !ok {
				wantlist = append(wantlist, parent[:])
			}
		}
	}

	return
}

// GetTag gets a single annotated tag by hash.
//
// It returns the packfile, the tag object and error.
func (c *BasicObjectStreamer) GetTag(
	ctx context.Context,
	repoName string,
	hash []byte) (io.ReadSeekerCloser, *object.Tag, error) {

	// Find providers of the object
	providers, err := c.GetProviders(ctx, repoName, hash)
	if err != nil {
		return nil, nil, err
	}

	// Remove banned providers and providers that have recently
	// sent NOPE as response to previous request for the key
	providers = funk.Filter(providers, func(p peer.AddrInfo) bool {
		return c.tracker.IsGood(p.ID) && !c.tracker.DidPeerSendNope(p.ID, hash)
	}).([]peer.AddrInfo)

	// Return immediate with error if no provider was found
	if len(providers) == 0 {
		return nil, nil, ErrNoProviderFound
	}

	// Register the providers we can track its behaviour over time.
	c.tracker.Register(providers...)

	// Start request session
	req := c.MakeRequester(RequestArgs{
		Providers:       providers,
		RepoName:        repoName,
		Key:             hash,
		Host:            c.dht.Host(),
		Log:             c.log,
		ReposDir:        c.reposDir,
		ProviderTracker: c.tracker,
	})

	// Do the request
	res, err := req.Do(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "request failed")
	}

	// Get the tag from the packfile
	// If the packfile could not be read, ban peer for sending a bad packfile.
	tag, err := c.PackObjectGetter(res.Pack, plumbing.BytesToHex(hash))
	if err != nil {
		c.tracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, errors.Wrap(err, "failed to get target tag from packfile")
	}

	// Ensure the tag exist in the packfile
	// If tag is unset, ban peer for sending a packfile that did not contain the queried object.
	if tag == nil {
		c.tracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, fmt.Errorf("target tag not found in the packfile")
	}

	return res.Pack, tag.(*object.Tag), nil
}

// GetTaggedCommitWithAncestors gets the ancestors of the commit pointed by the given tag that
// do not exist in the local repository.
//
// - If the start tag points to another tag, the function is recursively called on the nested tag.
// - If the start tag does not point to a commit or a tag, the tag's packfile is returned.
// - If EndHash is set, it must be an already existing tag pointing to a commit or a tag.
//   If it points to a tag, same rule is applied to the tag recursively.
// - If EndHash is set, it will stop fetching ancestors when it finds an
//   ancestor matching the commit pointed by the end hash tag.
// - Packfiles returned are expected to be closed by the caller.
// - If ResultCB is set, packfiles will be passed to the callback as soon as they are received.
// - If ResultCB is set, empty slice will be returned by the method.
// - If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
//   with a nil error.
func GetTaggedCommitWithAncestors(
	ctx context.Context,
	st types3.ObjectStreamer,
	repoGetter repo.GetLocalRepoFunc,
	args types3.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {

	// Get the target repo
	var r types.LocalRepo
	r, err = repoGetter(args.GitBinPath, filepath.Join(args.ReposDir, args.RepoName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get repo")
	}

	// If end commit hash is specified, ensure it exists locally and
	// it's a tag object pointing to a commit or tag object.
	// If it points to a tag object, recursively check the tag's target
	// under same rule.
	endTagHash := plumbing.BytesToHex(args.EndHash)
	var endTagTargetHash plumb.Hash
	if endTagHash != "" {
	checkEndTag:
		endTag, err := r.GetObject(endTagHash)
		if err != nil {
			if err == plumb.ErrObjectNotFound {
				return nil, ErrEndObjMustExistLocally
			}
			return nil, err
		}
		if endTag.Type() != plumb.TagObject {
			return nil, fmt.Errorf("end hash must be a tag object")
		}
		switch endTag.(*object.Tag).TargetType {
		case plumb.CommitObject:
			endTagTargetHash = endTag.(*object.Tag).Target
		case plumb.TagObject:
			endTagHash = endTag.(*object.Tag).Target.String()
			goto checkEndTag
		default:
			return nil, fmt.Errorf("end tag must point to a tag or commit object")
		}
	}

	// Attempt to get the packfile of the tag.
	pack, startTag, err := st.GetTag(ctx, args.RepoName, args.StartHash)
	if err != nil {
		return packfiles, err
	}

	// Pass fetched tag to result slice of callback if callback is set
	if args.ResultCB != nil {
		if err := args.ResultCB(pack, startTag.Hash.String()); err != nil {
			if err == types2.ErrExit {
				return packfiles, nil
			}
			return packfiles, err
		}
	} else {
		packfiles = append(packfiles, pack)
	}

	switch startTag.TargetType {
	case plumb.TagObject:
		args.StartHash = startTag.Target[:]
		res, err := GetTaggedCommitWithAncestors(ctx, st, repoGetter, args)
		if err != nil {
			packfiles = append(packfiles, res...)
			return packfiles, err
		}
		packfiles = append(packfiles, res...)
		return packfiles, err

	case plumb.CommitObject:
		args.StartHash = startTag.Target[:]
		if endTagHash != "" {
			args.EndHash = endTagTargetHash[:]
		}
		res, err := st.GetCommitWithAncestors(ctx, args)
		if err != nil {
			packfiles = append(packfiles, res...)
			return packfiles, errors.Wrap(err, "failed to get ancestors of commit pointed by tag")
		}
		packfiles = append(packfiles, res...)

	default:
		return packfiles, nil
	}

	return
}

// GetCommitWithAncestors gets a commit and also its ancestors that do not exist in the local repository.
//
// It will stop fetching ancestors when it finds
// an ancestor matching the given end hash.
// If EndHash is true, it is expected that EndHash commit must exist locally.
// Packfiles returned are expected to be closed by the caller.
// If ResultCB is set, packfiles will be passed to the callback and not returned.
// If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
// with a nil error.
func (c *BasicObjectStreamer) GetCommitWithAncestors(ctx context.Context,
	args types3.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
	args.ReposDir, args.GitBinPath = c.reposDir, c.gitBinPath
	return GetCommitWithAncestors(ctx, c, c.RepoGetter, args)
}

// GetTaggedCommitWithAncestors gets the ancestors of the commit pointed by the given tag that
// do not exist in the local repository.
//
// - If EndHash is set, it must be an already existing tag pointing to a commit.
// - If EndHash is set, it will stop fetching ancestors when it finds an
//   ancestor matching the commit pointed by the end hash tag.
// - Packfiles returned are expected to be closed by the caller.
// - If ResultCB is set, packfiles will be passed to the callback as soon as they are received.
// - If ResultCB is set, empty slice will be returned by the method.
// - If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
//   with a nil error.
func (c *BasicObjectStreamer) GetTaggedCommitWithAncestors(ctx context.Context,
	args types3.GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
	args.ReposDir, args.GitBinPath = c.reposDir, c.gitBinPath
	return GetTaggedCommitWithAncestors(ctx, c, c.RepoGetter, args)
}

// Handler handles the lifecycle of the object streaming protocol
func (c *BasicObjectStreamer) Handler(s network.Stream) {
	for {
		success, err := c.OnRequest(s)
		if err != nil {
			return
		}
		if success {
			break
		}
	}
}

// OnRequest handles incoming commit object requests
func (c *BasicObjectStreamer) OnRequest(s network.Stream) (bool, error) {

	// Get request message
	msgType, repoName, hash, err := dht2.ReadWantOrSendMsg(s)
	if err != nil {
		return false, errors.Wrap(err, "failed to read request")
	}

	switch msgType {

	// Handle 'want' message
	case dht2.MsgTypeWant:
		err := c.OnWantHandler(repoName, hash, s)
		return false, err

	// Handle 'send' message
	case dht2.MsgTypeSend:
		err := c.OnSendHandler(repoName, hash, s)
		return err == nil, err

	default:
		return false, ErrUnknownMsgType
	}
}

type WantSendHandler func(repo string, hash []byte, s network.Stream) error

// OnWantRequest handles incoming "WANT" requests
func (c *BasicObjectStreamer) OnWantRequest(repo string, hash []byte, s network.Stream) error {

	remotePeerID := s.Conn().RemotePeer().Pretty()
	c.log.Debug("WANT<-: Received request for object", "Peer", remotePeerID)

	// Check if repo exist
	r, err := c.RepoGetter(c.gitBinPath, filepath.Join(c.reposDir, repo))
	if err != nil {
		s.Reset()
		c.log.Debug("failed repository check", "Err", err)
		return err
	}

	commitHash := plumbing.BytesToHex(hash)
	c.log.Debug("WANT<-: Parsed request for object", "Repo", repo, "Hash", commitHash,
		"Peer", remotePeerID)

	// Check if object exist in the repo
	if !r.ObjectExist(commitHash) {
		if _, err = s.Write(dht2.MakeNopeMsg()); err != nil {
			return errors.Wrap(err, "failed to write 'nope' message")
		}
		c.log.Debug("Requested object does not exist in repo", "Repo", repo, "Hash", commitHash)
		return dht2.ErrObjNotFound
	}

	c.log.Debug("WANT<-: Requested object exist in repo", "Repo", repo, "Hash", commitHash,
		"Peer", remotePeerID)

	// Respond with a 'have' message
	if _, err := s.Write(dht2.MakeHaveMsg()); err != nil {
		s.Reset()
		c.log.Error("failed to Write 'have' message", "Err", err)
		return err
	}

	c.log.Debug("WANT<-: Sent HAVE message", "Repo", repo, "Hash", commitHash,
		"Peer", remotePeerID)

	return nil
}

// OnSendRequest handles incoming "SEND" requests.
func (c *BasicObjectStreamer) OnSendRequest(repo string, hash []byte, s network.Stream) error {

	remotePeerID := s.Conn().RemotePeer().Pretty()
	c.log.Debug("SEND<-: Received message", "Peer", remotePeerID)

	// Check if repo exist
	r, err := c.RepoGetter(c.gitBinPath, filepath.Join(c.reposDir, repo))
	if err != nil {
		s.Reset()
		c.log.Debug("failed repository check", "Err", err)
		return err
	}

	// Get the object
	commitHash := plumbing.BytesToHex(hash)
	obj, err := r.GetObject(commitHash)
	if err != nil {
		s.Reset()

		if err != plumb.ErrObjectNotFound {
			c.log.Error("failed local object check", "Err", err)
			return err
		}

		c.log.Debug("SEND<-: Object requested was not found", "Repo", repo, "Hash",
			commitHash, "Peer", remotePeerID)

		if _, err = s.Write(dht2.MakeNopeMsg()); err != nil {
			return errors.Wrap(err, "failed to write 'nope' message")
		}

		return err
	}

	c.log.Debug("SEND<-: Processing message", "Repo", repo, "Hash", commitHash,
		"Peer", remotePeerID)

	peerHaveCache := c.HaveCache.GetCache(remotePeerID)

	// Get the packfile representation of the object.
	// Filter out objects that we know the peer may have.
	pack, objs, err := c.PackObject(r, &plumbing.PackObjectArgs{
		Obj: obj,
		Filter: func(hash plumb.Hash) bool {
			if !peerHaveCache.Has(MakeHaveCacheKey(repo, hash)) {
				return true
			}
			c.log.Debug("SEND<-: Skip object already sent to requester", "Hash",
				commitHash, "Peer", remotePeerID)
			return false
		},
	})
	if err != nil {
		s.Reset()
		return errors.Wrap(err, "failed to generate commit packfile")
	}

	// Write the packfile to the requester
	w := bufio.NewWriter(bufio.NewWriter(s))
	if _, err := w.ReadFrom(pack); err != nil {
		s.Reset()
		c.log.Error("failed to Write commit pack", "Err", err)
		return errors.Wrap(err, "Write commit pack error")
	}
	w.Flush()
	s.Close()

	// Add the packed object to the peer's have-cache
	for _, obj := range objs {
		if !peerHaveCache.Has(obj) {
			peerHaveCache.Add(MakeHaveCacheKey(repo, obj), struct{}{})
		}
	}

	c.log.Debug("->PACK: Wrote object(s) to requester", "Hash",
		commitHash, "Peer", remotePeerID, "Count", len(objs))

	return nil
}
