// Code generated by MockGen. DO NOT EDIT.
// Source: types/libp2p.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	connmgr "github.com/libp2p/go-libp2p-core/connmgr"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	event "github.com/libp2p/go-libp2p-core/event"
	network "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// MockHost is a mock of Host interface.
type MockHost struct {
	ctrl     *gomock.Controller
	recorder *MockHostMockRecorder
}

// MockHostMockRecorder is the mock recorder for MockHost.
type MockHostMockRecorder struct {
	mock *MockHost
}

// NewMockHost creates a new mock instance.
func NewMockHost(ctrl *gomock.Controller) *MockHost {
	mock := &MockHost{ctrl: ctrl}
	mock.recorder = &MockHostMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHost) EXPECT() *MockHostMockRecorder {
	return m.recorder
}

// Addrs mocks base method.
func (m *MockHost) Addrs() []multiaddr.Multiaddr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Addrs")
	ret0, _ := ret[0].([]multiaddr.Multiaddr)
	return ret0
}

// Addrs indicates an expected call of Addrs.
func (mr *MockHostMockRecorder) Addrs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Addrs", reflect.TypeOf((*MockHost)(nil).Addrs))
}

// Close mocks base method.
func (m *MockHost) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockHostMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockHost)(nil).Close))
}

// ConnManager mocks base method.
func (m *MockHost) ConnManager() connmgr.ConnManager {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConnManager")
	ret0, _ := ret[0].(connmgr.ConnManager)
	return ret0
}

// ConnManager indicates an expected call of ConnManager.
func (mr *MockHostMockRecorder) ConnManager() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConnManager", reflect.TypeOf((*MockHost)(nil).ConnManager))
}

// Connect mocks base method.
func (m *MockHost) Connect(ctx context.Context, pi peer.AddrInfo) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Connect", ctx, pi)
	ret0, _ := ret[0].(error)
	return ret0
}

// Connect indicates an expected call of Connect.
func (mr *MockHostMockRecorder) Connect(ctx, pi interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connect", reflect.TypeOf((*MockHost)(nil).Connect), ctx, pi)
}

// EventBus mocks base method.
func (m *MockHost) EventBus() event.Bus {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EventBus")
	ret0, _ := ret[0].(event.Bus)
	return ret0
}

// EventBus indicates an expected call of EventBus.
func (mr *MockHostMockRecorder) EventBus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EventBus", reflect.TypeOf((*MockHost)(nil).EventBus))
}

// ID mocks base method.
func (m *MockHost) ID() peer.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ID")
	ret0, _ := ret[0].(peer.ID)
	return ret0
}

// ID indicates an expected call of ID.
func (mr *MockHostMockRecorder) ID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ID", reflect.TypeOf((*MockHost)(nil).ID))
}

// Mux mocks base method.
func (m *MockHost) Mux() protocol.Switch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mux")
	ret0, _ := ret[0].(protocol.Switch)
	return ret0
}

// Mux indicates an expected call of Mux.
func (mr *MockHostMockRecorder) Mux() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mux", reflect.TypeOf((*MockHost)(nil).Mux))
}

// Network mocks base method.
func (m *MockHost) Network() network.Network {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Network")
	ret0, _ := ret[0].(network.Network)
	return ret0
}

// Network indicates an expected call of Network.
func (mr *MockHostMockRecorder) Network() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Network", reflect.TypeOf((*MockHost)(nil).Network))
}

// NewStream mocks base method.
func (m *MockHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, p}
	for _, a := range pids {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "NewStream", varargs...)
	ret0, _ := ret[0].(network.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewStream indicates an expected call of NewStream.
func (mr *MockHostMockRecorder) NewStream(ctx, p interface{}, pids ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, p}, pids...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewStream", reflect.TypeOf((*MockHost)(nil).NewStream), varargs...)
}

// Peerstore mocks base method.
func (m *MockHost) Peerstore() peerstore.Peerstore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Peerstore")
	ret0, _ := ret[0].(peerstore.Peerstore)
	return ret0
}

// Peerstore indicates an expected call of Peerstore.
func (mr *MockHostMockRecorder) Peerstore() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Peerstore", reflect.TypeOf((*MockHost)(nil).Peerstore))
}

// RemoveStreamHandler mocks base method.
func (m *MockHost) RemoveStreamHandler(pid protocol.ID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveStreamHandler", pid)
}

