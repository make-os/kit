package parent2p_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/net"
	"github.com/make-os/kit/net/parent2p"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestParent2p(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parent2p Suite")
}

var _ = Describe("Parent2p", func() {
	var cfg, cfg2 *config.AppConfig
	var err error
	var p2p *parent2p.BasicParent2P
	var ctrl *gomock.Controller
	var h *net.BasicHost

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
		err = os.RemoveAll(cfg2.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConnectToParent", func() {
		BeforeEach(func() {
			cfg.DHT.Address = testutil.RandomAddr()
			h, err = net.New(context.Background(), cfg)
			Expect(err).To(BeNil())
			p2p = parent2p.New(cfg, h)
		})

		It("should return error when parent address is not a valid multiaddr", func() {
			err := p2p.ConnectToParent(context.Background(), "some-invalid-addr")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("bad parent address: failed to parse multiaddr"))
		})

		It("should return error when parent address is self", func() {
			err := p2p.ConnectToParent(context.Background(), h.FullAddr())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("cannot connect to self"))
		})

		It("should return nil when connected to parent", func() {
			cfg2.DHT.Address = testutil.RandomAddr()
			h2, err := net.New(context.Background(), cfg2)
			Expect(err).To(BeNil())
			err = p2p.ConnectToParent(context.Background(), h2.FullAddr())
			Expect(err).To(BeNil())
		})

		It("should return err when parent is offline", func() {
			cfg2.DHT.Address = testutil.RandomAddr()
			h2, err := net.New(context.Background(), cfg2)
			Expect(err).To(BeNil())
			addr := h2.FullAddr()
			_ = h2.Get().Close()
			err = p2p.ConnectToParent(context.Background(), addr)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect failed: failed to dial"))
		})
	})

	Describe(".Handler", func() {
		It("should return error if unable to read stream", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("error"))
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to read message: error"))
		})

		It("should return error if unable to decode message", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := []byte("gibberish")
				copy(p, v)
				return len(v), nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to decode message"))
		})

		It("should return error if message format is invalid", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := util.ToBytes([]interface{}{"valid_type", "part1", "unexpected_part"})
				copy(p, v)
				return len(v), nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("bad message format"))
		})

		It("should return error if message could not be decoded", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := []byte("gibberish")
				copy(p, v)
				return len(v), nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to decode message"))
		})

		It("should return error if message type is unsupported", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := util.ToBytes([]interface{}{"unknown_type", "stuff"})
				copy(p, v)
				return len(v), nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unknown message type"))
		})
	})

	Describe(".SendHandshakeMsg", func() {

		var mockHost *mocks.MockHost
		BeforeEach(func() {
			mockHost = mocks.NewMockHost(ctrl)
			mockHost.EXPECT().SetStreamHandler(gomock.Any(), gomock.Any())
			p2p = parent2p.New(cfg, net.NewWithHost(mockHost))
		})

		It("should return error when unable to create stream", func() {
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).
				Return(nil, fmt.Errorf("error"))
			_, err := p2p.SendHandshakeMsg(context.Background(), []string{"repo"})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to open stream: error"))
		})

		It("should return error when unable to write to stream", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, fmt.Errorf("error"))
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to send handshake: error"))
		})

		It("should return error when to unable to read handshake response message", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("error"))
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to read handshake response message: error"))
		})

		It("should return error when message could not be decoded", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			bad := bytes.NewBuffer([]byte("gibberish"))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return bad.Read(p)
			}).AnyTimes()
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to decode message"))
		})

		It("should return error when message format is unexpected", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			bad := bytes.NewBuffer(util.ToBytes([]interface{}{"valid_type", "something", "something_unexpected"}))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return bad.Read(p)
			}).AnyTimes()
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("bad message format"))
		})

		It("should return error when handshake response message has unknown type", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			body := bytes.NewBuffer(util.ToBytes([]interface{}{"valid_type", "something"}))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return body.Read(p)
			}).AnyTimes()
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unknown message type"))
		})

		It("should return nil on success", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			body := bytes.NewBuffer(parent2p.MakeAckHandshakeMsg("127.0.0.1:2333"))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return body.Read(p)
			}).AnyTimes()
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			addr, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).To(BeNil())
			Expect(addr).To(Equal("127.0.0.1:2333"))
		})
	})

	Describe(".HandleHandshakeMsg", func() {
		BeforeEach(func() {
			mockHost := mocks.NewMockHost(ctrl)
			mockHost.EXPECT().SetStreamHandler(gomock.Any(), gomock.Any())
			p2p = parent2p.New(cfg, net.NewWithHost(mockHost))
		})

		It("should cache remote peer and track list and send acknowledgement that contains RPC address", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			peerID := peer.ID("remote-peer")
			mockConn.EXPECT().RemotePeer().Return(peerID)
			mockStream.EXPECT().Conn().Return(mockConn)
			cfg.RPC.TMRPCAddress = testutil.RandomAddr()
			mockStream.EXPECT().Write(parent2p.MakeAckHandshakeMsg(cfg.RPC.TMRPCAddress))
			trackList := []string{"repo1", "repo2"}
			err = p2p.HandleHandshakeMsg(trackList, mockStream)
			Expect(err).To(BeNil())
			peers := p2p.Peers()
			Expect(peers).To(HaveKey(peerID.String()))
			Expect(peers[peerID.String()].TrackList).To(HaveLen(2))
			Expect(peers[peerID.String()].TrackList).To(HaveKey("repo1"))
			Expect(peers[peerID.String()].TrackList).To(HaveKey("repo2"))
		})

		It("should return error when unable to write handshake acknowledgement", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(peer.ID("remote-peer"))
			mockStream.EXPECT().Conn().Return(mockConn)
			cfg.RPC.TMRPCAddress = testutil.RandomAddr()
			mockStream.EXPECT().Write(gomock.Any()).Return(0, fmt.Errorf("error"))
			err = p2p.HandleHandshakeMsg(nil, mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to send ack handshake: error"))
		})
	})

	Describe(".SendUpdateTrackListMsg", func() {
		var mockHost *mocks.MockHost
		BeforeEach(func() {
			mockHost = mocks.NewMockHost(ctrl)
			mockHost.EXPECT().SetStreamHandler(gomock.Any(), gomock.Any())
			p2p = parent2p.New(cfg, net.NewWithHost(mockHost))
		})

		It("should return error when unable to create stream", func() {
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).
				Return(nil, fmt.Errorf("error"))
			err := p2p.SendUpdateTrackListMsg(context.Background(), []string{"repo1"}, []string{"repo2"})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to open stream: error"))
		})

		It("should return error when unable to write to stream", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			mockStream.EXPECT().Close()
			addList := []string{"repo1"}
			rmList := []string{"repo2"}
			mockStream.EXPECT().Write(parent2p.MakeTrackListUpdateMsg([]string{"repo1", "-repo2"})).
				Return(0, fmt.Errorf("error"))
			err := p2p.SendUpdateTrackListMsg(context.Background(), addList, rmList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to write message: error"))
		})

		It("should return error when unable to read response", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			mockStream.EXPECT().Close()

			mockStream.EXPECT().Write(parent2p.MakeTrackListUpdateMsg([]string{"repo1", "-repo2"})).Return(0, nil)

			mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("error"))
			err := p2p.SendUpdateTrackListMsg(context.Background(), []string{"repo1"}, []string{"repo2"})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to read response message: error"))
		})

		It("should return error when reject message was received", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			mockStream.EXPECT().Close()

			mockStream.EXPECT().Write(parent2p.MakeTrackListUpdateMsg([]string{"repo1", "-repo2"})).Return(0, nil)

			bad := bytes.NewBuffer(parent2p.MakeRejectMsg("rejected"))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return bad.Read(p)
			}).AnyTimes()

			err := p2p.SendUpdateTrackListMsg(context.Background(), []string{"repo1"}, []string{"repo2"})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("message was rejected: rejected"))
		})

		It("should return error when message type is unknown", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			mockStream.EXPECT().Close()

			mockStream.EXPECT().Write(parent2p.MakeTrackListUpdateMsg([]string{"repo1", "-repo2"})).Return(0, nil)

			bad := bytes.NewBuffer(util.ToBytes([]interface{}{"unknown"}))
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return bad.Read(p)
			}).AnyTimes()

			err := p2p.SendUpdateTrackListMsg(context.Background(), []string{"repo1"}, []string{"repo2"})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unknown message type"))
		})

		It("should return no error when OKAY message was received", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			mockStream.EXPECT().Close()

			mockStream.EXPECT().Write(parent2p.MakeTrackListUpdateMsg([]string{"repo1", "-repo2"})).Return(0, nil)

			bad := bytes.NewBuffer(parent2p.MakeOkayMsg())
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return bad.Read(p)
			}).AnyTimes()

			err := p2p.SendUpdateTrackListMsg(context.Background(), []string{"repo1"}, []string{"repo2"})
			Expect(err).To(BeNil())
		})
	})

	Describe(".HandleUpdateTrackListMsg", func() {
		BeforeEach(func() {
			mockHost := mocks.NewMockHost(ctrl)
			mockHost.EXPECT().SetStreamHandler(gomock.Any(), gomock.Any())
			p2p = parent2p.New(cfg, net.NewWithHost(mockHost))
		})

		It("should write a reject message if remote peer is unknown", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			peerID := peer.ID("remote-peer")
			mockConn.EXPECT().RemotePeer().Return(peerID)
			mockStream.EXPECT().Conn().Return(mockConn)
			mockStream.EXPECT().Write(parent2p.MakeRejectMsg(parent2p.ErrUnknownPeer.Error()))
			trackList := []string{"repo1", "repo2"}
			err = p2p.HandleUpdateTrackListMsg(trackList, mockStream)
			Expect(err).To(BeNil())
		})

		It("should return error when unable to write a reject message", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			peerID := peer.ID("remote-peer")
			mockConn.EXPECT().RemotePeer().Return(peerID)
			mockStream.EXPECT().Conn().Return(mockConn)
			mockStream.EXPECT().Write(parent2p.MakeRejectMsg(parent2p.ErrUnknownPeer.Error())).Return(0, fmt.Errorf("error"))
			trackList := []string{"repo1", "repo2"}
			err = p2p.HandleUpdateTrackListMsg(trackList, mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to write message: error"))
		})

		It("should return write OKAY message on success and apply new tracklist", func() {
			peerID := peer.ID("remote-peer")
			p2p.Peers()[peerID.String()] = &parent2p.ChildPeer{
				TrackList: map[string]struct{}{
					"repo1": {},
					"repo3": {},
				},
			}

			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(peerID)
			mockStream.EXPECT().Conn().Return(mockConn)
			mockStream.EXPECT().Write(parent2p.MakeOkayMsg()).Return(0, nil)

			trackList := []string{"-repo1", "repo2", "repo1", ""}
			err = p2p.HandleUpdateTrackListMsg(trackList, mockStream)
			Expect(err).To(BeNil())

			p := p2p.Peers()[peerID.String()]
			Expect(p.TrackList).To(HaveLen(2))
			Expect(p.TrackList).To(HaveKey("repo2"))
			Expect(p.TrackList).To(HaveKey("repo3"))
		})

		It("should return error when unable to write OKAY message", func() {
			peerID := peer.ID("remote-peer")
			p2p.Peers()[peerID.String()] = &parent2p.ChildPeer{
				TrackList: map[string]struct{}{
					"repo1": {},
					"repo3": {},
				},
			}

			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Close()
			mockConn := mocks.NewMockConn(ctrl)
			mockConn.EXPECT().RemotePeer().Return(peerID)
			mockStream.EXPECT().Conn().Return(mockConn)
			mockStream.EXPECT().Write(parent2p.MakeOkayMsg()).Return(0, fmt.Errorf("error"))

			trackList := []string{"-repo1", "repo2", "repo1", ""}
			err = p2p.HandleUpdateTrackListMsg(trackList, mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to write OKAY message: error"))
		})
	})
})
