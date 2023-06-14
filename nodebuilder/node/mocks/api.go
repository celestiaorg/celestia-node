// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/celestiaorg/celestia-node/nodebuilder/node (interfaces: Module)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	node "github.com/celestiaorg/celestia-node/nodebuilder/node"
	auth "github.com/filecoin-project/go-jsonrpc/auth"
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

// AuthNew mocks base method.
func (m *MockModule) AuthNew(arg0 context.Context, arg1 []auth.Permission) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AuthNew", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AuthNew indicates an expected call of AuthNew.
func (mr *MockModuleMockRecorder) AuthNew(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AuthNew", reflect.TypeOf((*MockModule)(nil).AuthNew), arg0, arg1)
}

// AuthVerify mocks base method.
func (m *MockModule) AuthVerify(arg0 context.Context, arg1 string) ([]auth.Permission, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AuthVerify", arg0, arg1)
	ret0, _ := ret[0].([]auth.Permission)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AuthVerify indicates an expected call of AuthVerify.
func (mr *MockModuleMockRecorder) AuthVerify(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AuthVerify", reflect.TypeOf((*MockModule)(nil).AuthVerify), arg0, arg1)
}

// Info mocks base method.
func (m *MockModule) Info(arg0 context.Context) (node.Info, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info", arg0)
	ret0, _ := ret[0].(node.Info)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Info indicates an expected call of Info.
func (mr *MockModuleMockRecorder) Info(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockModule)(nil).Info), arg0)
}

// LogLevelSet mocks base method.
func (m *MockModule) LogLevelSet(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogLevelSet", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// LogLevelSet indicates an expected call of LogLevelSet.
func (mr *MockModuleMockRecorder) LogLevelSet(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogLevelSet", reflect.TypeOf((*MockModule)(nil).LogLevelSet), arg0, arg1, arg2)
}
