// Code generated by MockGen. DO NOT EDIT.
// Source: keystore/types/types.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	crypto "github.com/make-os/lobe/crypto"
	types "github.com/make-os/lobe/keystore/types"
	io "io"
	reflect "reflect"
	time "time"
)

// MockStoredKey is a mock of StoredKey interface
type MockStoredKey struct {
	ctrl     *gomock.Controller
	recorder *MockStoredKeyMockRecorder
}

// MockStoredKeyMockRecorder is the mock recorder for MockStoredKey
type MockStoredKeyMockRecorder struct {
	mock *MockStoredKey
}

// NewMockStoredKey creates a new mock instance
func NewMockStoredKey(ctrl *gomock.Controller) *MockStoredKey {
	mock := &MockStoredKey{ctrl: ctrl}
	mock.recorder = &MockStoredKeyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockStoredKey) EXPECT() *MockStoredKeyMockRecorder {
	return m.recorder
}

// GetMeta mocks base method
func (m *MockStoredKey) GetMeta() types.StoredKeyMeta {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMeta")
	ret0, _ := ret[0].(types.StoredKeyMeta)
	return ret0
}

// GetMeta indicates an expected call of GetMeta
func (mr *MockStoredKeyMockRecorder) GetMeta() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMeta", reflect.TypeOf((*MockStoredKey)(nil).GetMeta))
}

// GetKey mocks base method
func (m *MockStoredKey) GetKey() *crypto.Key {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKey")
	ret0, _ := ret[0].(*crypto.Key)
	return ret0
}

// GetKey indicates an expected call of GetKey
func (mr *MockStoredKeyMockRecorder) GetKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKey", reflect.TypeOf((*MockStoredKey)(nil).GetKey))
}

// GetPayload mocks base method
func (m *MockStoredKey) GetPayload() *types.KeyPayload {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPayload")
	ret0, _ := ret[0].(*types.KeyPayload)
	return ret0
}

// GetPayload indicates an expected call of GetPayload
func (mr *MockStoredKeyMockRecorder) GetPayload() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPayload", reflect.TypeOf((*MockStoredKey)(nil).GetPayload))
}

// Unlock mocks base method
func (m *MockStoredKey) Unlock(passphrase string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unlock", passphrase)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unlock indicates an expected call of Unlock
func (mr *MockStoredKeyMockRecorder) Unlock(passphrase interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockStoredKey)(nil).Unlock), passphrase)
}

// GetFilename mocks base method
func (m *MockStoredKey) GetFilename() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetFilename")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetFilename indicates an expected call of GetFilename
func (mr *MockStoredKeyMockRecorder) GetFilename() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFilename", reflect.TypeOf((*MockStoredKey)(nil).GetFilename))
}

// GetUserAddress mocks base method
func (m *MockStoredKey) GetUserAddress() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserAddress")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetUserAddress indicates an expected call of GetUserAddress
func (mr *MockStoredKeyMockRecorder) GetUserAddress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserAddress", reflect.TypeOf((*MockStoredKey)(nil).GetUserAddress))
}

// GetPushKeyAddress mocks base method
func (m *MockStoredKey) GetPushKeyAddress() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPushKeyAddress")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetPushKeyAddress indicates an expected call of GetPushKeyAddress
func (mr *MockStoredKeyMockRecorder) GetPushKeyAddress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPushKeyAddress", reflect.TypeOf((*MockStoredKey)(nil).GetPushKeyAddress))
}

// IsUnprotected mocks base method
func (m *MockStoredKey) IsUnprotected() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsUnprotected")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsUnprotected indicates an expected call of IsUnprotected
func (mr *MockStoredKeyMockRecorder) IsUnprotected() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsUnprotected", reflect.TypeOf((*MockStoredKey)(nil).IsUnprotected))
}

// GetType mocks base method
func (m *MockStoredKey) GetType() types.KeyType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetType")
	ret0, _ := ret[0].(types.KeyType)
	return ret0
}

