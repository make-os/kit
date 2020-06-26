package streamer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	dht2 "gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/dht/providertracker"
	types4 "gitlab.com/makeos/mosdef/dht/server/types"
	types3 "gitlab.com/makeos/mosdef/dht/streamer/types"
	"gitlab.com/makeos/mosdef/pkgs/cache"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	types2 "gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util/io"
	plumb "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	ErrNoProviderFound        = fmt.Errorf("no provider found")
	ErrEndObjMustExistLocally = fmt.Errorf("end object must already exist in the local repo")
)

var (
	ObjectStreamerProtocolID = protocol.ID("/object/1.0")
	MsgTypeLen               = 4
)

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
	providerTracker  *providertracker.BasicProviderTracker
	OnWantHandler    func(msg []byte, s network.Stream) error
	OnSendHandler    func(msg []byte, s network.Stream) error
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
		providerTracker:  providertracker.NewProviderTracker(),
		HaveCache:        newHaveCache(1000),
		RepoGetter:       repo.GetWithLiteGit,
		PackObject:       plumbing.PackObject,
		PackObjectGetter: plumbing.GetObjectFromPack,
	}

	// Hook concrete functions to function type fields
	ce.OnWantHandler = ce.OnWant
	ce.OnSendHandler = ce.OnSend
	ce.MakeRequester = makeRequester

	dht.Host().SetStreamHandler(ObjectStreamerProtocolID, ce.Handler)
	return ce
}

// Announce announces an object's hash
func (c *BasicObjectStreamer) Announce(hash []byte, doneCB func(error)) {
	c.dht.Announce(dht2.MakeObjectKey(hash), doneCB)
}

// GetCommit gets a single commit by hash.
// It returns the packfile, the commit object and error.
func (c *BasicObjectStreamer) GetCommit(
	ctx context.Context,
	repoName string,
	hash []byte) (io.ReadSeekerCloser, *object.Commit, error) {

	// Find providers of the object
	key := dht2.MakeObjectKey(hash)
	providers, err := c.dht.GetProviders(ctx, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get providers")
	}

	// Return immediate with error if no provider was found
	if len(providers) == 0 {
		return nil, nil, ErrNoProviderFound
	}

	// Remove banned providers
	for i, prov := range providers {
		if good := c.providerTracker.IsGood(prov.ID); !good {
			providers = append(providers[:i], providers[i+1:]...)
		}
	}

	// Register the providers we can track its behaviour over time.
	c.providerTracker.Register(providers...)

	// Start request session
	req := c.MakeRequester(RequestArgs{
		Providers:       providers,
		RepoName:        repoName,
		Key:             key,
		Host:            c.dht.Host(),
		Log:             c.log,
		ReposDir:        c.reposDir,
		ProviderTracker: c.providerTracker,
	})

	// Do the request
	res, err := req.Do(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "request failed")
	}

	// Get the commit from the packfile
	// If the packfile could not be read, ban peer for sending a bad packfile.
	commit, err := c.PackObjectGetter(res.Pack, plumbing.BytesToHex(hash))
	if err != nil {
		c.providerTracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, errors.Wrap(err, "failed to get target commit from packfile")
	}

	// Ensure the commit exist in the packfile.
	// If commit is unset, ban peer for sending packfile that did not contain the queried object.
	if commit == nil {
		c.providerTracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, fmt.Errorf("target commit not found in the packfile")
	}

	c.log.Debug("New object downloaded", "Hash", commit.ID().String(), "Repo", repoName)

	return res.Pack, commit.(*object.Commit), nil
}

