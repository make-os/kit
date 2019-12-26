// Code generated by MockGen. DO NOT EDIT.
// Source: ticket.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	types "github.com/makeos/mosdef/types"
	reflect "reflect"
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
func (m *MockTicketManager) Index(tx types.BaseTx, blockHeight uint64, txIndex int) error {
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

// GetByProposer mocks base method
func (m *MockTicketManager) GetByProposer(ticketType int, proposerPubKey string, queryOpt ...interface{}) ([]*types.Ticket, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ticketType, proposerPubKey}
	for _, a := range queryOpt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetByProposer", varargs...)
	ret0, _ := ret[0].([]*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByProposer indicates an expected call of GetByProposer
func (mr *MockTicketManagerMockRecorder) GetByProposer(ticketType, proposerPubKey interface{}, queryOpt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ticketType, proposerPubKey}, queryOpt...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByProposer", reflect.TypeOf((*MockTicketManager)(nil).GetByProposer), varargs...)
}

// CountActiveValidatorTickets mocks base method
func (m *MockTicketManager) CountActiveValidatorTickets() (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CountActiveValidatorTickets")
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CountActiveValidatorTickets indicates an expected call of CountActiveValidatorTickets
func (mr *MockTicketManagerMockRecorder) CountActiveValidatorTickets() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CountActiveValidatorTickets", reflect.TypeOf((*MockTicketManager)(nil).CountActiveValidatorTickets))
}

// GetActiveTicketsByProposer mocks base method
func (m *MockTicketManager) GetActiveTicketsByProposer(proposer string, ticketType int, addDelegated bool) ([]*types.Ticket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActiveTicketsByProposer", proposer, ticketType, addDelegated)
	ret0, _ := ret[0].([]*types.Ticket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActiveTicketsByProposer indicates an expected call of GetActiveTicketsByProposer
func (mr *MockTicketManagerMockRecorder) GetActiveTicketsByProposer(proposer, ticketType, addDelegated interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActiveTicketsByProposer", reflect.TypeOf((*MockTicketManager)(nil).GetActiveTicketsByProposer), proposer, ticketType, addDelegated)
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
func (m *MockTicketManager) Query(qf func(*types.Ticket) bool, queryOpt ...interface{}) []*types.Ticket {
	m.ctrl.T.Helper()
	varargs := []interface{}{qf}
	for _, a := range queryOpt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Query", varargs...)
	ret0, _ := ret[0].([]*types.Ticket)
	return ret0
}

// Query indicates an expected call of Query
func (mr *MockTicketManagerMockRecorder) Query(qf interface{}, queryOpt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{qf}, queryOpt...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockTicketManager)(nil).Query), varargs...)
}

// QueryOne mocks base method
func (m *MockTicketManager) QueryOne(qf func(*types.Ticket) bool) *types.Ticket {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryOne", qf)
	ret0, _ := ret[0].(*types.Ticket)
	return ret0
}

// QueryOne indicates an expected call of QueryOne
func (mr *MockTicketManagerMockRecorder) QueryOne(qf interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryOne", reflect.TypeOf((*MockTicketManager)(nil).QueryOne), qf)
}

// GetByHash mocks base method
func (m *MockTicketManager) GetByHash(hash string) *types.Ticket {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByHash", hash)
	ret0, _ := ret[0].(*types.Ticket)
	return ret0
}

// GetByHash indicates an expected call of GetByHash
func (mr *MockTicketManagerMockRecorder) GetByHash(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByHash", reflect.TypeOf((*MockTicketManager)(nil).GetByHash), hash)
}

// UpdateDecayBy mocks base method
func (m *MockTicketManager) UpdateDecayBy(hash string, newDecayHeight uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDecayBy", hash, newDecayHeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateDecayBy indicates an expected call of UpdateDecayBy
func (mr *MockTicketManagerMockRecorder) UpdateDecayBy(hash, newDecayHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDecayBy", reflect.TypeOf((*MockTicketManager)(nil).UpdateDecayBy), hash, newDecayHeight)
}

// GetOrderedLiveValidatorTickets mocks base method
func (m *MockTicketManager) GetOrderedLiveValidatorTickets(height int64, limit int) []*types.Ticket {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOrderedLiveValidatorTickets", height, limit)
	ret0, _ := ret[0].([]*types.Ticket)
	return ret0
}

// GetOrderedLiveValidatorTickets indicates an expected call of GetOrderedLiveValidatorTickets
func (mr *MockTicketManagerMockRecorder) GetOrderedLiveValidatorTickets(height, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOrderedLiveValidatorTickets", reflect.TypeOf((*MockTicketManager)(nil).GetOrderedLiveValidatorTickets), height, limit)
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