// GetType indicates an expected call of GetType
func (mr *MockStoredKeyMockRecorder) GetType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetType", reflect.TypeOf((*MockStoredKey)(nil).GetType))
}

// GetUnlockedData mocks base method
func (m *MockStoredKey) GetUnlockedData() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnlockedData")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// GetUnlockedData indicates an expected call of GetUnlockedData
func (mr *MockStoredKeyMockRecorder) GetUnlockedData() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnlockedData", reflect.TypeOf((*MockStoredKey)(nil).GetUnlockedData))
}

// GetCreatedAt mocks base method
func (m *MockStoredKey) GetCreatedAt() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCreatedAt")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// GetCreatedAt indicates an expected call of GetCreatedAt
func (mr *MockStoredKeyMockRecorder) GetCreatedAt() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCreatedAt", reflect.TypeOf((*MockStoredKey)(nil).GetCreatedAt))
}

// MockKeystore is a mock of Keystore interface
type MockKeystore struct {
	ctrl     *gomock.Controller
	recorder *MockKeystoreMockRecorder
}

// MockKeystoreMockRecorder is the mock recorder for MockKeystore
type MockKeystoreMockRecorder struct {
	mock *MockKeystore
}

// NewMockKeystore creates a new mock instance
func NewMockKeystore(ctrl *gomock.Controller) *MockKeystore {
	mock := &MockKeystore{ctrl: ctrl}
	mock.recorder = &MockKeystoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKeystore) EXPECT() *MockKeystoreMockRecorder {
	return m.recorder
}

// SetOutput mocks base method
func (m *MockKeystore) SetOutput(out io.Writer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetOutput", out)
}

// SetOutput indicates an expected call of SetOutput
func (mr *MockKeystoreMockRecorder) SetOutput(out interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetOutput", reflect.TypeOf((*MockKeystore)(nil).SetOutput), out)
}

// AskForPassword mocks base method
func (m *MockKeystore) AskForPassword(prompt ...string) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range prompt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AskForPassword", varargs...)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AskForPassword indicates an expected call of AskForPassword
func (mr *MockKeystoreMockRecorder) AskForPassword(prompt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AskForPassword", reflect.TypeOf((*MockKeystore)(nil).AskForPassword), prompt...)
}

// AskForPasswordOnce mocks base method
func (m *MockKeystore) AskForPasswordOnce(prompt ...string) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range prompt {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AskForPasswordOnce", varargs...)
	ret0, _ := ret[0].(string)
	return ret0, nil
}

// AskForPasswordOnce indicates an expected call of AskForPasswordOnce
func (mr *MockKeystoreMockRecorder) AskForPasswordOnce(prompt ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AskForPasswordOnce", reflect.TypeOf((*MockKeystore)(nil).AskForPasswordOnce), prompt...)
}