// RemoveStreamHandler indicates an expected call of RemoveStreamHandler.
func (mr *MockHostMockRecorder) RemoveStreamHandler(pid interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveStreamHandler", reflect.TypeOf((*MockHost)(nil).RemoveStreamHandler), pid)
}

// SetStreamHandler mocks base method.
func (m *MockHost) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetStreamHandler", pid, handler)
}

// SetStreamHandler indicates an expected call of SetStreamHandler.
func (mr *MockHostMockRecorder) SetStreamHandler(pid, handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetStreamHandler", reflect.TypeOf((*MockHost)(nil).SetStreamHandler), pid, handler)
}

// SetStreamHandlerMatch mocks base method.
func (m *MockHost) SetStreamHandlerMatch(arg0 protocol.ID, arg1 func(string) bool, arg2 network.StreamHandler) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetStreamHandlerMatch", arg0, arg1, arg2)
}

// SetStreamHandlerMatch indicates an expected call of SetStreamHandlerMatch.
func (mr *MockHostMockRecorder) SetStreamHandlerMatch(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetStreamHandlerMatch", reflect.TypeOf((*MockHost)(nil).SetStreamHandlerMatch), arg0, arg1, arg2)
}

// MockPeerstore is a mock of Peerstore interface.
type MockPeerstore struct {
	ctrl     *gomock.Controller
	recorder *MockPeerstoreMockRecorder
}

// MockPeerstoreMockRecorder is the mock recorder for MockPeerstore.
type MockPeerstoreMockRecorder struct {
	mock *MockPeerstore
}

// NewMockPeerstore creates a new mock instance.
func NewMockPeerstore(ctrl *gomock.Controller) *MockPeerstore {
	mock := &MockPeerstore{ctrl: ctrl}
	mock.recorder = &MockPeerstoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPeerstore) EXPECT() *MockPeerstoreMockRecorder {
	return m.recorder
}

// AddAddr mocks base method.
func (m *MockPeerstore) AddAddr(p peer.ID, addr multiaddr.Multiaddr, ttl time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddAddr", p, addr, ttl)
}

// AddAddr indicates an expected call of AddAddr.
func (mr *MockPeerstoreMockRecorder) AddAddr(p, addr, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddAddr", reflect.TypeOf((*MockPeerstore)(nil).AddAddr), p, addr, ttl)
}

// AddAddrs mocks base method.
func (m *MockPeerstore) AddAddrs(p peer.ID, addrs []multiaddr.Multiaddr, ttl time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddAddrs", p, addrs, ttl)
}

// AddAddrs indicates an expected call of AddAddrs.
func (mr *MockPeerstoreMockRecorder) AddAddrs(p, addrs, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddAddrs", reflect.TypeOf((*MockPeerstore)(nil).AddAddrs), p, addrs, ttl)
}

// AddPrivKey mocks base method.
func (m *MockPeerstore) AddPrivKey(arg0 peer.ID, arg1 crypto.PrivKey) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddPrivKey", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddPrivKey indicates an expected call of AddPrivKey.
func (mr *MockPeerstoreMockRecorder) AddPrivKey(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPrivKey", reflect.TypeOf((*MockPeerstore)(nil).AddPrivKey), arg0, arg1)
}

// AddProtocols mocks base method.
func (m *MockPeerstore) AddProtocols(arg0 peer.ID, arg1 ...string) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AddProtocols", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddProtocols indicates an expected call of AddProtocols.
func (mr *MockPeerstoreMockRecorder) AddProtocols(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddProtocols", reflect.TypeOf((*MockPeerstore)(nil).AddProtocols), varargs...)
}

// AddPubKey mocks base method.
func (m *MockPeerstore) AddPubKey(arg0 peer.ID, arg1 crypto.PubKey) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddPubKey", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddPubKey indicates an expected call of AddPubKey.
func (mr *MockPeerstoreMockRecorder) AddPubKey(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPubKey", reflect.TypeOf((*MockPeerstore)(nil).AddPubKey), arg0, arg1)
}

// AddrStream mocks base method.
func (m *MockPeerstore) AddrStream(arg0 context.Context, arg1 peer.ID) <-chan multiaddr.Multiaddr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddrStream", arg0, arg1)
	ret0, _ := ret[0].(<-chan multiaddr.Multiaddr)
	return ret0
}

