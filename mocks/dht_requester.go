// Code generated by MockGen. DO NOT EDIT.
// Source: dht/streamer/requester.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	network "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	streamer "github.com/make-os/lobe/dht/streamer"
	io "github.com/make-os/lobe/util/io"
	reflect "reflect"
)

// MockObjectRequester is a mock of ObjectRequester interface
type MockObjectRequester struct {
	ctrl     *gomock.Controller
	recorder *MockObjectRequesterMockRecorder
}

// MockObjectRequesterMockRecorder is the mock recorder for MockObjectRequester
type MockObjectRequesterMockRecorder struct {
	mock *MockObjectRequester
}

// NewMockObjectRequester creates a new mock instance
func NewMockObjectRequester(ctrl *gomock.Controller) *MockObjectRequester {
	mock := &MockObjectRequester{ctrl: ctrl}
	mock.recorder = &MockObjectRequesterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockObjectRequester) EXPECT() *MockObjectRequesterMockRecorder {
	return m.recorder
}

// Write mocks base method
func (m *MockObjectRequester) Write(ctx context.Context, prov peer.AddrInfo, pid protocol.ID, data []byte) (network.Stream, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", ctx, prov, pid, data)
	ret0, _ := ret[0].(network.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Write indicates an expected call of Write
func (mr *MockObjectRequesterMockRecorder) Write(ctx, prov, pid, data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockObjectRequester)(nil).Write), ctx, prov, pid, data)
}

// WriteToStream mocks base method
func (m *MockObjectRequester) WriteToStream(str network.Stream, data []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteToStream", str, data)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteToStream indicates an expected call of WriteToStream
func (mr *MockObjectRequesterMockRecorder) WriteToStream(str, data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteToStream", reflect.TypeOf((*MockObjectRequester)(nil).WriteToStream), str, data)
}

// DoWant mocks base method
func (m *MockObjectRequester) DoWant(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DoWant", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// DoWant indicates an expected call of DoWant
func (mr *MockObjectRequesterMockRecorder) DoWant(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DoWant", reflect.TypeOf((*MockObjectRequester)(nil).DoWant), ctx)
}

// Do mocks base method
func (m *MockObjectRequester) Do(ctx context.Context) (*streamer.PackResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Do", ctx)
	ret0, _ := ret[0].(*streamer.PackResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Do indicates an expected call of Do
func (mr *MockObjectRequesterMockRecorder) Do(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Do", reflect.TypeOf((*MockObjectRequester)(nil).Do), ctx)
}

// GetProviderStreams mocks base method
func (m *MockObjectRequester) GetProviderStreams() []network.Stream {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProviderStreams")
	ret0, _ := ret[0].([]network.Stream)
	return ret0
}

// GetProviderStreams indicates an expected call of GetProviderStreams
func (mr *MockObjectRequesterMockRecorder) GetProviderStreams() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProviderStreams", reflect.TypeOf((*MockObjectRequester)(nil).GetProviderStreams))
}

// OnWantResponse mocks base method
func (m *MockObjectRequester) OnWantResponse(s network.Stream) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnWantResponse", s)
	ret0, _ := ret[0].(error)
	return ret0
}

// OnWantResponse indicates an expected call of OnWantResponse
func (mr *MockObjectRequesterMockRecorder) OnWantResponse(s interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnWantResponse", reflect.TypeOf((*MockObjectRequester)(nil).OnWantResponse), s)
}

// OnSendResponse mocks base method
func (m *MockObjectRequester) OnSendResponse(s network.Stream) (io.ReadSeekerCloser, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnSendResponse", s)
	ret0, _ := ret[0].(io.ReadSeekerCloser)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OnSendResponse indicates an expected call of OnSendResponse
func (mr *MockObjectRequesterMockRecorder) OnSendResponse(s interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnSendResponse", reflect.TypeOf((*MockObjectRequester)(nil).OnSendResponse), s)
}

// AddProviderStream mocks base method
func (m *MockObjectRequester) AddProviderStream(streams ...network.Stream) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range streams {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "AddProviderStream", varargs...)
}

// AddProviderStream indicates an expected call of AddProviderStream
func (mr *MockObjectRequesterMockRecorder) AddProviderStream(streams ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddProviderStream", reflect.TypeOf((*MockObjectRequester)(nil).AddProviderStream), streams...)
}
