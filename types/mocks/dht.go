// Code generated by MockGen. DO NOT EDIT.
// Source: dht.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	peer "github.com/libp2p/go-libp2p-core/peer"
	types "github.com/makeos/mosdef/types"
	reflect "reflect"
)

// MockObjectFinder is a mock of ObjectFinder interface
type MockObjectFinder struct {
	ctrl     *gomock.Controller
	recorder *MockObjectFinderMockRecorder
}

// MockObjectFinderMockRecorder is the mock recorder for MockObjectFinder
type MockObjectFinderMockRecorder struct {
	mock *MockObjectFinder
}

// NewMockObjectFinder creates a new mock instance
func NewMockObjectFinder(ctrl *gomock.Controller) *MockObjectFinder {
	mock := &MockObjectFinder{ctrl: ctrl}
	mock.recorder = &MockObjectFinderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockObjectFinder) EXPECT() *MockObjectFinderMockRecorder {
	return m.recorder
}

// FindObject mocks base method
func (m *MockObjectFinder) FindObject(key []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindObject", key)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindObject indicates an expected call of FindObject
func (mr *MockObjectFinderMockRecorder) FindObject(key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindObject", reflect.TypeOf((*MockObjectFinder)(nil).FindObject), key)
}

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

// Annonce mocks base method
func (m *MockDHT) Annonce(ctx context.Context, key []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Annonce", ctx, key)
	ret0, _ := ret[0].(error)
	return ret0
}

// Annonce indicates an expected call of Annonce
func (mr *MockDHTMockRecorder) Annonce(ctx, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Annonce", reflect.TypeOf((*MockDHT)(nil).Annonce), ctx, key)
}

// GetObject mocks base method
func (m *MockDHT) GetObject(ctx context.Context, query *types.DHTObjectQuery) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetObject", ctx, query)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetObject indicates an expected call of GetObject
func (mr *MockDHTMockRecorder) GetObject(ctx, query interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetObject", reflect.TypeOf((*MockDHT)(nil).GetObject), ctx, query)
}

// RegisterObjFinder mocks base method
func (m *MockDHT) RegisterObjFinder(objType string, finder types.ObjectFinder) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterObjFinder", objType, finder)
}

// RegisterObjFinder indicates an expected call of RegisterObjFinder
func (mr *MockDHTMockRecorder) RegisterObjFinder(objType, finder interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterObjFinder", reflect.TypeOf((*MockDHT)(nil).RegisterObjFinder), objType, finder)
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

// Close mocks base method
func (m *MockDHT) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockDHTMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockDHT)(nil).Close))
}