// AddrStream indicates an expected call of AddrStream.
func (mr *MockPeerstoreMockRecorder) AddrStream(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddrStream", reflect.TypeOf((*MockPeerstore)(nil).AddrStream), arg0, arg1)
}

// Addrs mocks base method.
func (m *MockPeerstore) Addrs(p peer.ID) []multiaddr.Multiaddr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Addrs", p)
	ret0, _ := ret[0].([]multiaddr.Multiaddr)
	return ret0
}

// Addrs indicates an expected call of Addrs.
func (mr *MockPeerstoreMockRecorder) Addrs(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Addrs", reflect.TypeOf((*MockPeerstore)(nil).Addrs), p)
}

// ClearAddrs mocks base method.
func (m *MockPeerstore) ClearAddrs(p peer.ID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearAddrs", p)
}

// ClearAddrs indicates an expected call of ClearAddrs.
func (mr *MockPeerstoreMockRecorder) ClearAddrs(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearAddrs", reflect.TypeOf((*MockPeerstore)(nil).ClearAddrs), p)
}

// Close mocks base method.
func (m *MockPeerstore) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockPeerstoreMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockPeerstore)(nil).Close))
}

// FirstSupportedProtocol mocks base method.
func (m *MockPeerstore) FirstSupportedProtocol(arg0 peer.ID, arg1 ...string) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "FirstSupportedProtocol", varargs...)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FirstSupportedProtocol indicates an expected call of FirstSupportedProtocol.
func (mr *MockPeerstoreMockRecorder) FirstSupportedProtocol(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FirstSupportedProtocol", reflect.TypeOf((*MockPeerstore)(nil).FirstSupportedProtocol), varargs...)
}

// Get mocks base method.
func (m *MockPeerstore) Get(p peer.ID, key string) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", p, key)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockPeerstoreMockRecorder) Get(p, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPeerstore)(nil).Get), p, key)
}

// GetProtocols mocks base method.
func (m *MockPeerstore) GetProtocols(arg0 peer.ID) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProtocols", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetProtocols indicates an expected call of GetProtocols.
func (mr *MockPeerstoreMockRecorder) GetProtocols(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProtocols", reflect.TypeOf((*MockPeerstore)(nil).GetProtocols), arg0)
}

// LatencyEWMA mocks base method.
func (m *MockPeerstore) LatencyEWMA(arg0 peer.ID) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LatencyEWMA", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// LatencyEWMA indicates an expected call of LatencyEWMA.
func (mr *MockPeerstoreMockRecorder) LatencyEWMA(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatencyEWMA", reflect.TypeOf((*MockPeerstore)(nil).LatencyEWMA), arg0)
}

// PeerInfo mocks base method.
func (m *MockPeerstore) PeerInfo(arg0 peer.ID) peer.AddrInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeerInfo", arg0)
	ret0, _ := ret[0].(peer.AddrInfo)
	return ret0
}

// PeerInfo indicates an expected call of PeerInfo.
func (mr *MockPeerstoreMockRecorder) PeerInfo(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeerInfo", reflect.TypeOf((*MockPeerstore)(nil).PeerInfo), arg0)
}

// Peers mocks base method.
func (m *MockPeerstore) Peers() peer.IDSlice {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Peers")
	ret0, _ := ret[0].(peer.IDSlice)
	return ret0
}

// Peers indicates an expected call of Peers.
func (mr *MockPeerstoreMockRecorder) Peers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Peers", reflect.TypeOf((*MockPeerstore)(nil).Peers))
}

// PeersWithAddrs mocks base method.
func (m *MockPeerstore) PeersWithAddrs() peer.IDSlice {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeersWithAddrs")
	ret0, _ := ret[0].(peer.IDSlice)
	return ret0
}

// PeersWithAddrs indicates an expected call of PeersWithAddrs.
func (mr *MockPeerstoreMockRecorder) PeersWithAddrs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeersWithAddrs", reflect.TypeOf((*MockPeerstore)(nil).PeersWithAddrs))
}

// PeersWithKeys mocks base method.
func (m *MockPeerstore) PeersWithKeys() peer.IDSlice {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeersWithKeys")
	ret0, _ := ret[0].(peer.IDSlice)
	return ret0
}

// PeersWithKeys indicates an expected call of PeersWithKeys.
func (mr *MockPeerstoreMockRecorder) PeersWithKeys() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeersWithKeys", reflect.TypeOf((*MockPeerstore)(nil).PeersWithKeys))
}

