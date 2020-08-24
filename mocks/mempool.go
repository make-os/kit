// Code generated by MockGen. DO NOT EDIT.
// Source: types/core/mempool.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	types1 "github.com/make-os/lobe/types"
	core "github.com/make-os/lobe/types/core"
	util "github.com/make-os/lobe/util"
	types "github.com/tendermint/tendermint/abci/types"
	mempool "github.com/tendermint/tendermint/mempool"
	types0 "github.com/tendermint/tendermint/types"
	reflect "reflect"
)

// MockMempool is a mock of Mempool interface
type MockMempool struct {
	ctrl     *gomock.Controller
	recorder *MockMempoolMockRecorder
}

// MockMempoolMockRecorder is the mock recorder for MockMempool
type MockMempoolMockRecorder struct {
	mock *MockMempool
}

// NewMockMempool creates a new mock instance
func NewMockMempool(ctrl *gomock.Controller) *MockMempool {
	mock := &MockMempool{ctrl: ctrl}
	mock.recorder = &MockMempoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockMempool) EXPECT() *MockMempoolMockRecorder {
	return m.recorder
}

// CheckTx mocks base method
func (m *MockMempool) CheckTx(tx types0.Tx, callback func(*types.Response)) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckTx", tx, callback)
	ret0, _ := ret[0].(error)
	return ret0
}

// CheckTx indicates an expected call of CheckTx
func (mr *MockMempoolMockRecorder) CheckTx(tx, callback interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckTx", reflect.TypeOf((*MockMempool)(nil).CheckTx), tx, callback)
}

// CheckTxWithInfo mocks base method
func (m *MockMempool) CheckTxWithInfo(tx types0.Tx, callback func(*types.Response), txInfo mempool.TxInfo) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckTxWithInfo", tx, callback, txInfo)
	ret0, _ := ret[0].(error)
	return ret0
}

// CheckTxWithInfo indicates an expected call of CheckTxWithInfo
func (mr *MockMempoolMockRecorder) CheckTxWithInfo(tx, callback, txInfo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckTxWithInfo", reflect.TypeOf((*MockMempool)(nil).CheckTxWithInfo), tx, callback, txInfo)
}

// ReapMaxBytesMaxGas mocks base method
func (m *MockMempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64) types0.Txs {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReapMaxBytesMaxGas", maxBytes, maxGas)
	ret0, _ := ret[0].(types0.Txs)
	return ret0
}

// ReapMaxBytesMaxGas indicates an expected call of ReapMaxBytesMaxGas
func (mr *MockMempoolMockRecorder) ReapMaxBytesMaxGas(maxBytes, maxGas interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReapMaxBytesMaxGas", reflect.TypeOf((*MockMempool)(nil).ReapMaxBytesMaxGas), maxBytes, maxGas)
}

// ReapMaxTxs mocks base method
func (m *MockMempool) ReapMaxTxs(max int) types0.Txs {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReapMaxTxs", max)
	ret0, _ := ret[0].(types0.Txs)
	return ret0
}

// ReapMaxTxs indicates an expected call of ReapMaxTxs
func (mr *MockMempoolMockRecorder) ReapMaxTxs(max interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReapMaxTxs", reflect.TypeOf((*MockMempool)(nil).ReapMaxTxs), max)
}

// Lock mocks base method
func (m *MockMempool) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock
func (mr *MockMempoolMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockMempool)(nil).Lock))
}

// Unlock mocks base method
func (m *MockMempool) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock
func (mr *MockMempoolMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockMempool)(nil).Unlock))
}

// Update mocks base method
func (m *MockMempool) Update(blockHeight int64, blockTxs types0.Txs, deliverTxResponses []*types.ResponseDeliverTx, newPreFn mempool.PreCheckFunc, newPostFn mempool.PostCheckFunc) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", blockHeight, blockTxs, deliverTxResponses, newPreFn, newPostFn)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update
func (mr *MockMempoolMockRecorder) Update(blockHeight, blockTxs, deliverTxResponses, newPreFn, newPostFn interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockMempool)(nil).Update), blockHeight, blockTxs, deliverTxResponses, newPreFn, newPostFn)
}

// FlushAppConn mocks base method
func (m *MockMempool) FlushAppConn() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FlushAppConn")
	ret0, _ := ret[0].(error)
	return ret0
}

// FlushAppConn indicates an expected call of FlushAppConn
func (mr *MockMempoolMockRecorder) FlushAppConn() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlushAppConn", reflect.TypeOf((*MockMempool)(nil).FlushAppConn))
}

// Flush mocks base method
func (m *MockMempool) Flush() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Flush")
}

