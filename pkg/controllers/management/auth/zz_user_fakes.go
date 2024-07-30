// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3 (interfaces: UserInterface)

// Package auth is a generated GoMock package.
package auth

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	objectclient "github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v30 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
)

// MockUserInterface is a mock of UserInterface interface.
type MockUserInterface struct {
	ctrl     *gomock.Controller
	recorder *MockUserInterfaceMockRecorder
}

// MockUserInterfaceMockRecorder is the mock recorder for MockUserInterface.
type MockUserInterfaceMockRecorder struct {
	mock *MockUserInterface
}

// NewMockUserInterface creates a new mock instance.
func NewMockUserInterface(ctrl *gomock.Controller) *MockUserInterface {
	mock := &MockUserInterface{ctrl: ctrl}
	mock.recorder = &MockUserInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUserInterface) EXPECT() *MockUserInterfaceMockRecorder {
	return m.recorder
}

// AddClusterScopedFeatureHandler mocks base method.
func (m *MockUserInterface) AddClusterScopedFeatureHandler(arg0 context.Context, arg1 func() bool, arg2, arg3 string, arg4 v30.UserHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedFeatureHandler", arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureHandler indicates an expected call of AddClusterScopedFeatureHandler.
func (mr *MockUserInterfaceMockRecorder) AddClusterScopedFeatureHandler(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedFeatureHandler", reflect.TypeOf((*MockUserInterface)(nil).AddClusterScopedFeatureHandler), arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureLifecycle mocks base method.
func (m *MockUserInterface) AddClusterScopedFeatureLifecycle(arg0 context.Context, arg1 func() bool, arg2, arg3 string, arg4 v30.UserLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedFeatureLifecycle", arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureLifecycle indicates an expected call of AddClusterScopedFeatureLifecycle.
func (mr *MockUserInterfaceMockRecorder) AddClusterScopedFeatureLifecycle(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedFeatureLifecycle", reflect.TypeOf((*MockUserInterface)(nil).AddClusterScopedFeatureLifecycle), arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedHandler mocks base method.
func (m *MockUserInterface) AddClusterScopedHandler(arg0 context.Context, arg1, arg2 string, arg3 v30.UserHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedHandler", arg0, arg1, arg2, arg3)
}

// AddClusterScopedHandler indicates an expected call of AddClusterScopedHandler.
func (mr *MockUserInterfaceMockRecorder) AddClusterScopedHandler(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedHandler", reflect.TypeOf((*MockUserInterface)(nil).AddClusterScopedHandler), arg0, arg1, arg2, arg3)
}

// AddClusterScopedLifecycle mocks base method.
func (m *MockUserInterface) AddClusterScopedLifecycle(arg0 context.Context, arg1, arg2 string, arg3 v30.UserLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedLifecycle", arg0, arg1, arg2, arg3)
}

// AddClusterScopedLifecycle indicates an expected call of AddClusterScopedLifecycle.
func (mr *MockUserInterfaceMockRecorder) AddClusterScopedLifecycle(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedLifecycle", reflect.TypeOf((*MockUserInterface)(nil).AddClusterScopedLifecycle), arg0, arg1, arg2, arg3)
}

// AddFeatureHandler mocks base method.
func (m *MockUserInterface) AddFeatureHandler(arg0 context.Context, arg1 func() bool, arg2 string, arg3 v30.UserHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFeatureHandler", arg0, arg1, arg2, arg3)
}

// AddFeatureHandler indicates an expected call of AddFeatureHandler.
func (mr *MockUserInterfaceMockRecorder) AddFeatureHandler(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFeatureHandler", reflect.TypeOf((*MockUserInterface)(nil).AddFeatureHandler), arg0, arg1, arg2, arg3)
}

// AddFeatureLifecycle mocks base method.
func (m *MockUserInterface) AddFeatureLifecycle(arg0 context.Context, arg1 func() bool, arg2 string, arg3 v30.UserLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFeatureLifecycle", arg0, arg1, arg2, arg3)
}

// AddFeatureLifecycle indicates an expected call of AddFeatureLifecycle.
func (mr *MockUserInterfaceMockRecorder) AddFeatureLifecycle(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFeatureLifecycle", reflect.TypeOf((*MockUserInterface)(nil).AddFeatureLifecycle), arg0, arg1, arg2, arg3)
}

// AddHandler mocks base method.
func (m *MockUserInterface) AddHandler(arg0 context.Context, arg1 string, arg2 v30.UserHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddHandler", arg0, arg1, arg2)
}

// AddHandler indicates an expected call of AddHandler.
func (mr *MockUserInterfaceMockRecorder) AddHandler(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddHandler", reflect.TypeOf((*MockUserInterface)(nil).AddHandler), arg0, arg1, arg2)
}

// AddLifecycle mocks base method.
func (m *MockUserInterface) AddLifecycle(arg0 context.Context, arg1 string, arg2 v30.UserLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddLifecycle", arg0, arg1, arg2)
}

// AddLifecycle indicates an expected call of AddLifecycle.
func (mr *MockUserInterfaceMockRecorder) AddLifecycle(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLifecycle", reflect.TypeOf((*MockUserInterface)(nil).AddLifecycle), arg0, arg1, arg2)
}

// Controller mocks base method.
func (m *MockUserInterface) Controller() v30.UserController {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Controller")
	ret0, _ := ret[0].(v30.UserController)
	return ret0
}

// Controller indicates an expected call of Controller.
func (mr *MockUserInterfaceMockRecorder) Controller() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Controller", reflect.TypeOf((*MockUserInterface)(nil).Controller))
}

// Create mocks base method.
func (m *MockUserInterface) Create(arg0 *v3.User) (*v3.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0)
	ret0, _ := ret[0].(*v3.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockUserInterfaceMockRecorder) Create(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockUserInterface)(nil).Create), arg0)
}

