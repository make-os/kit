package dht

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	types2 "gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util/io"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	ErrNoProviderFound = fmt.Errorf("no provider found")
)

var (
	CommitStreamProtocolID = protocol.ID("/commit/1.0")
	CommitKeyID            = "/c"
	MsgTypeLen             = 4
)

// CommitStreamer provides an interface for announcing/fetching
// commits to/from the underlying DHT network.
type CommitStreamer interface {

	// Announce announces a commit hash
	Announce(ctx context.Context, hash []byte) error

	// Get gets a packfile containing a commit, its tree and the containing blobs
	Get(ctx context.Context, repoName string, hash []byte) (packfile io.ReadSeekerCloser,
		commit *object.Commit, err error)

	// GetAncestors like Get, gets a commit and also its ancestors that do not exist
	// in the local repository. It will stop fetching ancestors when it finds
	// an ancestor matching the given end hash.
	GetAncestors(ctx context.Context, args GetAncestorArgs) (packfiles []io.ReadSeekerCloser,
		err error)

	// Validate validates a commit's packfile.
	// hash is the commit hash
	Validate(hash []byte, packfile io.ReadSeekerCloser) (targetCommit *object.Commit, err error)

	// OnRequest handles incoming commit object requests
	OnRequest(s network.Stream) error
}

// BasicCommitStreamer implements CommitStreamer. It provides a mechanism for
// announcing/fetching commits to/from the DHT.
type BasicCommitStreamer struct {
	dht              DHT
	log              logger.Logger
	reposDir         string
	gitBinPath       string
	OnWantHandler    func(msg []byte, s network.Stream) error
	OnSendHandler    func(msg []byte, s network.Stream) error
	RepoGetter       repo.LocalRepoGetter
	PackCommit       plumbing.CommitPacker
	MakeRequester    MakeCommitRequester
	ValidatePackfile PackfileValidator
	Unpack           plumbing.PackfileUnpacker
}

// NewCommitStreamer creates an instance of BasicCommitStreamer
func NewCommitStreamer(dht DHT, cfg *config.AppConfig) *BasicCommitStreamer {
	ce := &BasicCommitStreamer{
		dht:        dht,
		reposDir:   cfg.GetRepoRoot(),
		log:        cfg.G().Log.Module("commit-streamer"),
		gitBinPath: cfg.Node.GitBinPath,
		RepoGetter: repo.GetWithLiteGit,
		PackCommit: plumbing.PackCommitObject,
		Unpack:     plumbing.UnPackfile,
	}

	ce.OnWantHandler = ce.OnWant
	ce.OnSendHandler = ce.OnSend
	ce.MakeRequester = ce.makeRequester
	ce.ValidatePackfile = ce.Validate

	dht.Host().SetStreamHandler(CommitStreamProtocolID, func(s network.Stream) {
		for {
			ce.OnRequest(s)
		}
	})
	return ce
}

// Announce announces a commit hash
func (c *BasicCommitStreamer) Announce(ctx context.Context, hash []byte) error {
	return c.dht.Announce(ctx, MakeCommitKey(hash))
}

// MakeCommitRequester describes a function type for create commit requester object
type MakeCommitRequester func(args CommitRequesterArgs) CommitRequester

func (c *BasicCommitStreamer) makeRequester(args CommitRequesterArgs) CommitRequester {
	return NewCommitRequester(args)
}

// Get gets a single commit by hash.
// It returns the packfile, the commit object and error.
func (c *BasicCommitStreamer) Get(
	ctx context.Context,
	repoName string,
	hash []byte) (io.ReadSeekerCloser, *object.Commit, error) {

	// Find providers of the commit hash
	key := MakeCommitKey(hash)
	providers, err := c.dht.GetProviders(ctx, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get providers")
	}

	if len(providers) == 0 {
		return nil, nil, ErrNoProviderFound
	}

	// Start request session
	req := c.MakeRequester(CommitRequesterArgs{
		Providers:  providers,
		RepoName:   repoName,
		RequestKey: key,
		Host:       c.dht.Host(),
		Log:        c.log,
		ReposDir:   c.reposDir,
	})

	// Do the request
	pack, err := req.Do(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "request failed")
	}

	// Validate the pack
	commit, err := c.ValidatePackfile(hash, pack)
	if err != nil {
		return nil, nil, errors.Wrap(err, "validation failed")
	}

	return pack, commit, nil
}

// GetAncestorArgs contain arguments for GetAncestors method
type GetAncestorArgs struct {

	// RepoName is the target repository to query commits from.
	RepoName string

	// StartCommitHash is the hash of the commit to start from
	StartCommitHash []byte

	// EndCommitHash is the hash of the commit that indicates the end of the query.
	// If provided, it must exist on the local repository of the caller.
	EndCommitHash []byte

	// ExcludeEndCommit when true, indicates that the end commit should not be fetched.
	ExcludeEndCommit bool

	// GitBinPath is the path to the git binary
	GitBinPath string

	// ReposDir is the root directory containing all repositories
	ReposDir string

	// ResultCB is a callback used for collecting packfiles as they are fetched.
	// If not set, all packfile results a collected and return at the end of the query.
	ResultCB func(packfile io.ReadSeekerCloser) error
}

