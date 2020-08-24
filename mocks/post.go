// Code generated by MockGen. DO NOT EDIT.
// Source: remote/plumbing/post.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	plumbing "github.com/make-os/lobe/remote/plumbing"
	reflect "reflect"
)

// MockPostEntry is a mock of PostEntry interface
type MockPostEntry struct {
	ctrl     *gomock.Controller
	recorder *MockPostEntryMockRecorder
}

// MockPostEntryMockRecorder is the mock recorder for MockPostEntry
type MockPostEntryMockRecorder struct {
	mock *MockPostEntry
}

// NewMockPostEntry creates a new mock instance
func NewMockPostEntry(ctrl *gomock.Controller) *MockPostEntry {
	mock := &MockPostEntry{ctrl: ctrl}
	mock.recorder = &MockPostEntryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPostEntry) EXPECT() *MockPostEntryMockRecorder {
	return m.recorder
}

// GetComments mocks base method
func (m *MockPostEntry) GetComments() (plumbing.Comments, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetComments")
	ret0, _ := ret[0].(plumbing.Comments)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetComments indicates an expected call of GetComments
func (mr *MockPostEntryMockRecorder) GetComments() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetComments", reflect.TypeOf((*MockPostEntry)(nil).GetComments))
}

// IsClosed mocks base method
func (m *MockPostEntry) IsClosed() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsClosed")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsClosed indicates an expected call of IsClosed
func (mr *MockPostEntryMockRecorder) IsClosed() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsClosed", reflect.TypeOf((*MockPostEntry)(nil).IsClosed))
}

// GetTitle mocks base method
func (m *MockPostEntry) GetTitle() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTitle")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetTitle indicates an expected call of GetTitle
func (mr *MockPostEntryMockRecorder) GetTitle() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTitle", reflect.TypeOf((*MockPostEntry)(nil).GetTitle))
}

// GetName mocks base method
func (m *MockPostEntry) GetName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetName")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetName indicates an expected call of GetName
func (mr *MockPostEntryMockRecorder) GetName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetName", reflect.TypeOf((*MockPostEntry)(nil).GetName))
}

// Comment mocks base method
func (m *MockPostEntry) Comment() *plumbing.Comment {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Comment")
	ret0, _ := ret[0].(*plumbing.Comment)
	return ret0
}

// Comment indicates an expected call of Comment
func (mr *MockPostEntryMockRecorder) Comment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Comment", reflect.TypeOf((*MockPostEntry)(nil).Comment))
}
