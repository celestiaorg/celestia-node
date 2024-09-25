// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/celestiaorg/celestia-node/nodebuilder/blob (interfaces: Module)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	blob "github.com/celestiaorg/celestia-node/blob"
	state "github.com/celestiaorg/celestia-node/state"
	share "github.com/celestiaorg/go-square/v2/share"
	gomock "github.com/golang/mock/gomock"
)

// MockModule is a mock of Module interface.
type MockModule struct {
	ctrl     *gomock.Controller
	recorder *MockModuleMockRecorder
}

// MockModuleMockRecorder is the mock recorder for MockModule.
type MockModuleMockRecorder struct {
	mock *MockModule
}

// NewMockModule creates a new mock instance.
func NewMockModule(ctrl *gomock.Controller) *MockModule {
	mock := &MockModule{ctrl: ctrl}
	mock.recorder = &MockModuleMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockModule) EXPECT() *MockModuleMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockModule) Get(arg0 context.Context, arg1 uint64, arg2 share.Namespace, arg3 blob.Commitment) (*blob.Blob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*blob.Blob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockModuleMockRecorder) Get(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockModule)(nil).Get), arg0, arg1, arg2, arg3)
}

// GetAll mocks base method.
func (m *MockModule) GetAll(arg0 context.Context, arg1 uint64, arg2 []share.Namespace) ([]*blob.Blob, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAll", arg0, arg1, arg2)
	ret0, _ := ret[0].([]*blob.Blob)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAll indicates an expected call of GetAll.
func (mr *MockModuleMockRecorder) GetAll(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockModule)(nil).GetAll), arg0, arg1, arg2)
}

// GetCommitmentProof mocks base method.
func (m *MockModule) GetCommitmentProof(arg0 context.Context, arg1 uint64, arg2 share.Namespace, arg3 []byte) (*blob.CommitmentProof, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCommitmentProof", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*blob.CommitmentProof)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCommitmentProof indicates an expected call of GetCommitmentProof.
func (mr *MockModuleMockRecorder) GetCommitmentProof(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCommitmentProof", reflect.TypeOf((*MockModule)(nil).GetCommitmentProof), arg0, arg1, arg2, arg3)
}

// GetProof mocks base method.
func (m *MockModule) GetProof(arg0 context.Context, arg1 uint64, arg2 share.Namespace, arg3 blob.Commitment) (*blob.Proof, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProof", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*blob.Proof)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetProof indicates an expected call of GetProof.
func (mr *MockModuleMockRecorder) GetProof(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProof", reflect.TypeOf((*MockModule)(nil).GetProof), arg0, arg1, arg2, arg3)
}

// Included mocks base method.
func (m *MockModule) Included(arg0 context.Context, arg1 uint64, arg2 share.Namespace, arg3 *blob.Proof, arg4 blob.Commitment) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Included", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Included indicates an expected call of Included.
func (mr *MockModuleMockRecorder) Included(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Included", reflect.TypeOf((*MockModule)(nil).Included), arg0, arg1, arg2, arg3, arg4)
}

// Submit mocks base method.
func (m *MockModule) Submit(arg0 context.Context, arg1 []*blob.Blob, arg2 *state.TxConfig) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Submit", arg0, arg1, arg2)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Submit indicates an expected call of Submit.
func (mr *MockModuleMockRecorder) Submit(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Submit", reflect.TypeOf((*MockModule)(nil).Submit), arg0, arg1, arg2)
}

// Subscribe mocks base method.
func (m *MockModule) Subscribe(arg0 context.Context, arg1 share.Namespace) (<-chan *blob.SubscriptionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", arg0, arg1)
	ret0, _ := ret[0].(<-chan *blob.SubscriptionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockModuleMockRecorder) Subscribe(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockModule)(nil).Subscribe), arg0, arg1)
}