// Delete mocks base method.
func (m *MockUserInterface) Delete(arg0 string, arg1 *v1.DeleteOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockUserInterfaceMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockUserInterface)(nil).Delete), arg0, arg1)
}

// DeleteCollection mocks base method.
func (m *MockUserInterface) DeleteCollection(arg0 *v1.DeleteOptions, arg1 v1.ListOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteCollection", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteCollection indicates an expected call of DeleteCollection.
func (mr *MockUserInterfaceMockRecorder) DeleteCollection(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCollection", reflect.TypeOf((*MockUserInterface)(nil).DeleteCollection), arg0, arg1)
}

// DeleteNamespaced mocks base method.
func (m *MockUserInterface) DeleteNamespaced(arg0, arg1 string, arg2 *v1.DeleteOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNamespaced", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNamespaced indicates an expected call of DeleteNamespaced.
func (mr *MockUserInterfaceMockRecorder) DeleteNamespaced(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNamespaced", reflect.TypeOf((*MockUserInterface)(nil).DeleteNamespaced), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockUserInterface) Get(arg0 string, arg1 v1.GetOptions) (*v3.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(*v3.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockUserInterfaceMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockUserInterface)(nil).Get), arg0, arg1)
}

// GetNamespaced mocks base method.
func (m *MockUserInterface) GetNamespaced(arg0, arg1 string, arg2 v1.GetOptions) (*v3.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNamespaced", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v3.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNamespaced indicates an expected call of GetNamespaced.
func (mr *MockUserInterfaceMockRecorder) GetNamespaced(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNamespaced", reflect.TypeOf((*MockUserInterface)(nil).GetNamespaced), arg0, arg1, arg2)
}

// List mocks base method.
func (m *MockUserInterface) List(arg0 v1.ListOptions) (*v3.UserList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].(*v3.UserList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockUserInterfaceMockRecorder) List(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockUserInterface)(nil).List), arg0)
}

// ListNamespaced mocks base method.
func (m *MockUserInterface) ListNamespaced(arg0 string, arg1 v1.ListOptions) (*v3.UserList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListNamespaced", arg0, arg1)
	ret0, _ := ret[0].(*v3.UserList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListNamespaced indicates an expected call of ListNamespaced.
func (mr *MockUserInterfaceMockRecorder) ListNamespaced(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListNamespaced", reflect.TypeOf((*MockUserInterface)(nil).ListNamespaced), arg0, arg1)
}

// ObjectClient mocks base method.
func (m *MockUserInterface) ObjectClient() *objectclient.ObjectClient {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ObjectClient")
	ret0, _ := ret[0].(*objectclient.ObjectClient)
	return ret0
}

// ObjectClient indicates an expected call of ObjectClient.
func (mr *MockUserInterfaceMockRecorder) ObjectClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObjectClient", reflect.TypeOf((*MockUserInterface)(nil).ObjectClient))
}

// Update mocks base method.
func (m *MockUserInterface) Update(arg0 *v3.User) (*v3.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0)
	ret0, _ := ret[0].(*v3.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockUserInterfaceMockRecorder) Update(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockUserInterface)(nil).Update), arg0)
}

// Watch mocks base method.
func (m *MockUserInterface) Watch(arg0 v1.ListOptions) (watch.Interface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Watch", arg0)
	ret0, _ := ret[0].(watch.Interface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Watch indicates an expected call of Watch.
func (mr *MockUserInterfaceMockRecorder) Watch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockUserInterface)(nil).Watch), arg0)
}