// GetCommitWithAncestors gets a commit and its ancestors that do not exist.
// in the local repository. It stops fetching ancestors when it finds an ancestor matching the given end commit hash.
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
				goto process_fetched
			}
		}

		// Attempt to get the packfile of the commit from the network
		pack, fetchedCommit, err = c.GetCommit(ctx, args.RepoName, target)
		if err != nil {
			return packfiles, err
		}

	process_fetched:

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
// It returns the packfile, the tag object and error.
func (c *BasicObjectStreamer) GetTag(
	ctx context.Context,
	repoName string,
	hash []byte) (io.ReadSeekerCloser, *object.Tag, error) {

	// Find providers of the object
	key := dht2.MakeObjectKey(hash)
	providers, err := c.dht.GetProviders(ctx, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get providers")
	}

	// Return immediate with error if no provider was found
	if len(providers) == 0 {
		return nil, nil, ErrNoProviderFound
	}

	// Remove banned providers
	for i, prov := range providers {
		if good := c.providerTracker.IsGood(prov.ID); !good {
			providers = append(providers[:i], providers[i+1:]...)
		}
	}

	// Register the providers we can track its behaviour over time.
	c.providerTracker.Register(providers...)

	// Start request session
	req := c.MakeRequester(RequestArgs{
		Providers:       providers,
		RepoName:        repoName,
		Key:             key,
		Host:            c.dht.Host(),
		Log:             c.log,
		ReposDir:        c.reposDir,
		ProviderTracker: c.providerTracker,
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
		c.providerTracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, errors.Wrap(err, "failed to get target tag from packfile")
	}

	// Ensure the tag exist in the packfile
	// If tag is unset, ban peer for sending a packfile that did not contain the queried object.
	if tag == nil {
		c.providerTracker.Ban(res.RemotePeer, 24*time.Hour)
		return nil, nil, fmt.Errorf("target tag not found in the packfile")
	}

	return res.Pack, tag.(*object.Tag), nil
}

// GetTaggedCommitWithAncestors gets the ancestors of the commit pointed by the given tag that
// do not exist in the local repository.
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
	check_end_tag:
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
			goto check_end_tag
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

// GetCommitWithAncestors gets a commit and also its ancestors that do not exist
// in the local repository. It will stop fetching ancestors when it finds
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
	msg := make([]byte, 128)
	_, err := s.Read(msg)
	if err != nil {
		return false, errors.Wrap(err, "failed to read request")
	}

	switch string(msg[:MsgTypeLen]) {

	// Handle 'want' message
	case dht2.MsgTypeWant:
		err := c.OnWantHandler(msg, s)
		return false, err

		// Handle 'send' message
	case dht2.MsgTypeSend:
		err := c.OnSendHandler(msg, s)
		return err == nil, err

	default:
		return false, ErrUnknownMsgType
	}
}

// OnWant handles incoming "WANT" requests
func (c *BasicObjectStreamer) OnWant(msg []byte, s network.Stream) error {

	repoName, key, err := dht2.ParseWantOrSendMsg(msg)
	if err != nil {
		s.Reset()
		c.log.Debug("failed parse 'want' message", "Err", err)
		return err
	}

	// Check if repo exist
	r, err := c.RepoGetter(c.gitBinPath, filepath.Join(c.reposDir, repoName))
	if err != nil {
		s.Reset()
		c.log.Debug("failed repository check", "Err", err)
		return err
	}

	commitHash, err := dht2.ParseObjectKey(key)
	if err != nil {
		s.Reset()
		c.log.Debug("unable to parse commit key", "Err", err)
		return err
	}

	// Check if object exist in the repo
	if !r.ObjectExist(plumbing.BytesToHex(commitHash)) {
		s.Reset()
		if _, err = s.Write(dht2.MakeNopeMsg()); err != nil {
			return errors.Wrap(err, "failed to write 'nope' message")
		}
		return dht2.ErrObjNotFound
	}

	// Respond with a 'have' message
	if _, err := s.Write(dht2.MakeHaveMsg()); err != nil {
		s.Reset()
		c.log.Error("failed to Write 'have' message", "Err", err)
		return err
	}

	return nil
}

// OnSend handles incoming "SEND" requests.
func (c *BasicObjectStreamer) OnSend(msg []byte, s network.Stream) error {

	// Parse the message
	repoName, key, err := dht2.ParseWantOrSendMsg(msg)
	if err != nil {
		s.Reset()
		c.log.Debug("failed parse 'want' message", "Err", err)
		return err
	}

	// Check if repo exist
	r, err := c.RepoGetter(c.gitBinPath, filepath.Join(c.reposDir, repoName))
	if err != nil {
		s.Reset()
		c.log.Debug("failed repository check", "Err", err)
		return err
	}

	// Parse the object key
	commitHash, err := dht2.ParseObjectKey(key)
	if err != nil {
		s.Reset()
		c.log.Debug("unable to parse commit key", "Err", err)
		return err
	}

	// Get the object
	obj, err := r.GetObject(plumbing.BytesToHex(commitHash))
	if err != nil {
		s.Reset()

		if err != plumb.ErrObjectNotFound {
			c.log.Error("failed local object check", "Err", err)
			return err
		}

		if _, err = s.Write(dht2.MakeNopeMsg()); err != nil {
			return errors.Wrap(err, "failed to write 'nope' message")
		}

		return err
	}

	peerHaveCache := c.HaveCache.GetCache(s.Conn().RemotePeer().Pretty())

	// Get the packfile representation of the object.
	// Filter out objects that we know the peer may have.
	pack, objs, err := c.PackObject(r, &plumbing.PackObjectArgs{
		Obj: obj,
		Filter: func(hash plumb.Hash) bool {
			return !peerHaveCache.Has(hash)
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
			peerHaveCache.Add(obj, struct{}{})
		}
	}

	return nil
}
