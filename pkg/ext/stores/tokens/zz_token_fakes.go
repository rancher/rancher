// Code generated by MockGen. DO NOT EDIT.
// Source: tokens.go
//
// Generated by this command:
//
//	mockgen -source tokens.go -destination=zz_token_fakes.go -package=tokens
//

// Package tokens is a generated GoMock package.
package tokens

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	user "k8s.io/apiserver/pkg/authentication/user"
)

// MocktimeHandler is a mock of timeHandler interface.
type MocktimeHandler struct {
	ctrl     *gomock.Controller
	recorder *MocktimeHandlerMockRecorder
}

// MocktimeHandlerMockRecorder is the mock recorder for MocktimeHandler.
type MocktimeHandlerMockRecorder struct {
	mock *MocktimeHandler
}

// NewMocktimeHandler creates a new mock instance.
func NewMocktimeHandler(ctrl *gomock.Controller) *MocktimeHandler {
	mock := &MocktimeHandler{ctrl: ctrl}
	mock.recorder = &MocktimeHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MocktimeHandler) EXPECT() *MocktimeHandlerMockRecorder {
	return m.recorder
}

// Now mocks base method.
func (m *MocktimeHandler) Now() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Now")
	ret0, _ := ret[0].(string)
	return ret0
}

// Now indicates an expected call of Now.
func (mr *MocktimeHandlerMockRecorder) Now() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Now", reflect.TypeOf((*MocktimeHandler)(nil).Now))
}

// MockhashHandler is a mock of hashHandler interface.
type MockhashHandler struct {
	ctrl     *gomock.Controller
	recorder *MockhashHandlerMockRecorder
}

// MockhashHandlerMockRecorder is the mock recorder for MockhashHandler.
type MockhashHandlerMockRecorder struct {
	mock *MockhashHandler
}

// NewMockhashHandler creates a new mock instance.
func NewMockhashHandler(ctrl *gomock.Controller) *MockhashHandler {
	mock := &MockhashHandler{ctrl: ctrl}
	mock.recorder = &MockhashHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockhashHandler) EXPECT() *MockhashHandlerMockRecorder {
	return m.recorder
}

// MakeAndHashSecret mocks base method.
func (m *MockhashHandler) MakeAndHashSecret() (string, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MakeAndHashSecret")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// MakeAndHashSecret indicates an expected call of MakeAndHashSecret.
func (mr *MockhashHandlerMockRecorder) MakeAndHashSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MakeAndHashSecret", reflect.TypeOf((*MockhashHandler)(nil).MakeAndHashSecret))
}

// MockauthHandler is a mock of authHandler interface.
type MockauthHandler struct {
	ctrl     *gomock.Controller
	recorder *MockauthHandlerMockRecorder
}

// MockauthHandlerMockRecorder is the mock recorder for MockauthHandler.
type MockauthHandlerMockRecorder struct {
	mock *MockauthHandler
}

// NewMockauthHandler creates a new mock instance.
func NewMockauthHandler(ctrl *gomock.Controller) *MockauthHandler {
	mock := &MockauthHandler{ctrl: ctrl}
	mock.recorder = &MockauthHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockauthHandler) EXPECT() *MockauthHandlerMockRecorder {
	return m.recorder
}

// SessionID mocks base method.
func (m *MockauthHandler) SessionID(ctx context.Context) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SessionID", ctx)
	ret0, _ := ret[0].(string)
	return ret0
}

// SessionID indicates an expected call of SessionID.
func (mr *MockauthHandlerMockRecorder) SessionID(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SessionID", reflect.TypeOf((*MockauthHandler)(nil).SessionID), ctx)
}

// UserName mocks base method.
func (m *MockauthHandler) UserName(ctx context.Context, store *SystemStore, verb string) (user.Info, bool, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UserName", ctx, store, verb)
	ret0, _ := ret[0].(user.Info)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(bool)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// UserName indicates an expected call of UserName.
func (mr *MockauthHandlerMockRecorder) UserName(ctx, store, verb any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UserName", reflect.TypeOf((*MockauthHandler)(nil).UserName), ctx, store, verb)
}