// PrivKey mocks base method.
func (m *MockPeerstore) PrivKey(arg0 peer.ID) crypto.PrivKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrivKey", arg0)
	ret0, _ := ret[0].(crypto.PrivKey)
	return ret0
}

// PrivKey indicates an expected call of PrivKey.
func (mr *MockPeerstoreMockRecorder) PrivKey(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrivKey", reflect.TypeOf((*MockPeerstore)(nil).PrivKey), arg0)
}

// PubKey mocks base method.
func (m *MockPeerstore) PubKey(arg0 peer.ID) crypto.PubKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PubKey", arg0)
	ret0, _ := ret[0].(crypto.PubKey)
	return ret0
}

// PubKey indicates an expected call of PubKey.
func (mr *MockPeerstoreMockRecorder) PubKey(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PubKey", reflect.TypeOf((*MockPeerstore)(nil).PubKey), arg0)
}

// Put mocks base method.
func (m *MockPeerstore) Put(p peer.ID, key string, val interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", p, key, val)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put.
func (mr *MockPeerstoreMockRecorder) Put(p, key, val interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockPeerstore)(nil).Put), p, key, val)
}

// RecordLatency mocks base method.
func (m *MockPeerstore) RecordLatency(arg0 peer.ID, arg1 time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RecordLatency", arg0, arg1)
}

// RecordLatency indicates an expected call of RecordLatency.
func (mr *MockPeerstoreMockRecorder) RecordLatency(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordLatency", reflect.TypeOf((*MockPeerstore)(nil).RecordLatency), arg0, arg1)
}

// RemoveProtocols mocks base method.
func (m *MockPeerstore) RemoveProtocols(arg0 peer.ID, arg1 ...string) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RemoveProtocols", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveProtocols indicates an expected call of RemoveProtocols.
func (mr *MockPeerstoreMockRecorder) RemoveProtocols(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveProtocols", reflect.TypeOf((*MockPeerstore)(nil).RemoveProtocols), varargs...)
}

// SetAddr mocks base method.
func (m *MockPeerstore) SetAddr(p peer.ID, addr multiaddr.Multiaddr, ttl time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetAddr", p, addr, ttl)
}

// SetAddr indicates an expected call of SetAddr.
func (mr *MockPeerstoreMockRecorder) SetAddr(p, addr, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAddr", reflect.TypeOf((*MockPeerstore)(nil).SetAddr), p, addr, ttl)
}

// SetAddrs mocks base method.
func (m *MockPeerstore) SetAddrs(p peer.ID, addrs []multiaddr.Multiaddr, ttl time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetAddrs", p, addrs, ttl)
}

// SetAddrs indicates an expected call of SetAddrs.
func (mr *MockPeerstoreMockRecorder) SetAddrs(p, addrs, ttl interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAddrs", reflect.TypeOf((*MockPeerstore)(nil).SetAddrs), p, addrs, ttl)
}

// SetProtocols mocks base method.
func (m *MockPeerstore) SetProtocols(arg0 peer.ID, arg1 ...string) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetProtocols", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetProtocols indicates an expected call of SetProtocols.
func (mr *MockPeerstoreMockRecorder) SetProtocols(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProtocols", reflect.TypeOf((*MockPeerstore)(nil).SetProtocols), varargs...)
}

// SupportsProtocols mocks base method.
func (m *MockPeerstore) SupportsProtocols(arg0 peer.ID, arg1 ...string) ([]string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SupportsProtocols", varargs...)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SupportsProtocols indicates an expected call of SupportsProtocols.
func (mr *MockPeerstoreMockRecorder) SupportsProtocols(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SupportsProtocols", reflect.TypeOf((*MockPeerstore)(nil).SupportsProtocols), varargs...)
}

// UpdateAddrs mocks base method.
func (m *MockPeerstore) UpdateAddrs(p peer.ID, oldTTL, newTTL time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateAddrs", p, oldTTL, newTTL)
}

// UpdateAddrs indicates an expected call of UpdateAddrs.
func (mr *MockPeerstoreMockRecorder) UpdateAddrs(p, oldTTL, newTTL interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAddrs", reflect.TypeOf((*MockPeerstore)(nil).UpdateAddrs), p, oldTTL, newTTL)
}

// MockStream is a mock of Stream interface.
type MockStream struct {
	ctrl     *gomock.Controller
	recorder *MockStreamMockRecorder
}

// MockStreamMockRecorder is the mock recorder for MockStream.
type MockStreamMockRecorder struct {
	mock *MockStream
}

// NewMockStream creates a new mock instance.
func NewMockStream(ctrl *gomock.Controller) *MockStream {
	mock := &MockStream{ctrl: ctrl}
	mock.recorder = &MockStreamMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStream) EXPECT() *MockStreamMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockStream) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockStreamMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockStream)(nil).Close))
}

