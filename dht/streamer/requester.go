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
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/dht/providertracker"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util/io"
)

var (
	RequesterDeadline       = 5 * time.Minute
	MaxPackSize       int64 = 100000000
)

var (
	ErrUnknownMsgType = fmt.Errorf("unknown message type")
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
	ProviderTracker *providertracker.BasicProviderTracker
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
	pTracker              *providertracker.BasicProviderTracker
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
		pTracker:  args.ProviderTracker,
	}

	r.OnWantResponseHandler = r.OnWantResponse
	r.OnSendResponseHandler = r.OnSendResponse

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

		// Send 'WANT' message to providers
		var s network.Stream
		s, err = r.Write(ctx, prov, ObjectStreamerProtocolID, dht.MakeWantMsg(r.repoName, r.key))
		if err != nil {
			wg.Done()
			r.log.Error("unable to write `WANT` message to peer", "ID", prov.ID.Pretty(), "Err", err)
			if r.pTracker != nil {
				r.pTracker.MarkFailure(prov.ID)
			}
			continue
		}

		// Handle 'WANT' response.
		// All providers that responded with a 'HAVE' message are
		// collected into the 'HAVE' stream channel
		go func() {
			r.OnWantResponseHandler(s)
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

			if r.pTracker != nil {
				r.pTracker.MarkFailure(str.Conn().RemotePeer())
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

			if r.pTracker != nil {
				r.pTracker.MarkFailure(str.Conn().RemotePeer())
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
// Streams that responded with 'HAVE' will be cached while others are reset.
func (r *BasicObjectRequester) OnWantResponse(s network.Stream) error {

	msg := make([]byte, 4)
	_, err := s.Read(msg)
	if err != nil {
		return errors.Wrap(err, "failed to read message type")
	}

	// Mark remote peer as seen.
	if r.pTracker != nil {
		r.pTracker.MarkSeen(s.Conn().RemotePeer())
	}

	switch string(msg[:4]) {
	case dht.MsgTypeHave:
		r.lck.Lock()
		r.providerStreams = append(r.providerStreams, s)
		r.lck.Unlock()

	case dht.MsgTypeNope:
		s.Reset()

	default:
		s.Reset()
	}

	return nil
}

// OnSendResponse handles incoming packfile data from remote peer.
// The remote peer may respond with "NOPE", it means they no longer
// have the requested object and an ErrObjNotFound is returned.
func (r *BasicObjectRequester) OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error) {
	defer s.Reset()

	var buf = bufio.NewReader(s)
	op, err := buf.Peek(4)
	if err != nil {
		if r.pTracker != nil {
			r.pTracker.MarkFailure(s.Conn().RemotePeer())
		}
		return nil, errors.Wrap(err, "unable to read msg type")
	}

	// Mark remote peer as seen.
	if r.pTracker != nil {
		r.pTracker.MarkSeen(s.Conn().RemotePeer())
	}

	switch string(op) {
	case dht.MsgTypeNope:
		return nil, dht.ErrObjNotFound

	case dht.MsgTypePack:
		rdr, err := io.LimitedReadToTmpFile(buf, MaxPackSize)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read pack data")
		}
		return rdr, nil

	default:
		return nil, ErrUnknownMsgType
	}
}
