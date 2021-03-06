// Code generated by MockGen. DO NOT EDIT.
// Source: remote/push/push_handler.go

// Package mocks is a generated GoMock package.
package mocks

import (
	pktline "github.com/go-git/go-git/v5/plumbing/format/pktline"
	packp "github.com/go-git/go-git/v5/plumbing/protocol/packp"
	gomock "github.com/golang/mock/gomock"
	types "github.com/make-os/kit/remote/push/types"
	util "github.com/make-os/kit/util"
	io "io"
	reflect "reflect"
)

// MockHandler is a mock of Handler interface
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// HandleStream mocks base method
func (m *MockHandler) HandleStream(packfile io.Reader, gitReceive io.WriteCloser, gitRcvCmd util.Cmd, pktEnc *pktline.Encoder) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleStream", packfile, gitReceive, gitRcvCmd, pktEnc)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleStream indicates an expected call of HandleStream
func (mr *MockHandlerMockRecorder) HandleStream(packfile, gitReceive, gitRcvCmd, pktEnc interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleStream", reflect.TypeOf((*MockHandler)(nil).HandleStream), packfile, gitReceive, gitRcvCmd, pktEnc)
}

// EnsureReferencesHaveTxDetail mocks base method
func (m *MockHandler) EnsureReferencesHaveTxDetail() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureReferencesHaveTxDetail")
	ret0, _ := ret[0].(error)
	return ret0
}

// EnsureReferencesHaveTxDetail indicates an expected call of EnsureReferencesHaveTxDetail
func (mr *MockHandlerMockRecorder) EnsureReferencesHaveTxDetail() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureReferencesHaveTxDetail", reflect.TypeOf((*MockHandler)(nil).EnsureReferencesHaveTxDetail))
}

// DoAuth mocks base method
func (m *MockHandler) DoAuth(ur *packp.ReferenceUpdateRequest, targetRef string, ignorePostRefs bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DoAuth", ur, targetRef, ignorePostRefs)
	ret0, _ := ret[0].(error)
	return ret0
}

// DoAuth indicates an expected call of DoAuth
func (mr *MockHandlerMockRecorder) DoAuth(ur, targetRef, ignorePostRefs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DoAuth", reflect.TypeOf((*MockHandler)(nil).DoAuth), ur, targetRef, ignorePostRefs)
}

// HandleAuthorization mocks base method
func (m *MockHandler) HandleAuthorization(ur *packp.ReferenceUpdateRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleAuthorization", ur)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleAuthorization indicates an expected call of HandleAuthorization
func (mr *MockHandlerMockRecorder) HandleAuthorization(ur interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleAuthorization", reflect.TypeOf((*MockHandler)(nil).HandleAuthorization), ur)
}

// HandleReferences mocks base method
func (m *MockHandler) HandleReferences() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleReferences")
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleReferences indicates an expected call of HandleReferences
func (mr *MockHandlerMockRecorder) HandleReferences() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleReferences", reflect.TypeOf((*MockHandler)(nil).HandleReferences))
}

// HandleGCAndSizeCheck mocks base method
func (m *MockHandler) HandleGCAndSizeCheck() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleGCAndSizeCheck")
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleGCAndSizeCheck indicates an expected call of HandleGCAndSizeCheck
func (mr *MockHandlerMockRecorder) HandleGCAndSizeCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleGCAndSizeCheck", reflect.TypeOf((*MockHandler)(nil).HandleGCAndSizeCheck))
}

// HandleUpdate mocks base method
func (m *MockHandler) HandleUpdate(targetNote types.PushNote) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleUpdate", targetNote)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleUpdate indicates an expected call of HandleUpdate
func (mr *MockHandlerMockRecorder) HandleUpdate(targetNote interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleUpdate", reflect.TypeOf((*MockHandler)(nil).HandleUpdate), targetNote)
}

// HandleReference mocks base method
func (m *MockHandler) HandleReference(ref string) []error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleReference", ref)
	ret0, _ := ret[0].([]error)
	return ret0
}

// HandleReference indicates an expected call of HandleReference
func (mr *MockHandlerMockRecorder) HandleReference(ref interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleReference", reflect.TypeOf((*MockHandler)(nil).HandleReference), ref)
}

// HandleReversion mocks base method
func (m *MockHandler) HandleReversion() []error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleReversion")
	ret0, _ := ret[0].([]error)
	return ret0
}

// HandleReversion indicates an expected call of HandleReversion
func (mr *MockHandlerMockRecorder) HandleReversion() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleReversion", reflect.TypeOf((*MockHandler)(nil).HandleReversion))
}

// HandlePushNote mocks base method
func (m *MockHandler) HandlePushNote(note types.PushNote) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandlePushNote", note)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandlePushNote indicates an expected call of HandlePushNote
func (mr *MockHandlerMockRecorder) HandlePushNote(note interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandlePushNote", reflect.TypeOf((*MockHandler)(nil).HandlePushNote), note)
}
