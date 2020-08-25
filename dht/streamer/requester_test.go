package streamer_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/dht/providertracker"
	"github.com/make-os/lobe/dht/streamer"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	io2 "github.com/make-os/lobe/util/io"
	"github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BasicObjectRequester", func() {
	var err error
	var cfg *config.AppConfig
	var log logger.Logger
	var ctrl *gomock.Controller
	var mockHost *mocks.MockHost

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		log = cfg.G().Log
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockHost = mocks.NewMockHost(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Write", func() {
		It("should return error when unable to create new stream", func() {
			ctx := context.Background()
			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}
			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockHost.EXPECT().NewStream(ctx, prov.ID, core.ProtocolID("")).Return(nil, fmt.Errorf("error"))
			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost})
			_, err := r.Write(context.Background(), prov, "", nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to write to new stream", func() {
			ctx := context.Background()
			data := []byte("xyz")
			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}

			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().SetDeadline(gomock.Any())
			mockStream.EXPECT().Write(data).Return(0, fmt.Errorf("write error"))
			mockStream.EXPECT().Reset()
			mockHost.EXPECT().NewStream(ctx, prov.ID, core.ProtocolID("")).Return(mockStream, nil)

			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost})
			_, err := r.Write(context.Background(), prov, "", data)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("write error"))
		})

		It("should return stream and nil when successful", func() {
			ctx := context.Background()
			data := []byte("xyz")
			prov := peer.AddrInfo{ID: "id", Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}

			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().SetDeadline(gomock.Any())
			mockStream.EXPECT().Write(data).Return(0, nil)
			mockHost.EXPECT().NewStream(ctx, prov.ID, core.ProtocolID("")).Return(mockStream, nil)

			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost})
			stream, err := r.Write(context.Background(), prov, "", data)
			Expect(err).To(BeNil())
			Expect(mockStream).To(Equal(stream))
		})
	})

	Describe(".WriteToStream", func() {
		It("should return error when unable to write to specified stream", func() {
			data := []byte("xyz")

			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(data).Return(0, fmt.Errorf("write error"))

			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost})
			err := r.WriteToStream(mockStream, data)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("write error"))
		})
	})

	Describe(".DoWant", func() {
		It("should return no error when there are no providers", func() {
			ctx := context.Background()
			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost})
			err := r.DoWant(ctx)
			Expect(err).To(BeNil())
		})

		It("should return no error and skip provider if it has no address info", func() {
			ctx := context.Background()
			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost, Providers: []peer.AddrInfo{{
				Addrs: []multiaddr.Multiaddr{},
			}}})
			err := r.DoWant(ctx)
			Expect(err).To(BeNil())
		})

		It("should return error when unable to write 'WANT' message to provider", func() {
			ctx := context.Background()
			prov := peer.AddrInfo{Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}

			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockHost.EXPECT().NewStream(ctx, prov.ID, streamer.ObjectStreamerProtocolID).Return(nil, fmt.Errorf("error"))

			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{Host: mockHost, Providers: []peer.AddrInfo{prov}, Log: log})
			err := r.DoWant(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error"))
		})

		It("should return error when 'WANT' message is sent, 'WANT' response handler must be called", func() {
			ctx := context.Background()
			repoName := "repo1"
			key := []byte("object_key")
			prov := peer.AddrInfo{Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}

			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().SetDeadline(gomock.Any())
			mockStream.EXPECT().Write(dht.MakeWantMsg(repoName, key)).Return(0, nil)
			mockHost.EXPECT().NewStream(ctx, prov.ID, streamer.ObjectStreamerProtocolID).Return(mockStream, nil)

			r := streamer.NewBasicObjectRequester(streamer.RequestArgs{
				Host:      mockHost,
				Providers: []peer.AddrInfo{prov},
				RepoName:  repoName,
				Key:       key,
				Log:       log,
			})
			called := false
			r.OnWantResponseHandler = func(stream network.Stream) error {
				called = true
				return nil
			}

			err := r.DoWant(ctx)
			Expect(err).To(BeNil())
			Expect(called).To(BeTrue())
		})
	})

	Describe(".Do", func() {
		When("there is a provider stream", func() {
			var reqArgs streamer.RequestArgs
			var mockStream *mocks.MockStream
			var repoName = "repo1"
			var key = []byte("object_key")

			BeforeEach(func() {
				mockStream = mocks.NewMockStream(ctrl)
				reqArgs = streamer.RequestArgs{
					Host:     mockHost,
					RepoName: repoName,
					Key:      key,
					Log:      log,
				}
			})

			It("should return error when context has expired or is cancelled", func() {
				ctx, cn := context.WithCancel(context.Background())
				cn()
				mockStream.EXPECT().Reset()
				r := streamer.NewBasicObjectRequester(reqArgs)
				r.AddProviderStream(mockStream)
				_, err := r.Do(ctx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(context.Canceled))
			})

			It("should return error when attempt to write 'SEND' message to stream failed", func() {
				ctx := context.Background()
				mockStream.EXPECT().Write(dht.MakeSendMsg(repoName, key)).Return(0, fmt.Errorf("error"))
				mockStream.EXPECT().Reset()
				mockConn := mocks.NewMockConn(ctrl)
				mockConn.EXPECT().RemotePeer().Return(core.PeerID("peer_id"))
				mockStream.EXPECT().Conn().Return(mockConn)
				r := streamer.NewBasicObjectRequester(reqArgs)
				r.AddProviderStream(mockStream)
				_, err := r.Do(ctx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when 'SEND' message response handler failed", func() {
				ctx := context.Background()
				mockStream.EXPECT().Write(dht.MakeSendMsg(repoName, key)).Return(0, nil)
				mockStream.EXPECT().Reset()
				mockConn := mocks.NewMockConn(ctrl)
				mockConn.EXPECT().RemotePeer().Return(core.PeerID("peer_id"))
				mockStream.EXPECT().Conn().Return(mockConn)
				r := streamer.NewBasicObjectRequester(reqArgs)
				r.AddProviderStream(mockStream)
				r.OnSendResponseHandler = func(network.Stream) (io2.ReadSeekerCloser, error) {
					return nil, fmt.Errorf("bad error")
				}
				_, err := r.Do(ctx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad error"))
			})

			It("should return packfile when 'SEND' message response handler succeeds", func() {
				ctx := context.Background()
				mockStream.EXPECT().Write(dht.MakeSendMsg(repoName, key)).Return(0, nil)
				mockConn := mocks.NewMockConn(ctrl)
				remotePeerID := core.PeerID("peer_id")
				mockConn.EXPECT().RemotePeer().Return(remotePeerID)
				mockStream.EXPECT().Conn().Return(mockConn)
				r := streamer.NewBasicObjectRequester(reqArgs)
				r.AddProviderStream(mockStream)

				tmpFile, _ := ioutil.TempFile(os.TempDir(), "")
				defer tmpFile.Close()
				r.OnSendResponseHandler = func(network.Stream) (io2.ReadSeekerCloser, error) {
					return tmpFile, nil
				}

				result, err := r.Do(ctx)
				Expect(err).To(BeNil())
				Expect(result.Pack).NotTo(BeNil())
				Expect(result.Pack).To(Equal(tmpFile))
				Expect(result.RemotePeer).To(Equal(remotePeerID))
			})
		})
	})

	Describe(".OnWantResponse", func() {
		var mockStream *mocks.MockStream
		var reqArgs streamer.RequestArgs

		BeforeEach(func() {
			mockStream = mocks.NewMockStream(ctrl)
			reqArgs = streamer.RequestArgs{
				Key:             util.RandBytes(5),
				ProviderTracker: providertracker.NewProviderTracker(),
			}
		})

		It("should return error when unable to read message type from stream", func() {
			mockStream.EXPECT().Read(make([]byte, 4)).Return(0, fmt.Errorf("read error"))
			r := streamer.NewBasicObjectRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read message type: read error"))
		})

		It("should add stream to provider's stream cache, when message type is 'HAVE'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypeHave)
				return len(dht.MsgTypeHave), nil
			})
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(core.PeerID("peer_id"))
			mockStream.EXPECT().Conn().Return(mockConn)
			r := streamer.NewBasicObjectRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(r.GetProviderStreams()).To(HaveLen(1))
			Expect(r.GetProviderStreams()[0]).To(Equal(mockStream))
		})

		When("message type is 'NOPE'", func() {
			remotePeer := core.PeerID("peer_id")

			BeforeEach(func() {
				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, dht.MsgTypeNope)
					return len(dht.MsgTypeNope), nil
				})
				mockConn := mocks.NewMockConn(ctrl)
				mockConn.EXPECT().RemotePeer().Return(remotePeer)
				mockStream.EXPECT().Conn().Return(mockConn)
			})

			It("should reset stream", func() {
				mockStream.EXPECT().Reset()
				r := streamer.NewBasicObjectRequester(reqArgs)
				r.OnWantResponse(mockStream)
			})

			It("should return err=ErrNopeReceived", func() {
				mockStream.EXPECT().Reset()
				r := streamer.NewBasicObjectRequester(reqArgs)
				err := r.OnWantResponse(mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(streamer.ErrNopeReceived))
				Expect(r.GetProviderStreams()).To(HaveLen(0))
			})

			Specify("that there are no providers", func() {
				mockStream.EXPECT().Reset()
				r := streamer.NewBasicObjectRequester(reqArgs)
				err := r.OnWantResponse(mockStream)
				Expect(err).ToNot(BeNil())
				Expect(r.GetProviderStreams()).To(HaveLen(0))
			})

			Specify("that the peer was added to the NOPE cache for the given key", func() {
				mockStream.EXPECT().Reset()
				r := streamer.NewBasicObjectRequester(reqArgs)
				err := r.OnWantResponse(mockStream)
				Expect(err).ToNot(BeNil())
				Expect(reqArgs.ProviderTracker.DidPeerSendNope(remotePeer, reqArgs.Key)).To(BeTrue())
			})
		})

		It("should reset stream, when message type is 'UNKNOWN'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, "UNKNOWN")
				return len("UNKNOWN"), nil
			})
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(core.PeerID("peer_id"))
			mockStream.EXPECT().Conn().Return(mockConn)
			mockStream.EXPECT().Reset()
			r := streamer.NewBasicObjectRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(r.GetProviderStreams()).To(HaveLen(0))
		})
	})

	Describe(".OnSendResponse", func() {
		var mockStream *mocks.MockStream
		var reqArgs streamer.RequestArgs
		var remotePeer = core.PeerID("peer_id")

		BeforeEach(func() {
			mockStream = mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(remotePeer)
			mockStream.EXPECT().Conn().Return(mockConn)
			reqArgs = streamer.RequestArgs{
				Key:             util.RandBytes(5),
				ProviderTracker: providertracker.NewProviderTracker(),
			}
		})

		It("should return error when unable to read message type from stream", func() {
			mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("read error"))
			r := streamer.NewBasicObjectRequester(reqArgs)
			_, err := r.OnSendResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unable to read msg type: read error"))
		})

		When("msg type is 'NOPE'", func() {
			BeforeEach(func() {
				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, dht.MsgTypeNope)
					return len(dht.MsgTypeNope), nil
				})
			})

			It("should return ErrObjNotFound", func() {
				r := streamer.NewBasicObjectRequester(reqArgs)
				_, err := r.OnSendResponse(mockStream)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(dht.ErrObjNotFound))
			})

			Specify("that the peer was added to the NOPE cache for the given key", func() {
				r := streamer.NewBasicObjectRequester(reqArgs)
				_, err := r.OnSendResponse(mockStream)
				Expect(err).ToNot(BeNil())
				Expect(reqArgs.ProviderTracker.DidPeerSendNope(remotePeer, reqArgs.Key)).To(BeTrue())
			})
		})

		It("should return packfile if msg type is 'PACK'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypePack)
				return len(dht.MsgTypePack), io.EOF
			})
			r := streamer.NewBasicObjectRequester(reqArgs)
			packfile, err := r.OnSendResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(packfile).ToNot(BeNil())
			data, err := ioutil.ReadAll(packfile)
			Expect(err).To(BeNil())
			Expect(data).To(Equal([]byte(dht.MsgTypePack)))
		})

		It("should return ErrUnknownMsgType if msg type is unknown", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, "UNKNOWN")
				return len("UNKNOWN"), nil
			})
			r := streamer.NewBasicObjectRequester(reqArgs)
			_, err := r.OnSendResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(streamer.ErrUnknownMsgType))
		})
	})
})
