// Code generated by MockGen. DO NOT EDIT.
// Source: ticket.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/makeos/mosdef/types"
)

// MockTicketManager is a mock of TicketManager interface
type MockTicketManager struct {
	ctrl     *gomock.Controller
	recorder *MockTicketManagerMockRecorder
}

// MockTicketManagerMockRecorder is the mock recorder for MockTicketManager
type MockTicketManagerMockRecorder struct {
	mock *MockTicketManager
}

// NewMockTicketManager creates a new mock instance
func NewMockTicketManager(ctrl *gomock.Controller) *MockTicketManager {
	mock := &MockTicketManager{ctrl: ctrl}
	mock.recorder = &MockTicketManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockTicketManager) EXPECT() *MockTicketManagerMockRecorder {
	return m.recorder
}

// Index mocks base method
func (m *MockTicketManager) Index(tx *types.Transaction, blockHeight uint64, txIndex int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Index", tx, blockHeight, txIndex)
	ret0, _ := ret[0].(error)
	return ret0
}

// Index indicates an expected call of Index
func (mr *MockTicketManagerMockRecorder) Index(tx, blockHeight, txIndex interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Index", reflect.TypeOf((*MockTicketManager)(nil).Index), tx, blockHeight, txIndex)
}

// GetValidatorTicketByProposer mocks base method
func (m *MockTicketManager) GetValidatorTicketByProposer(proposerPubKey string, queryOpt types.QueryOptions) ([]*types.Ticket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetValidatorTicketByProposer", proposerPubKey, queryOpt)
	ret0, _ := ret[0].([]*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidatorTicketByProposer indicates an expected call of GetValidatorTicketByProposer
func (mr *MockTicketManagerMockRecorder) GetValidatorTicketByProposer(proposerPubKey, queryOpt interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidatorTicketByProposer", reflect.TypeOf((*MockTicketManager)(nil).GetValidatorTicketByProposer), proposerPubKey, queryOpt)
}

// CountLiveValidatorsValidatorTickets mocks base method
func (m *MockTicketManager) CountLiveValidatorsValidatorTickets(arg0 ...types.QueryOptions) (int, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CountLiveValidatorsValidatorTickets", varargs...)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CountLiveValidatorsValidatorTickets indicates an expected call of CountLiveValidatorsValidatorTickets
func (mr *MockTicketManagerMockRecorder) CountLiveValidatorsValidatorTickets(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CountLiveValidatorsValidatorTickets", reflect.TypeOf((*MockTicketManager)(nil).CountLiveValidatorsValidatorTickets), arg0...)
}

// SelectRandom mocks base method
func (m *MockTicketManager) SelectRandom(height int64, seed []byte, limit int) ([]*types.Ticket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SelectRandom", height, seed, limit)
	ret0, _ := ret[0].([]*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SelectRandom indicates an expected call of SelectRandom
func (mr *MockTicketManagerMockRecorder) SelectRandom(height, seed, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SelectRandom", reflect.TypeOf((*MockTicketManager)(nil).SelectRandom), height, seed, limit)
}

// Query mocks base method
func (m *MockTicketManager) Query(q types.Ticket, queryOpt ...types.QueryOptions) ([]*types.Ticket, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{q}
	for _, a := range queryOpt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Query", varargs...)
	ret0, _ := ret[0].([]*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query
func (mr *MockTicketManagerMockRecorder) Query(q interface{}, queryOpt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{q}, queryOpt...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockTicketManager)(nil).Query), varargs...)
}

// QueryOne mocks base method
func (m *MockTicketManager) QueryOne(q types.Ticket, queryOpt ...types.QueryOptions) (*types.Ticket, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{q}
	for _, a := range queryOpt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "QueryOne", varargs...)
	ret0, _ := ret[0].(*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryOne indicates an expected call of QueryOne
func (mr *MockTicketManagerMockRecorder) QueryOne(q interface{}, queryOpt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{q}, queryOpt...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryOne", reflect.TypeOf((*MockTicketManager)(nil).QueryOne), varargs...)
}

// Remove mocks base method
func (m *MockTicketManager) Remove(hash string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", hash)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove
func (mr *MockTicketManagerMockRecorder) Remove(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockTicketManager)(nil).Remove), hash)
}

// Stop mocks base method
func (m *MockTicketManager) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockTicketManagerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockTicketManager)(nil).Stop))
}
