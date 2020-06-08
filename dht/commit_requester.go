package dht

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

type CommitRequester interface {
	Write(ctx context.Context, prov peer.AddrInfo, pid protocol.ID, data []byte) (network.Stream, error)
	WriteToStream(str network.Stream, data []byte) error
	DoWant(ctx context.Context) (err error)
	Do(ctx context.Context) (packfile io.ReadSeekerCloser, err error)
	GetProviderStreams() []network.Stream
	OnWantResponse(s network.Stream) error
	OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error)
}

// CommitRequesterArgs contain arguments for NewCommitRequester function.
type CommitRequesterArgs struct {

	// Host is the libp2p network host
	Host host.Host

	// Providers are addresses of providers
	Providers []peer.AddrInfo

	// ProvidersStream sets the initial provider's streams
	ProviderStreams []network.Stream

	// ReposDir is the root directory for all repos
	ReposDir string

	// RepoName is the name of the repo to query commit from
	RepoName string

	// RequestKey is the requested commit key
	RequestKey []byte

	// Log is the app logger
	Log logger.Logger
}

// BasicCommitRequester manages object download sessions between multiple providers
type BasicCommitRequester struct {
	lck                   *sync.Mutex
	providers             []peer.AddrInfo
	repoName              string
	key                   []byte
	host                  host.Host
	log                   logger.Logger
	reposDir              string
	closed                bool
	providerStreams       []network.Stream
	OnWantResponseHandler func(network.Stream) error
	OnSendResponseHandler func(network.Stream) (io.ReadSeekerCloser, error)
}

// NewCommitRequester creates an instance of BasicCommitRequester
func NewCommitRequester(args CommitRequesterArgs) *BasicCommitRequester {
	r := BasicCommitRequester{
		lck:             &sync.Mutex{},
		providers:       args.Providers,
		repoName:        args.RepoName,
		key:             args.RequestKey,
		host:            args.Host,
		log:             args.Log,
		reposDir:        args.ReposDir,
		providerStreams: args.ProviderStreams,
	}
	r.OnWantResponseHandler = r.OnWantResponse
	r.OnSendResponseHandler = r.OnSendResponse
	return &r
}

// Write writes a message to a provider
func (r *BasicCommitRequester) Write(ctx context.Context, prov peer.AddrInfo, pid protocol.ID, data []byte) (network.Stream, error) {
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
func (r *BasicCommitRequester) WriteToStream(str network.Stream, data []byte) error {
	_, err := str.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// DoWant sends 'WANT' messages to providers, then caches the
// stream of providers that responded with 'HAVE' message.
func (r *BasicCommitRequester) DoWant(ctx context.Context) (err error) {
	var wg sync.WaitGroup
	wg.Add(len(r.providers))

	for _, prov := range r.providers {
		if len(prov.Addrs) == 0 {
			wg.Done()
			continue
		}

		// Send 'WANT' message to providers
		var s network.Stream
		s, err = r.Write(ctx, prov, CommitStreamProtocolID, MakeWantMsg(r.repoName, r.key))
		if err != nil {
			wg.Done()
			r.log.Error("unable to Write `WANT` message to peer", "ID", prov.ID.Pretty())
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

// Do starts the commit object request protocol
func (r *BasicCommitRequester) Do(ctx context.Context) (packfile io.ReadSeekerCloser, err error) {

	// Send `WANT` message to providers
	r.DoWant(ctx)

	// Process streams that have the commit object. Synchronously send 'SEND'
	// message to each stream and stop when we receive a packfile the
	// first stream. Once done, simply reset the unused provider streams.
	// TODO: We should use an algorithm that tries to prioritizes highly available providers
	//  and prevents the requester from overwhelming them.
	for _, str := range r.providerStreams {

		if err = ctx.Err(); err != nil {
			str.Reset()
			return nil, err
		}

		// Send a 'SEND' message to the stream.
		if err = r.WriteToStream(str, MakeSendMsg(r.repoName, r.key)); err != nil {
			str.Reset()
			r.log.Error("failed to Write 'SEND' message to peer", "Err", err,
				"Peer", str.Conn().RemotePeer().Pretty())
			continue
		}

		// Handle 'SEND' response.
		packfile, err = r.OnSendResponseHandler(str)
		if err != nil {
			str.Reset()
			r.log.Error("failed to read 'SEND' response", "Err", err,
				"Peer", str.Conn().RemotePeer().Pretty())
			continue
		}

		return packfile, nil
	}

	return nil, err
}

// GetProviderStreams returns the provider's streams
func (r *BasicCommitRequester) GetProviderStreams() []network.Stream {
	return r.providerStreams
}

// OnWantResponse handles a remote peer's response to a WANT message.
// Streams that responded with 'HAVE' will be cached while others are reset.
func (r *BasicCommitRequester) OnWantResponse(s network.Stream) error {

	msg := make([]byte, 4)
	_, err := s.Read(msg)
	if err != nil {
		return errors.Wrap(err, "failed to read message type")
	}

	switch string(msg[:4]) {
	case MsgTypeHave:
		r.lck.Lock()
		r.providerStreams = append(r.providerStreams, s)
		r.lck.Unlock()

	case MsgTypeNope:
		s.Reset()

	default:
		s.Reset()
	}

	return nil
}

// OnSendResponse handles incoming packfile data from remote peer.
// The remote peer may respond with "NOPE", it means they no longer
// have the requested commit and an ErrObjNotFound is returned.
func (r *BasicCommitRequester) OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error) {
	defer s.Reset()

	var buf = bufio.NewReader(s)
	op, err := buf.Peek(4)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read msg type")
	}

	switch string(op) {
	case MsgTypeNope:
		return nil, ErrObjNotFound

	case MsgTypePack:
		rdr, err := io.LimitedReadToTmpFile(buf, MaxPackSize)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read pack data")
		}
		return rdr, nil

	default:
		return nil, ErrUnknownMsgType
	}
}