// Flush indicates an expected call of Flush
func (mr *MockMempoolMockRecorder) Flush() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Flush", reflect.TypeOf((*MockMempool)(nil).Flush))
}

// TxsAvailable mocks base method
func (m *MockMempool) TxsAvailable() <-chan struct{} {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TxsAvailable")
	ret0, _ := ret[0].(<-chan struct{})
	return ret0
}

// TxsAvailable indicates an expected call of TxsAvailable
func (mr *MockMempoolMockRecorder) TxsAvailable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TxsAvailable", reflect.TypeOf((*MockMempool)(nil).TxsAvailable))
}

// EnableTxsAvailable mocks base method
func (m *MockMempool) EnableTxsAvailable() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "EnableTxsAvailable")
}

// EnableTxsAvailable indicates an expected call of EnableTxsAvailable
func (mr *MockMempoolMockRecorder) EnableTxsAvailable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableTxsAvailable", reflect.TypeOf((*MockMempool)(nil).EnableTxsAvailable))
}

// Size mocks base method
func (m *MockMempool) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size
func (mr *MockMempoolMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockMempool)(nil).Size))
}

// TxsBytes mocks base method
func (m *MockMempool) TxsBytes() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TxsBytes")
	ret0, _ := ret[0].(int64)
	return ret0
}

// TxsBytes indicates an expected call of TxsBytes
func (mr *MockMempoolMockRecorder) TxsBytes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TxsBytes", reflect.TypeOf((*MockMempool)(nil).TxsBytes))
}

// InitWAL mocks base method
func (m *MockMempool) InitWAL() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "InitWAL")
}

// InitWAL indicates an expected call of InitWAL
func (mr *MockMempoolMockRecorder) InitWAL() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitWAL", reflect.TypeOf((*MockMempool)(nil).InitWAL))
}

// CloseWAL mocks base method
func (m *MockMempool) CloseWAL() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CloseWAL")
}

// CloseWAL indicates an expected call of CloseWAL
func (mr *MockMempoolMockRecorder) CloseWAL() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseWAL", reflect.TypeOf((*MockMempool)(nil).CloseWAL))
}

// Add mocks base method
func (m *MockMempool) Add(tx types1.BaseTx) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add
func (mr *MockMempoolMockRecorder) Add(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockMempool)(nil).Add), tx)
}

// MockMempoolReactor is a mock of MempoolReactor interface
type MockMempoolReactor struct {
	ctrl     *gomock.Controller
	recorder *MockMempoolReactorMockRecorder
}

// MockMempoolReactorMockRecorder is the mock recorder for MockMempoolReactor
type MockMempoolReactorMockRecorder struct {
	mock *MockMempoolReactor
}

// NewMockMempoolReactor creates a new mock instance
func NewMockMempoolReactor(ctrl *gomock.Controller) *MockMempoolReactor {
	mock := &MockMempoolReactor{ctrl: ctrl}
	mock.recorder = &MockMempoolReactorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockMempoolReactor) EXPECT() *MockMempoolReactorMockRecorder {
	return m.recorder
}

// GetPoolSize mocks base method
func (m *MockMempoolReactor) GetPoolSize() *core.PoolSizeInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPoolSize")
	ret0, _ := ret[0].(*core.PoolSizeInfo)
	return ret0
}

// GetPoolSize indicates an expected call of GetPoolSize
func (mr *MockMempoolReactorMockRecorder) GetPoolSize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPoolSize", reflect.TypeOf((*MockMempoolReactor)(nil).GetPoolSize))
}

// GetTop mocks base method
func (m *MockMempoolReactor) GetTop(n int) []types1.BaseTx {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTop", n)
	ret0, _ := ret[0].([]types1.BaseTx)
	return ret0
}

// GetTop indicates an expected call of GetTop
func (mr *MockMempoolReactorMockRecorder) GetTop(n interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTop", reflect.TypeOf((*MockMempoolReactor)(nil).GetTop), n)
}

// AddTx mocks base method
func (m *MockMempoolReactor) AddTx(tx types1.BaseTx) (util.HexBytes, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTx", tx)
	ret0, _ := ret[0].(util.HexBytes)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddTx indicates an expected call of AddTx
func (mr *MockMempoolReactorMockRecorder) AddTx(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTx", reflect.TypeOf((*MockMempoolReactor)(nil).AddTx), tx)
}

// GetTx mocks base method
func (m *MockMempoolReactor) GetTx(hash string) types1.BaseTx {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTx", hash)
	ret0, _ := ret[0].(types1.BaseTx)
	return ret0
}

// GetTx indicates an expected call of GetTx
func (mr *MockMempoolReactorMockRecorder) GetTx(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTx", reflect.TypeOf((*MockMempoolReactor)(nil).GetTx), hash)
}
