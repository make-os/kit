// Code generated by MockGen. DO NOT EDIT.
// Source: dht/server/types/types.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	types "gitlab.com/makeos/mosdef/dht/streamer/types"
	reflect "reflect"
)

// MockDHT is a mock of DHT interface
type MockDHT struct {
	ctrl     *gomock.Controller
	recorder *MockDHTMockRecorder
}

// MockDHTMockRecorder is the mock recorder for MockDHT
type MockDHTMockRecorder struct {
	mock *MockDHT
}

// NewMockDHT creates a new mock instance
func NewMockDHT(ctrl *gomock.Controller) *MockDHT {
	mock := &MockDHT{ctrl: ctrl}
	mock.recorder = &MockDHTMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockDHT) EXPECT() *MockDHTMockRecorder {
	return m.recorder
}

// Store mocks base method
func (m *MockDHT) Store(ctx context.Context, key string, value []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Store", ctx, key, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// Store indicates an expected call of Store
func (mr *MockDHTMockRecorder) Store(ctx, key, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Store", reflect.TypeOf((*MockDHT)(nil).Store), ctx, key, value)
}

// Lookup mocks base method
func (m *MockDHT) Lookup(ctx context.Context, key string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lookup", ctx, key)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Lookup indicates an expected call of Lookup
func (mr *MockDHTMockRecorder) Lookup(ctx, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lookup", reflect.TypeOf((*MockDHT)(nil).Lookup), ctx, key)
}

// GetProviders mocks base method
func (m *MockDHT) GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProviders", ctx, key)
	ret0, _ := ret[0].([]peer.AddrInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetProviders indicates an expected call of GetProviders
func (mr *MockDHTMockRecorder) GetProviders(ctx, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProviders", reflect.TypeOf((*MockDHT)(nil).GetProviders), ctx, key)
}

// Announce mocks base method
func (m *MockDHT) Announce(key []byte, doneCB func(error)) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Announce", key, doneCB)
}

// Announce indicates an expected call of Announce
func (mr *MockDHTMockRecorder) Announce(key, doneCB interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Announce", reflect.TypeOf((*MockDHT)(nil).Announce), key, doneCB)
}

// ObjectStreamer mocks base method
func (m *MockDHT) ObjectStreamer() types.ObjectStreamer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ObjectStreamer")
	ret0, _ := ret[0].(types.ObjectStreamer)
	return ret0
}

// ObjectStreamer indicates an expected call of ObjectStreamer
func (mr *MockDHTMockRecorder) ObjectStreamer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObjectStreamer", reflect.TypeOf((*MockDHT)(nil).ObjectStreamer))
}

// Host mocks base method
func (m *MockDHT) Host() host.Host {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Host")
	ret0, _ := ret[0].(host.Host)
	return ret0
}

// Host indicates an expected call of Host
func (mr *MockDHTMockRecorder) Host() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Host", reflect.TypeOf((*MockDHT)(nil).Host))
}

// Start mocks base method
func (m *MockDHT) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockDHTMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockDHT)(nil).Start))
}

// Peers mocks base method
func (m *MockDHT) Peers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Peers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// Peers indicates an expected call of Peers
func (mr *MockDHTMockRecorder) Peers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Peers", reflect.TypeOf((*MockDHT)(nil).Peers))
}

// Stop mocks base method
func (m *MockDHT) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockDHTMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockDHT)(nil).Stop))
}