// CloseRead mocks base method.
func (m *MockStream) CloseRead() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseRead")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseRead indicates an expected call of CloseRead.
func (mr *MockStreamMockRecorder) CloseRead() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseRead", reflect.TypeOf((*MockStream)(nil).CloseRead))
}

// CloseWrite mocks base method.
func (m *MockStream) CloseWrite() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseWrite")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseWrite indicates an expected call of CloseWrite.
func (mr *MockStreamMockRecorder) CloseWrite() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseWrite", reflect.TypeOf((*MockStream)(nil).CloseWrite))
}

// Conn mocks base method.
func (m *MockStream) Conn() network.Conn {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Conn")
	ret0, _ := ret[0].(network.Conn)
	return ret0
}

// Conn indicates an expected call of Conn.
func (mr *MockStreamMockRecorder) Conn() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Conn", reflect.TypeOf((*MockStream)(nil).Conn))
}

// ID mocks base method.
func (m *MockStream) ID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ID indicates an expected call of ID.
func (mr *MockStreamMockRecorder) ID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ID", reflect.TypeOf((*MockStream)(nil).ID))
}

// Protocol mocks base method.
func (m *MockStream) Protocol() protocol.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Protocol")
	ret0, _ := ret[0].(protocol.ID)
	return ret0
}

// Protocol indicates an expected call of Protocol.
func (mr *MockStreamMockRecorder) Protocol() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Protocol", reflect.TypeOf((*MockStream)(nil).Protocol))
}

// Read mocks base method.
func (m *MockStream) Read(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockStreamMockRecorder) Read(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockStream)(nil).Read), p)
}

// Reset mocks base method.
func (m *MockStream) Reset() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reset")
	ret0, _ := ret[0].(error)
	return ret0
}

// Reset indicates an expected call of Reset.
func (mr *MockStreamMockRecorder) Reset() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reset", reflect.TypeOf((*MockStream)(nil).Reset))
}

// SetDeadline mocks base method.
func (m *MockStream) SetDeadline(arg0 time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDeadline", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetDeadline indicates an expected call of SetDeadline.
func (mr *MockStreamMockRecorder) SetDeadline(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDeadline", reflect.TypeOf((*MockStream)(nil).SetDeadline), arg0)
}

// SetProtocol mocks base method.
func (m *MockStream) SetProtocol(id protocol.ID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetProtocol", id)
}

// SetProtocol indicates an expected call of SetProtocol.
func (mr *MockStreamMockRecorder) SetProtocol(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProtocol", reflect.TypeOf((*MockStream)(nil).SetProtocol), id)
}

// SetReadDeadline mocks base method.
func (m *MockStream) SetReadDeadline(arg0 time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetReadDeadline", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetReadDeadline indicates an expected call of SetReadDeadline.
func (mr *MockStreamMockRecorder) SetReadDeadline(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetReadDeadline", reflect.TypeOf((*MockStream)(nil).SetReadDeadline), arg0)
}

// SetWriteDeadline mocks base method.
func (m *MockStream) SetWriteDeadline(arg0 time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetWriteDeadline", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetWriteDeadline indicates an expected call of SetWriteDeadline.
func (mr *MockStreamMockRecorder) SetWriteDeadline(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetWriteDeadline", reflect.TypeOf((*MockStream)(nil).SetWriteDeadline), arg0)
}

// Stat mocks base method.
func (m *MockStream) Stat() network.Stat {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat")
	ret0, _ := ret[0].(network.Stat)
	return ret0
}

// Stat indicates an expected call of Stat.
func (mr *MockStreamMockRecorder) Stat() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockStream)(nil).Stat))
}

// Write mocks base method.
func (m *MockStream) Write(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Write indicates an expected call of Write.
func (mr *MockStreamMockRecorder) Write(p interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockStream)(nil).Write), p)
}

