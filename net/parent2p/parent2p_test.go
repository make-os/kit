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

		It("should return error if message length is less than 4", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, []byte{1})
				return 0, nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("bad message length"))
		})

		It("should return error if message format is invalid", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := "valid_type part1 unexpected_part"
				copy(p, v)
				return len(v), nil
			})
			err := p2p.Handler(mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("bad message format"))
		})

		It("should return error if message type is unsupported", func() {
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				v := "unknown_type"
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

		It("should return error when handshake response message length is invalid", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, []byte{1})
				return 0, nil
			})
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			_, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("bad message length"))
		})

		It("should return error when message format is unexpected", func() {
			trackList := []string{"repo"}
			mockStream := mocks.NewMockStream(ctrl)
			mockStream.EXPECT().Write(parent2p.MakeHandshakeMsg(trackList)).Return(0, nil)
			bad := bytes.NewBuffer([]byte("valid_type something something_unexpected"))
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
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, "unknown")
				return 7, nil
			})
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
			mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				copy(p, parent2p.MakeAckHandshakeMsg("127.0.0.1:2333"))
				return 7, nil
			})
			mockStream.EXPECT().Close()
			mockHost.EXPECT().NewStream(gomock.Any(), gomock.Any(), parent2p.ProtocolID).Return(mockStream, nil)
			addr, err := p2p.SendHandshakeMsg(context.Background(), trackList)
			Expect(err).To(BeNil())
			Expect(addr).To(Equal("127.0.0.1:2333"))
		})
	})

	Describe(".HandleHandshake", func() {
		BeforeEach(func() {
			p2p = parent2p.New(cfg, h)
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
			bz := "repo1,repo2"
			err = p2p.HandleHandshake(bz, mockStream)
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
			err = p2p.HandleHandshake("", mockStream)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to send ack handshake: error"))
		})
	})
})
