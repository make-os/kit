package streamer

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/dht/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/util/io"
	"github.com/pkg/errors"
)

var (
	RequesterDeadline       = 5 * time.Minute
	MaxPackSize       int64 = 100000000
)

var (
	ErrUnknownMsgType = fmt.Errorf("unknown message type")
	ErrNopeReceived   = fmt.Errorf("nope received")
)

type PackResult struct {
	Pack       io.ReadSeekerCloser
	RemotePeer peer.ID
}

type ObjectRequester interface {
	Write(ctx context.Context, prov peer.AddrInfo, pid protocol.ID, data []byte) (network.Stream, error)
	WriteToStream(str network.Stream, data []byte) error
	DoWant(ctx context.Context) (err error)
	Do(ctx context.Context) (result *PackResult, err error)
	GetProviderStreams() []network.Stream
	OnWantResponse(s network.Stream) error
	OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error)
	AddProviderStream(streams ...network.Stream)
}

// MakeObjectRequester describes a function type for creating an object requester
type MakeObjectRequester func(args RequestArgs) ObjectRequester

// makeRequester creates a new object requester
func makeRequester(args RequestArgs) ObjectRequester {
	return NewBasicObjectRequester(args)
}

// RequestArgs contain arguments for NewBasicObjectRequester function.
type RequestArgs struct {

	// Host is the libp2p network host
	Host host.Host

	// Providers are addresses of providers
	Providers []peer.AddrInfo

	// ReposDir is the root directory for all repos
	ReposDir string

	// RepoName is the name of the repo to query object from
	RepoName string

	// Key is the requested object key
	Key []byte

	// Log is the app logger
	Log logger.Logger

	// BasicProviderTracker for recording and tracking provider behaviour
	ProviderTracker types.ProviderTracker
}

// BasicObjectRequester manages object download sessions between multiple providers
type BasicObjectRequester struct {
	lck                   *sync.Mutex
	providers             []peer.AddrInfo
	repoName              string
	key                   []byte
	host                  host.Host
	log                   logger.Logger
	reposDir              string
	closed                bool
	tracker               types.ProviderTracker
	providerStreams       []network.Stream
	OnWantResponseHandler func(network.Stream) error
	OnSendResponseHandler func(network.Stream) (io.ReadSeekerCloser, error)
}

// NewBasicObjectRequester creates an instance of BasicObjectRequester
func NewBasicObjectRequester(args RequestArgs) *BasicObjectRequester {
	r := BasicObjectRequester{
		lck:       &sync.Mutex{},
		providers: args.Providers,
		repoName:  args.RepoName,
		key:       args.Key,
		host:      args.Host,
		log:       args.Log,
		reposDir:  args.ReposDir,
		tracker:   args.ProviderTracker,
	}

	r.OnWantResponseHandler = r.OnWantResponse
	r.OnSendResponseHandler = r.OnSendResponse

	if r.log == nil {
		r.log = logger.NewLogrus(nil)
	}

	return &r
}

// AddProviderStream adds provider streams
func (r *BasicObjectRequester) AddProviderStream(streams ...network.Stream) {
	r.providerStreams = append(r.providerStreams, streams...)
}

// Write writes a message to a provider
func (r *BasicObjectRequester) Write(ctx context.Context, prov peer.AddrInfo, pid protocol.ID, data []byte) (network.Stream, error) {
	r.host.Peerstore().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
	str, err := r.host.NewStream(ctx, prov.ID, pid)
	if err != nil {
		return nil, err
	}

	str.SetDeadline(time.Now().Add(RequesterDeadline))

	_, err = str.Write(data)
	if err != nil {
		str.Reset()
		return nil, err
	}

	return str, nil
}

