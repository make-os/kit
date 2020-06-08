package dht_test

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
	"github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/testutil"
	io2 "gitlab.com/makeos/mosdef/util/io"
)

var _ = Describe("BasicCommitRequester", func() {
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
			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost})
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

			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost})
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

			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost})
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

			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost})
			err := r.WriteToStream(mockStream, data)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("write error"))
		})
	})

	Describe(".DoWant", func() {
		It("should return no error when there are no providers", func() {
			ctx := context.Background()
			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost})
			err := r.DoWant(ctx)
			Expect(err).To(BeNil())
		})

		It("should return no error and skip provider if it has no address info", func() {
			ctx := context.Background()
			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost, Providers: []peer.AddrInfo{{
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
			mockHost.EXPECT().NewStream(ctx, prov.ID, dht.CommitStreamProtocolID).Return(nil, fmt.Errorf("error"))

			r := dht.NewCommitRequester(dht.CommitRequesterArgs{Host: mockHost, Providers: []peer.AddrInfo{prov}, Log: log})
			err := r.DoWant(ctx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error"))
		})

		It("should return error when 'WANT' message is sent, 'WANT' response handler must be called", func() {
			ctx := context.Background()
			repoName := "repo1"
			key := []byte("commit_key")
			prov := peer.AddrInfo{Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1")}}

			mockPeerstore := mocks.NewMockPeerstore(ctrl)
			mockPeerstore.EXPECT().AddAddr(prov.ID, prov.Addrs[0], peerstore.ProviderAddrTTL)
			mockHost.EXPECT().Peerstore().Return(mockPeerstore)
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().SetDeadline(gomock.Any())
			mockStream.EXPECT().Write(dht.MakeWantMsg(repoName, key)).Return(0, nil)
			mockHost.EXPECT().NewStream(ctx, prov.ID, dht.CommitStreamProtocolID).Return(mockStream, nil)

			r := dht.NewCommitRequester(dht.CommitRequesterArgs{
				Host:       mockHost,
				Providers:  []peer.AddrInfo{prov},
				RepoName:   repoName,
				RequestKey: key,
				Log:        log,
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
			var reqArgs dht.CommitRequesterArgs
			var mockStream *mocks.MockStream
			var repoName = "repo1"
			var key = []byte("commit_key")

			BeforeEach(func() {
				mockStream = mocks.NewMockStream(ctrl)
				reqArgs = dht.CommitRequesterArgs{
					Host:            mockHost,
					ProviderStreams: []network.Stream{mockStream},
					RepoName:        repoName,
					RequestKey:      key,
					Log:             log,
				}
			})

			It("should return error when context has expired or is cancelled", func() {
				ctx, cn := context.WithCancel(context.Background())
				cn()
				mockStream.EXPECT().Reset()
				r := dht.NewCommitRequester(reqArgs)
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
				r := dht.NewCommitRequester(reqArgs)
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
				r := dht.NewCommitRequester(reqArgs)
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
				r := dht.NewCommitRequester(reqArgs)

				tmpFile, _ := ioutil.TempFile(os.TempDir(), "")
				defer tmpFile.Close()
				r.OnSendResponseHandler = func(network.Stream) (io2.ReadSeekerCloser, error) {
					return tmpFile, nil
				}
				packfile, err := r.Do(ctx)
				Expect(err).To(BeNil())
				Expect(packfile).NotTo(BeNil())
				Expect(packfile).To(Equal(tmpFile))
			})
		})
	})

	Describe(".OnWantResponse", func() {
		var mockStream *mocks.MockStream
		var reqArgs dht.CommitRequesterArgs

		BeforeEach(func() {
			mockStream = mocks.NewMockStream(ctrl)
			reqArgs = dht.CommitRequesterArgs{}
		})

		It("should return error when unable to read message type from stream", func() {
			mockStream.EXPECT().Read(make([]byte, 4)).Return(0, fmt.Errorf("read error"))
			r := dht.NewCommitRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read message type: read error"))
		})

		It("should add stream to provider's stream cache, when message type is 'HAVE'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypeHave)
				return len(dht.MsgTypeHave), nil
			})
			r := dht.NewCommitRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(r.GetProviderStreams()).To(HaveLen(1))
			Expect(r.GetProviderStreams()[0]).To(Equal(mockStream))
		})

		It("should reset stream, when message type is 'NOPE'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypeNope)
				return len(dht.MsgTypeNope), nil
			})
			mockStream.EXPECT().Reset()
			r := dht.NewCommitRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(r.GetProviderStreams()).To(HaveLen(0))
		})

		It("should reset stream, when message type is 'UNKNOWN'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, "UNKNOWN")
				return len("UNKNOWN"), nil
			})
			mockStream.EXPECT().Reset()
			r := dht.NewCommitRequester(reqArgs)
			err := r.OnWantResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(r.GetProviderStreams()).To(HaveLen(0))
		})
	})

	Describe(".OnSendResponse", func() {
		var mockStream *mocks.MockStream
		var reqArgs dht.CommitRequesterArgs

		BeforeEach(func() {
			mockStream = mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Reset()
			reqArgs = dht.CommitRequesterArgs{}
		})

		It("should return error when unable to read message type from stream", func() {
			mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("read error"))
			r := dht.NewCommitRequester(reqArgs)
			_, err := r.OnSendResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unable to read msg type: read error"))
		})

		It("should return ErrObjNotFound if msg type is 'NOPE'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypeNope)
				return len(dht.MsgTypeNope), nil
			})
			r := dht.NewCommitRequester(reqArgs)
			_, err := r.OnSendResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(dht.ErrObjNotFound))
		})

		It("should return packfile if msg type is 'PACK'", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, dht.MsgTypePack)
				return len(dht.MsgTypePack), io.EOF
			})
			r := dht.NewCommitRequester(reqArgs)
			packfile, err := r.OnSendResponse(mockStream)
			Expect(err).To(BeNil())
			Expect(packfile).ToNot(BeNil())
			data, err := ioutil.ReadAll(packfile)
			Expect(err).To(BeNil())
			Expect(data).To(Equal([]byte(dht.MsgTypePack)))
		})

		It("should return ErrUnknownMshType if msg type is unknown", func() {
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, "UNKNOWN")
				return len("UNKNOWN"), nil
			})
			r := dht.NewCommitRequester(reqArgs)
			_, err := r.OnSendResponse(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(dht.ErrUnknownMsgType))
		})
	})
})
