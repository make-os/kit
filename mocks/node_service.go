// Code generated by MockGen. DO NOT EDIT.
// Source: node/services/service.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/make-os/kit/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	types0 "github.com/tendermint/tendermint/types"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// GetBlock mocks base method.
func (m *MockService) GetBlock(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlock", ctx, height)
	ret0, _ := ret[0].(*coretypes.ResultBlock)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlock indicates an expected call of GetBlock.
func (mr *MockServiceMockRecorder) GetBlock(ctx, height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlock", reflect.TypeOf((*MockService)(nil).GetBlock), ctx, height)
}

// GetTx mocks base method.
func (m *MockService) GetTx(ctx context.Context, hash []byte, proof bool) (types.BaseTx, *types0.TxProof, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTx", ctx, hash, proof)
	ret0, _ := ret[0].(types.BaseTx)
	ret1, _ := ret[1].(*types0.TxProof)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetTx indicates an expected call of GetTx.
func (mr *MockServiceMockRecorder) GetTx(ctx, hash, proof interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTx", reflect.TypeOf((*MockService)(nil).GetTx), ctx, hash, proof)
}

// IsSyncing mocks base method.
func (m *MockService) IsSyncing(ctx context.Context) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSyncing", ctx)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsSyncing indicates an expected call of IsSyncing.
func (mr *MockServiceMockRecorder) IsSyncing(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSyncing", reflect.TypeOf((*MockService)(nil).IsSyncing), ctx)
}

// NetInfo mocks base method.
func (m *MockService) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetInfo", ctx)
	ret0, _ := ret[0].(*coretypes.ResultNetInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetInfo indicates an expected call of NetInfo.
func (mr *MockServiceMockRecorder) NetInfo(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetInfo", reflect.TypeOf((*MockService)(nil).NetInfo), ctx)
}