// UnlockKeyUI mocks base method
func (m *MockKeystore) UnlockKeyUI(addressOrIndex, passphrase, promptMsg string) (types.StoredKey, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnlockKeyUI", addressOrIndex, passphrase, promptMsg)
	ret0, _ := ret[0].(types.StoredKey)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// UnlockKeyUI indicates an expected call of UnlockKeyUI
func (mr *MockKeystoreMockRecorder) UnlockKeyUI(addressOrIndex, passphrase, promptMsg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnlockKeyUI", reflect.TypeOf((*MockKeystore)(nil).UnlockKeyUI), addressOrIndex, passphrase, promptMsg)
}

// UpdateCmd mocks base method
func (m *MockKeystore) UpdateCmd(addressOrIndex, passphrase string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateCmd", addressOrIndex, passphrase)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateCmd indicates an expected call of UpdateCmd
func (mr *MockKeystoreMockRecorder) UpdateCmd(addressOrIndex, passphrase interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCmd", reflect.TypeOf((*MockKeystore)(nil).UpdateCmd), addressOrIndex, passphrase)
}

// GetCmd mocks base method
func (m *MockKeystore) GetCmd(addrOrIdx, pass string, showPrivKey bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCmd", addrOrIdx, pass, showPrivKey)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetCmd indicates an expected call of GetCmd
func (mr *MockKeystoreMockRecorder) GetCmd(addrOrIdx, pass, showPrivKey interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCmd", reflect.TypeOf((*MockKeystore)(nil).GetCmd), addrOrIdx, pass, showPrivKey)
}

// ImportCmd mocks base method
func (m *MockKeystore) ImportCmd(keyfile string, keyType types.KeyType, pass string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ImportCmd", keyfile, keyType, pass)
	ret0, _ := ret[0].(error)
	return ret0
}

// ImportCmd indicates an expected call of ImportCmd
func (mr *MockKeystoreMockRecorder) ImportCmd(keyfile, keyType, pass interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ImportCmd", reflect.TypeOf((*MockKeystore)(nil).ImportCmd), keyfile, keyType, pass)
}

// Exist mocks base method
func (m *MockKeystore) Exist(address string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exist", address)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exist indicates an expected call of Exist
func (mr *MockKeystoreMockRecorder) Exist(address interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exist", reflect.TypeOf((*MockKeystore)(nil).Exist), address)
}

// GetByIndex mocks base method
func (m *MockKeystore) GetByIndex(i int) (types.StoredKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByIndex", i)
	ret0, _ := ret[0].(types.StoredKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByIndex indicates an expected call of GetByIndex
func (mr *MockKeystoreMockRecorder) GetByIndex(i interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByIndex", reflect.TypeOf((*MockKeystore)(nil).GetByIndex), i)
}

// GetByIndexOrAddress mocks base method
func (m *MockKeystore) GetByIndexOrAddress(idxOrAddr string) (types.StoredKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByIndexOrAddress", idxOrAddr)
	ret0, _ := ret[0].(types.StoredKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByIndexOrAddress indicates an expected call of GetByIndexOrAddress
func (mr *MockKeystoreMockRecorder) GetByIndexOrAddress(idxOrAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByIndexOrAddress", reflect.TypeOf((*MockKeystore)(nil).GetByIndexOrAddress), idxOrAddr)
}

// GetByAddress mocks base method
func (m *MockKeystore) GetByAddress(addr string) (types.StoredKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByAddress", addr)
	ret0, _ := ret[0].(types.StoredKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByAddress indicates an expected call of GetByAddress
func (mr *MockKeystoreMockRecorder) GetByAddress(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByAddress", reflect.TypeOf((*MockKeystore)(nil).GetByAddress), addr)
}

// CreateKey mocks base method
func (m *MockKeystore) CreateKey(key *crypto.Key, keyType types.KeyType, passphrase string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateKey", key, keyType, passphrase)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateKey indicates an expected call of CreateKey
func (mr *MockKeystoreMockRecorder) CreateKey(key, keyType, passphrase interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateKey", reflect.TypeOf((*MockKeystore)(nil).CreateKey), key, keyType, passphrase)
}

// CreateCmd mocks base method
func (m *MockKeystore) CreateCmd(keyType types.KeyType, seed int64, passphrase string, nopass bool) (*crypto.Key, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCmd", keyType, seed, passphrase, nopass)
	ret0, _ := ret[0].(*crypto.Key)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateCmd indicates an expected call of CreateCmd
func (mr *MockKeystoreMockRecorder) CreateCmd(keyType, seed, passphrase, nopass interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCmd", reflect.TypeOf((*MockKeystore)(nil).CreateCmd), keyType, seed, passphrase, nopass)
}

// List mocks base method
func (m *MockKeystore) List() ([]types.StoredKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]types.StoredKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List
func (mr *MockKeystoreMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockKeystore)(nil).List))
}

// ListCmd mocks base method
func (m *MockKeystore) ListCmd(out io.Writer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListCmd", out)
	ret0, _ := ret[0].(error)
	return ret0
}

// ListCmd indicates an expected call of ListCmd
func (mr *MockKeystoreMockRecorder) ListCmd(out interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListCmd", reflect.TypeOf((*MockKeystore)(nil).ListCmd), out)
}
