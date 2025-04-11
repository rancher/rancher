// Code generated by MockGen. DO NOT EDIT.
// Source: ../../controllers/management/oidcprovider/controller.go
//
// Generated by this command:
//
//	mockgen -source=../../controllers/management/oidcprovider/controller.go -destination=./strgenerator.go -package=mocks
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockClientIDAndSecretGenerator is a mock of ClientIDAndSecretGenerator interface.
type MockClientIDAndSecretGenerator struct {
	ctrl     *gomock.Controller
	recorder *MockClientIDAndSecretGeneratorMockRecorder
}

// MockClientIDAndSecretGeneratorMockRecorder is the mock recorder for MockClientIDAndSecretGenerator.
type MockClientIDAndSecretGeneratorMockRecorder struct {
	mock *MockClientIDAndSecretGenerator
}

// NewMockClientIDAndSecretGenerator creates a new mock instance.
func NewMockClientIDAndSecretGenerator(ctrl *gomock.Controller) *MockClientIDAndSecretGenerator {
	mock := &MockClientIDAndSecretGenerator{ctrl: ctrl}
	mock.recorder = &MockClientIDAndSecretGeneratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientIDAndSecretGenerator) EXPECT() *MockClientIDAndSecretGeneratorMockRecorder {
	return m.recorder
}

// GenerateClientID mocks base method.
func (m *MockClientIDAndSecretGenerator) GenerateClientID() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GenerateClientID")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GenerateClientID indicates an expected call of GenerateClientID.
func (mr *MockClientIDAndSecretGeneratorMockRecorder) GenerateClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GenerateClientID", reflect.TypeOf((*MockClientIDAndSecretGenerator)(nil).GenerateClientID))
}

// GenerateClientSecret mocks base method.
func (m *MockClientIDAndSecretGenerator) GenerateClientSecret() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GenerateClientSecret")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GenerateClientSecret indicates an expected call of GenerateClientSecret.
func (mr *MockClientIDAndSecretGeneratorMockRecorder) GenerateClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GenerateClientSecret", reflect.TypeOf((*MockClientIDAndSecretGenerator)(nil).GenerateClientSecret))
}