// MockConn is a mock of Conn interface.
type MockConn struct {
	ctrl     *gomock.Controller
	recorder *MockConnMockRecorder
}

// MockConnMockRecorder is the mock recorder for MockConn.
type MockConnMockRecorder struct {
	mock *MockConn
}

// NewMockConn creates a new mock instance.
func NewMockConn(ctrl *gomock.Controller) *MockConn {
	mock := &MockConn{ctrl: ctrl}
	mock.recorder = &MockConnMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConn) EXPECT() *MockConnMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockConn) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockConnMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockConn)(nil).Close))
}

// GetStreams mocks base method.
func (m *MockConn) GetStreams() []network.Stream {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStreams")
	ret0, _ := ret[0].([]network.Stream)
	return ret0
}

// GetStreams indicates an expected call of GetStreams.
func (mr *MockConnMockRecorder) GetStreams() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStreams", reflect.TypeOf((*MockConn)(nil).GetStreams))
}

// ID mocks base method.
func (m *MockConn) ID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ID indicates an expected call of ID.
func (mr *MockConnMockRecorder) ID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ID", reflect.TypeOf((*MockConn)(nil).ID))
}

// LocalMultiaddr mocks base method.
func (m *MockConn) LocalMultiaddr() multiaddr.Multiaddr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalMultiaddr")
	ret0, _ := ret[0].(multiaddr.Multiaddr)
	return ret0
}

// LocalMultiaddr indicates an expected call of LocalMultiaddr.
func (mr *MockConnMockRecorder) LocalMultiaddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalMultiaddr", reflect.TypeOf((*MockConn)(nil).LocalMultiaddr))
}

// LocalPeer mocks base method.
func (m *MockConn) LocalPeer() peer.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalPeer")
	ret0, _ := ret[0].(peer.ID)
	return ret0
}

// LocalPeer indicates an expected call of LocalPeer.
func (mr *MockConnMockRecorder) LocalPeer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalPeer", reflect.TypeOf((*MockConn)(nil).LocalPeer))
}

// LocalPrivateKey mocks base method.
func (m *MockConn) LocalPrivateKey() crypto.PrivKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalPrivateKey")
	ret0, _ := ret[0].(crypto.PrivKey)
	return ret0
}

// LocalPrivateKey indicates an expected call of LocalPrivateKey.
func (mr *MockConnMockRecorder) LocalPrivateKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalPrivateKey", reflect.TypeOf((*MockConn)(nil).LocalPrivateKey))
}

// NewStream mocks base method.
func (m *MockConn) NewStream() (network.Stream, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewStream")
	ret0, _ := ret[0].(network.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewStream indicates an expected call of NewStream.
func (mr *MockConnMockRecorder) NewStream() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewStream", reflect.TypeOf((*MockConn)(nil).NewStream))
}

// RemoteMultiaddr mocks base method.
func (m *MockConn) RemoteMultiaddr() multiaddr.Multiaddr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoteMultiaddr")
	ret0, _ := ret[0].(multiaddr.Multiaddr)
	return ret0
}

// RemoteMultiaddr indicates an expected call of RemoteMultiaddr.
func (mr *MockConnMockRecorder) RemoteMultiaddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoteMultiaddr", reflect.TypeOf((*MockConn)(nil).RemoteMultiaddr))
}

// RemotePeer mocks base method.
func (m *MockConn) RemotePeer() peer.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemotePeer")
	ret0, _ := ret[0].(peer.ID)
	return ret0
}

// RemotePeer indicates an expected call of RemotePeer.
func (mr *MockConnMockRecorder) RemotePeer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemotePeer", reflect.TypeOf((*MockConn)(nil).RemotePeer))
}

// RemotePublicKey mocks base method.
func (m *MockConn) RemotePublicKey() crypto.PubKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemotePublicKey")
	ret0, _ := ret[0].(crypto.PubKey)
	return ret0
}

// RemotePublicKey indicates an expected call of RemotePublicKey.
func (mr *MockConnMockRecorder) RemotePublicKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemotePublicKey", reflect.TypeOf((*MockConn)(nil).RemotePublicKey))
}

// Stat mocks base method.
func (m *MockConn) Stat() network.Stat {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat")
	ret0, _ := ret[0].(network.Stat)
	return ret0
}

// Stat indicates an expected call of Stat.
func (mr *MockConnMockRecorder) Stat() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockConn)(nil).Stat))
}
