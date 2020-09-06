// Code generated by MockGen. DO NOT EDIT.
// Source: remote/refsync/types/types.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	types "github.com/make-os/lobe/remote/refsync/types"
	txns "github.com/make-os/lobe/types/txns"
	reflect "reflect"
)

// MockWatcher is a mock of Watcher interface
type MockWatcher struct {
	ctrl     *gomock.Controller
	recorder *MockWatcherMockRecorder
}

// MockWatcherMockRecorder is the mock recorder for MockWatcher
type MockWatcherMockRecorder struct {
	mock *MockWatcher
}

// NewMockWatcher creates a new mock instance
func NewMockWatcher(ctrl *gomock.Controller) *MockWatcher {
	mock := &MockWatcher{ctrl: ctrl}
	mock.recorder = &MockWatcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockWatcher) EXPECT() *MockWatcherMockRecorder {
	return m.recorder
}

// Do mocks base method
func (m *MockWatcher) Do(task *types.WatcherTask) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Do", task)
	ret0, _ := ret[0].(error)
	return ret0
}

// Do indicates an expected call of Do
func (mr *MockWatcherMockRecorder) Do(task interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Do", reflect.TypeOf((*MockWatcher)(nil).Do), task)
}

// Watch mocks base method
func (m *MockWatcher) Watch(repo, reference string, startHeight, endHeight uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Watch", repo, reference, startHeight, endHeight)
}

// Watch indicates an expected call of Watch
func (mr *MockWatcherMockRecorder) Watch(repo, reference, startHeight, endHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockWatcher)(nil).Watch), repo, reference, startHeight, endHeight)
}

// QueueSize mocks base method
func (m *MockWatcher) QueueSize() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueueSize")
	ret0, _ := ret[0].(int)
	return ret0
}

// QueueSize indicates an expected call of QueueSize
func (mr *MockWatcherMockRecorder) QueueSize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueueSize", reflect.TypeOf((*MockWatcher)(nil).QueueSize))
}

// HasTask mocks base method
func (m *MockWatcher) HasTask() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasTask")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasTask indicates an expected call of HasTask
func (mr *MockWatcherMockRecorder) HasTask() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasTask", reflect.TypeOf((*MockWatcher)(nil).HasTask))
}

// IsRunning mocks base method
func (m *MockWatcher) IsRunning() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsRunning")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsRunning indicates an expected call of IsRunning
func (mr *MockWatcherMockRecorder) IsRunning() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsRunning", reflect.TypeOf((*MockWatcher)(nil).IsRunning))
}

// Start mocks base method
func (m *MockWatcher) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start
func (mr *MockWatcherMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockWatcher)(nil).Start))
}

// Stop mocks base method
func (m *MockWatcher) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop
func (mr *MockWatcherMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockWatcher)(nil).Stop))
}

// MockRefSync is a mock of RefSync interface
type MockRefSync struct {
	ctrl     *gomock.Controller
	recorder *MockRefSyncMockRecorder
}

// MockRefSyncMockRecorder is the mock recorder for MockRefSync
type MockRefSyncMockRecorder struct {
	mock *MockRefSync
}

// NewMockRefSync creates a new mock instance
func NewMockRefSync(ctrl *gomock.Controller) *MockRefSync {
	mock := &MockRefSync{ctrl: ctrl}
	mock.recorder = &MockRefSyncMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockRefSync) EXPECT() *MockRefSyncMockRecorder {
	return m.recorder
}

// OnNewTx mocks base method
func (m *MockRefSync) OnNewTx(tx *txns.TxPush, targetRef string, txIndex int, height int64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "OnNewTx", tx, targetRef, txIndex, height)
}

// OnNewTx indicates an expected call of OnNewTx
func (mr *MockRefSyncMockRecorder) OnNewTx(tx, targetRef, txIndex, height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnNewTx", reflect.TypeOf((*MockRefSync)(nil).OnNewTx), tx, targetRef, txIndex, height)
}

// Watch mocks base method
func (m *MockRefSync) Watch(repo, reference string, startHeight, endHeight uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Watch", repo, reference, startHeight, endHeight)
}

// Watch indicates an expected call of Watch
func (mr *MockRefSyncMockRecorder) Watch(repo, reference, startHeight, endHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockRefSync)(nil).Watch), repo, reference, startHeight, endHeight)
}

// CanSync mocks base method
func (m *MockRefSync) CanSync(namespace, repoName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CanSync", namespace, repoName)
	ret0, _ := ret[0].(error)
	return ret0
}

// CanSync indicates an expected call of CanSync
func (mr *MockRefSyncMockRecorder) CanSync(namespace, repoName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CanSync", reflect.TypeOf((*MockRefSync)(nil).CanSync), namespace, repoName)
}

// Stop mocks base method
func (m *MockRefSync) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop
func (mr *MockRefSyncMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockRefSync)(nil).Stop))
}