// GetAncestors gets a commit and also its ancestors that do not exist
// in the local repository. It will stop fetching ancestors when it finds
// an ancestor matching the given end hash.
// If EndCommitHash is true, tt is expected that EndCommitHash commit must exist locally.
// Packfiles returned are expected to be closed by the caller.
// If ResultCB is set, packfiles will be passed to the callback and not returned.
// If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
// with a nil error.
func GetAncestors(
	ctx context.Context,
	c CommitStreamer,
	repoGetter repo.LocalRepoGetter,
	args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {

	// Get the target repo
	var r types.LocalRepo
	r, err = repoGetter(args.GitBinPath, filepath.Join(args.ReposDir, args.RepoName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get repo")
	}

	// If end commit hash is specified, ensure it exists locally.
	endCommitHash := string(args.EndCommitHash)
	if endCommitHash != "" {
		if !r.ObjectExist(endCommitHash) {
			return nil, fmt.Errorf("end commit must already exist in the local repo")
		}
	}

	// Maintain a wantlist containing commits to fetch, starting with the start commit.
	var wantList = [][]byte{args.StartCommitHash}
	var fetched = map[string]struct{}{}
	var endCommitSeen bool
	for len(wantList) > 0 {

		// Get a hash
		target := wantList[0]
		wantList = wantList[1:]

		// Skip if already fetched
		targetHash := string(target)
		if _, ok := fetched[targetHash]; ok {
			continue
		}

		// Attempt to get the packfile of the commit.
		// Cache successfully fetched commits
		pack, fetchedCommit, err := c.Get(ctx, args.RepoName, target)
		if err != nil {
			return packfiles, err
		}
		fetched[targetHash] = struct{}{}

		// Collect packfile if the fetched commit is not the end commit and ExcludeEndCommit is false
		fetchedHash := fetchedCommit.ID().String()
		if fetchedHash != endCommitHash || fetchedHash == endCommitHash && !args.ExcludeEndCommit {

			// Pass packfile to callback if provided, otherwise, use the regular packfiles result slice
			// Return immediately if the callback returns an error.
			// If the callback returns an error, return nil if err is an ErrExit.
			if args.ResultCB != nil {
				if err := args.ResultCB(pack); err != nil {
					if err == types2.ErrExit {
						return packfiles, nil
					}
					return packfiles, err
				}
			} else {
				packfiles = append(packfiles, pack)
			}
		}

		// Skip this iteration if fetched commit is the end commit
		if fetchedHash == endCommitHash {
			continue
		}

		// At this point, the fetched commit is not the end commit,
		// we need to add parents that are currently unknown and un-fetched into the wantlist.
		// If end commit hash is set,
		// - Skip parent if it matches the end commit hash but ExcludeEndCommit is true.
		// - Add parent if it matches the end commit hash but ExcludeEndCommit is false.
		// - Skip parent if it is an ancestor of the already seen/fetched end commit hash.
		// - Add a parent to the wantlist if it is not an ancestor or does not exist locally.
		_, endCommitFetched := fetched[endCommitHash]
		for _, parent := range fetchedCommit.ParentHashes {
			parentIsEndCommit := parent.String() == endCommitHash

			// Skip parent if it already exist and not the end commit (if set).
			// But if the parent is the end commit, we will can only skip it if ExcludeEndCommit is true.
			if r.ObjectExist(parent.String()) && (!parentIsEndCommit || args.ExcludeEndCommit) {
				continue
			}

			// Add the parent if there is no end commit hash.
			if endCommitHash == "" {
				goto add
			}

			// If this parent is the end commit, set endCommitSeen flag to true
			// and skip it only if ExcludeEndCommit is true.
			if parentIsEndCommit {
				endCommitSeen = true
				if args.ExcludeEndCommit {
					continue
				}
			}

			// At this point, if the parent is not the end commit
			if !parentIsEndCommit && (endCommitFetched || endCommitSeen) {

				// If this parent is an ancestor of the end commit, it means
				// we already have the parent since it is already part of the
				// end commit history, as such, we skip t
				err := r.IsAncestor(parent.String(), endCommitHash)

				// However, if we do not have the parent commit locally
				// or it is not an ancestor of the end commit, it is okay
				// to add it to the wantlist.
				if err == plumbing2.ErrObjectNotFound || err == repo.ErrNotAnAncestor {
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
				wantList = append(wantList, []byte(parent.String()))
			}
		}
	}

	return
}

// GetAncestors gets a commit and also its ancestors that do not exist
// in the local repository. It will stop fetching ancestors when it finds
// an ancestor matching the given end hash.
func (c *BasicCommitStreamer) GetAncestors(ctx context.Context,
	args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error) {
	args.ReposDir, args.GitBinPath = c.reposDir, c.gitBinPath
	return GetAncestors(ctx, c, c.RepoGetter, args)
}

// PackfileValidator describes a function for validating a commit's packfile
type PackfileValidator func(hash []byte, pack io.ReadSeekerCloser) (targetCommit *object.Commit, err error)

// Validate performs sanity validations on the given commit packfile and returns the
// commit object.
//
// Basic rules are:
// - The packfile must contain the target commit.
// - The packfile must contain the target commit tree.
// - The packfile must contain the tree entries of the commit.
func (c *BasicCommitStreamer) Validate(hash []byte, pack io.ReadSeekerCloser) (target *object.Commit, err error) {

	// Find the target commit and headers of other objects.
	var hdrs = make(map[string]*packfile.ObjectHeader)
	var targetCommitTree *object.Tree
	c.Unpack(pack, func(h *packfile.ObjectHeader, read func() (object.Object, error)) error {

		obj, err := read()
		if err != nil {
			return errors.Wrap(err, "failed to read object")
		}

		// Find the target commit object.
		if h.Type == plumbing2.CommitObject && obj.ID().String() == string(hash) {
			target = obj.(*object.Commit)
			return nil
		}

		// Find the target commit tree object
		if h.Type == plumbing2.TreeObject && target != nil && obj.ID().String() == target.TreeHash.String() {
			targetCommitTree = obj.(*object.Tree)
		}

		// Cache the headers of other objects
		h.Reference = obj.ID()
		hdrs[obj.ID().String()] = h
		return nil
	})

	// We expect the packfile to contain the target commit.
	if target == nil {
		return nil, fmt.Errorf("target commit was not found in packfile")
	}

	// We expect the packfile to contain the tree of the commit
	if targetCommitTree == nil {
		return nil, fmt.Errorf("target commit tree was not found in packfile")
	}

	// We expect the packfile to contain every entries of the commit tree
	for _, entry := range targetCommitTree.Entries {
		if _, ok := hdrs[entry.Hash.String()]; !ok {
			return nil, fmt.Errorf("target commit tree entry (%s) was not found in packfile", entry.Name)
		}
	}

	return target, nil
}

// OnRequest handles incoming commit object requests
func (c *BasicCommitStreamer) OnRequest(s network.Stream) error {

	// Get request message
	msg := make([]byte, 128)
	_, err := s.Read(msg)
	if err != nil {
		return errors.Wrap(err, "failed to read request")
	}

	switch string(msg[:MsgTypeLen]) {

	// Handle 'want' message
	case MsgTypeWant:
		return c.OnWantHandler(msg, s)

		// Handle 'send' message
	case MsgTypeSend:
		return c.OnSendHandler(msg, s)

	default:
		return ErrUnknownMsgType
	}
}

// OnWant handles incoming "WANT" requests
func (c *BasicCommitStreamer) OnWant(msg []byte, s network.Stream) error {

	repoName, key, err := parseWantOrSendMsg(msg)
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

	commitHash, err := parseCommitKey(key)
	if err != nil {
		s.Reset()
		c.log.Debug("unable to parse commit key", "Err", err)
		return err
	}

	// Check if object exist in the repo
	_, err = r.CommitObject(plumbing2.NewHash(string(commitHash)))
	if err != nil {
		if err != plumbing2.ErrObjectNotFound {
			s.Reset()
			c.log.Error("failed local object check", "Err", err)
			return err
		}

		if _, err = s.Write(MakeNopeMsg()); err != nil {
			s.Reset()
			return errors.Wrap(err, "failed to write 'nope' message")
		}
		return nil
	}

	// Respond with a 'have' message
	if _, err := s.Write(MakeHaveMsg()); err != nil {
		s.Reset()
		c.log.Error("failed to Write 'have' message", "Err", err)
		return err
	}

	return nil
}

// OnSend handles incoming "SEND" requests.
func (c *BasicCommitStreamer) OnSend(msg []byte, s network.Stream) error {

	repoName, key, err := parseWantOrSendMsg(msg)
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

	commitHash, err := parseCommitKey(key)
	if err != nil {
		s.Reset()
		c.log.Debug("unable to parse commit key", "Err", err)
		return err
	}

	// Check if object exist in repo
	commit, err := r.CommitObject(plumbing2.NewHash(string(commitHash)))
	if err != nil {
		if err != plumbing2.ErrObjectNotFound {
			s.Reset()
			c.log.Error("failed local object check", "Err", err)
			return err
		}

		if _, err = s.Write(MakeNopeMsg()); err != nil {
			s.Reset()
			return errors.Wrap(err, "failed to write 'nope' message")
		}
	}

	// Get the commit and its contained object into a packfile
	commitPack, err := c.PackCommit(r, commit)
	if err != nil {
		s.Reset()
		return errors.Wrap(err, "failed to generate commit packfile")
	}

	// Write the packfile to the requester
	w := bufio.NewWriter(bufio.NewWriter(s))
	if _, err := w.ReadFrom(commitPack); err != nil {
		c.log.Error("failed to Write commit pack", "Err", err)
		return errors.Wrap(err, "Write commit pack error")
	}
	w.Flush()
	s.Close()

	return nil
}
