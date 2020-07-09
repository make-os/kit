// Code generated by MockGen. DO NOT EDIT.
// Source: api/rpc/client/client.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	client "gitlab.com/makeos/mosdef/api/rpc/client"
	types "gitlab.com/makeos/mosdef/api/types"
	util "gitlab.com/makeos/mosdef/util"
	reflect "reflect"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// SendTxPayload mocks base method
func (m *MockClient) SendTxPayload(data map[string]interface{}) (*types.SendTxPayloadResponse, *util.ReqError) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendTxPayload", data)
	ret0, _ := ret[0].(*types.SendTxPayloadResponse)
	ret1, _ := ret[1].(*util.ReqError)
	return ret0, ret1
}

// SendTxPayload indicates an expected call of SendTxPayload
func (mr *MockClientMockRecorder) SendTxPayload(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendTxPayload", reflect.TypeOf((*MockClient)(nil).SendTxPayload), data)
}

// GetAccount mocks base method
func (m *MockClient) GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, *util.ReqError) {
	m.ctrl.T.Helper()
	varargs := []interface{}{address}
	for _, a := range blockHeight {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetAccount", varargs...)
	ret0, _ := ret[0].(*types.GetAccountResponse)
	ret1, _ := ret[1].(*util.ReqError)
	return ret0, ret1
}

// GetAccount indicates an expected call of GetAccount
func (mr *MockClientMockRecorder) GetAccount(address interface{}, blockHeight ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{address}, blockHeight...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccount", reflect.TypeOf((*MockClient)(nil).GetAccount), varargs...)
}

// GetPushKeyOwner mocks base method
func (m *MockClient) GetPushKeyOwner(id string, blockHeight ...uint64) (*types.GetAccountResponse, *util.ReqError) {
	m.ctrl.T.Helper()
	varargs := []interface{}{id}
	for _, a := range blockHeight {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetPushKeyOwner", varargs...)
	ret0, _ := ret[0].(*types.GetAccountResponse)
	ret1, _ := ret[1].(*util.ReqError)
	return ret0, ret1
}

// GetPushKeyOwner indicates an expected call of GetPushKeyOwner
func (mr *MockClientMockRecorder) GetPushKeyOwner(id interface{}, blockHeight ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{id}, blockHeight...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPushKeyOwner", reflect.TypeOf((*MockClient)(nil).GetPushKeyOwner), varargs...)
}

// RegisterPushKey mocks base method
func (m *MockClient) RegisterPushKey(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterPushKey", body)
	ret0, _ := ret[0].(*types.RegisterPushKeyResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterPushKey indicates an expected call of RegisterPushKey
func (mr *MockClientMockRecorder) RegisterPushKey(body interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterPushKey", reflect.TypeOf((*MockClient)(nil).RegisterPushKey), body)
}

// CreateRepo mocks base method
func (m *MockClient) CreateRepo(body *types.CreateRepoBody) (*types.CreateRepoResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRepo", body)
	ret0, _ := ret[0].(*types.CreateRepoResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRepo indicates an expected call of CreateRepo
func (mr *MockClientMockRecorder) CreateRepo(body interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRepo", reflect.TypeOf((*MockClient)(nil).CreateRepo), body)
}

// GetRepo mocks base method
func (m *MockClient) GetRepo(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, *util.ReqError) {
	m.ctrl.T.Helper()
	varargs := []interface{}{name}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetRepo", varargs...)
	ret0, _ := ret[0].(*types.GetRepoResponse)
	ret1, _ := ret[1].(*util.ReqError)
	return ret0, ret1
}

// GetRepo indicates an expected call of GetRepo
func (mr *MockClientMockRecorder) GetRepo(name interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRepo", reflect.TypeOf((*MockClient)(nil).GetRepo), varargs...)
}

// AddRepoContributors mocks base method
func (m *MockClient) AddRepoContributors(body *types.AddRepoContribsBody) (*types.AddRepoContribsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRepoContributors", body)
	ret0, _ := ret[0].(*types.AddRepoContribsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddRepoContributors indicates an expected call of AddRepoContributors
func (mr *MockClientMockRecorder) AddRepoContributors(body interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRepoContributors", reflect.TypeOf((*MockClient)(nil).AddRepoContributors), body)
}

// GetOptions mocks base method
func (m *MockClient) GetOptions() *client.Options {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOptions")
	ret0, _ := ret[0].(*client.Options)
	return ret0
}

// GetOptions indicates an expected call of GetOptions
func (mr *MockClientMockRecorder) GetOptions() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOptions", reflect.TypeOf((*MockClient)(nil).GetOptions))
}

// Call mocks base method
func (m *MockClient) Call(method string, params interface{}) (util.Map, int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Call", method, params)
	ret0, _ := ret[0].(util.Map)
	ret1, _ := ret[1].(int)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Call indicates an expected call of Call
func (mr *MockClientMockRecorder) Call(method, params interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*MockClient)(nil).Call), method, params)
}