// WriteToStream writes a message to a stream
func (r *BasicObjectRequester) WriteToStream(str network.Stream, data []byte) error {
	_, err := str.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// DoWant sends 'WANT' messages to providers, then caches the
// stream of providers that responded with 'HAVE' message.
func (r *BasicObjectRequester) DoWant(ctx context.Context) (err error) {
	var wg sync.WaitGroup
	wg.Add(len(r.providers))

	for _, prov := range r.providers {
		if len(prov.Addrs) == 0 {
			wg.Done()
			continue
		}

		// Send 'WANT' message to provider
		var s network.Stream
		s, err = r.Write(ctx, prov, ObjectStreamerProtocolID, dht.MakeWantMsg(r.repoName, r.key))
		if err != nil {
			wg.Done()
			r.log.Error("Unable to write `WANT` message to peer", "Peer", prov.ID.Pretty(), "Err", err)
			if r.tracker != nil {
				r.tracker.MarkFailure(prov.ID)
			}
			continue
		}

		commitHash, _ := dht.ParseObjectKey(r.key)
		r.log.Debug("WANT->: Sent request for an object",
			"Repo", r.repoName, "Hash", commitHash, "Peer", prov.ID.Pretty())

		// Handle 'WANT' response.
		go func() {
			err = r.OnWantResponseHandler(s)
			wg.Done()
		}()
	}

	wg.Wait()
	return
}

// Do starts the object request protocol
func (r *BasicObjectRequester) Do(ctx context.Context) (result *PackResult, err error) {

	// Send `WANT` message to providers
	err = r.DoWant(ctx)

	// Return error if no provider stream
	if len(r.providerStreams) == 0 {
		return nil, fmt.Errorf("no provider stream")
	}

	// Process streams that have the requested object. Synchronously send 'SEND'
	// message to each stream and stop when we receive a packfile the
	// first stream. Once done, simply reset the unused provider streams.
	for _, str := range r.providerStreams {

		if err = ctx.Err(); err != nil {
			str.Reset()
			return nil, err
		}

		// Send a 'SEND' message to the stream.
		if err = r.WriteToStream(str, dht.MakeSendMsg(r.repoName, r.key)); err != nil {
			str.Reset()
			r.log.Error("failed to write 'SEND' message to peer", "Err", err,
				"Peer", str.Conn().RemotePeer().Pretty())

			if r.tracker != nil {
				r.tracker.MarkFailure(str.Conn().RemotePeer())
			}
			continue
		}

		// Handle 'SEND' response.
		var packfile io.ReadSeekerCloser
		packfile, err = r.OnSendResponseHandler(str)
		if err != nil {
			str.Reset()
			r.log.Error("failed to read 'SEND' response", "Err", err,
				"Peer", str.Conn().RemotePeer().Pretty())

			if r.tracker != nil {
				r.tracker.MarkFailure(str.Conn().RemotePeer())
			}
			continue
		}

		return &PackResult{
			Pack:       packfile,
			RemotePeer: str.Conn().RemotePeer(),
		}, nil
	}

	return nil, err
}

// GetProviderStreams returns the provider's streams
func (r *BasicObjectRequester) GetProviderStreams() []network.Stream {
	return r.providerStreams
}

// OnWantResponse handles a remote peer's response to a WANT message.
// If the remote stream responds with 'HAVE', it will be cached.
// If the remote stream responds with 'NOPE', it will be logged in the nope cache.
func (r *BasicObjectRequester) OnWantResponse(s network.Stream) error {

	msg := make([]byte, 4)
	_, err := s.Read(msg)
	if err != nil {
		return errors.Wrap(err, "failed to read message type")
	}

	// Mark remote peer as seen.
	remotePeer := s.Conn().RemotePeer()
	if r.tracker != nil {
		r.tracker.MarkSeen(remotePeer)
	}

	hash, _ := dht.ParseObjectKeyToHex(r.key)

	switch string(msg[:4]) {
	case dht.MsgTypeHave:
		r.log.Debug("HAVE<-: Received message from provider",
			"Repo", r.repoName, "Hash", hash, "Peer", remotePeer.Pretty())
		r.lck.Lock()
		r.providerStreams = append(r.providerStreams, s)
		r.lck.Unlock()

	case dht.MsgTypeNope:
		r.log.Debug("NOPE<-: Provider no longer has the object", "Hash", hash)
		s.Reset()
		r.tracker.PeerSentNope(remotePeer, r.key)
		return ErrNopeReceived

	default:
		s.Reset()
	}

	return nil
}

// OnSendResponse handles incoming packfile data from remote peer.
// If the remote peer responds with 'NOPE', it will be logged in the nope cache.
func (r *BasicObjectRequester) OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error) {
	defer s.Reset()

	remotePeer := s.Conn().RemotePeer()

	var buf = bufio.NewReader(s)
	op, err := buf.Peek(4)
	if err != nil {
		if r.tracker != nil {
			r.tracker.MarkFailure(remotePeer)
		}
		return nil, errors.Wrap(err, "unable to read msg type")
	}

	// Mark remote peer as seen.
	if r.tracker != nil {
		r.tracker.MarkSeen(remotePeer)
	}

	hash, _ := dht.ParseObjectKeyToHex(r.key)

	switch string(op) {
	case dht.MsgTypeNope:
		r.log.Debug("NOPE<-: Expected packfile but provider refused to send",
			"Repo", r.repoName, "Hash", hash, "Peer", remotePeer.Pretty())
		r.tracker.PeerSentNope(remotePeer, r.key)
		return nil, dht.ErrObjNotFound

	case dht.MsgTypePack:
		r.log.Debug("PACK<-: Packfile received from provider",
			"Repo", r.repoName, "Hash", hash, "Peer", remotePeer.Pretty())
		rdr, err := io.LimitedReadToTmpFile(buf, MaxPackSize)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read pack data")
		}
		return rdr, nil

	default:
		return nil, ErrUnknownMsgType
	}
}
