// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/rancher/rancher/pkg/generated/norman/core/v1 (interfaces: NamespaceInterface)

// Package auth is a generated GoMock package.
package auth

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	objectclient "github.com/rancher/norman/objectclient"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v10 "k8s.io/api/core/v1"
	v11 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
)

// MockNamespaceInterface is a mock of NamespaceInterface interface.
type MockNamespaceInterface struct {
	ctrl     *gomock.Controller
	recorder *MockNamespaceInterfaceMockRecorder
}

// MockNamespaceInterfaceMockRecorder is the mock recorder for MockNamespaceInterface.
type MockNamespaceInterfaceMockRecorder struct {
	mock *MockNamespaceInterface
}

// NewMockNamespaceInterface creates a new mock instance.
func NewMockNamespaceInterface(ctrl *gomock.Controller) *MockNamespaceInterface {
	mock := &MockNamespaceInterface{ctrl: ctrl}
	mock.recorder = &MockNamespaceInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNamespaceInterface) EXPECT() *MockNamespaceInterfaceMockRecorder {
	return m.recorder
}

// AddClusterScopedFeatureHandler mocks base method.
func (m *MockNamespaceInterface) AddClusterScopedFeatureHandler(arg0 context.Context, arg1 func() bool, arg2, arg3 string, arg4 v1.NamespaceHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedFeatureHandler", arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureHandler indicates an expected call of AddClusterScopedFeatureHandler.
func (mr *MockNamespaceInterfaceMockRecorder) AddClusterScopedFeatureHandler(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedFeatureHandler", reflect.TypeOf((*MockNamespaceInterface)(nil).AddClusterScopedFeatureHandler), arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureLifecycle mocks base method.
func (m *MockNamespaceInterface) AddClusterScopedFeatureLifecycle(arg0 context.Context, arg1 func() bool, arg2, arg3 string, arg4 v1.NamespaceLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedFeatureLifecycle", arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedFeatureLifecycle indicates an expected call of AddClusterScopedFeatureLifecycle.
func (mr *MockNamespaceInterfaceMockRecorder) AddClusterScopedFeatureLifecycle(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedFeatureLifecycle", reflect.TypeOf((*MockNamespaceInterface)(nil).AddClusterScopedFeatureLifecycle), arg0, arg1, arg2, arg3, arg4)
}

// AddClusterScopedHandler mocks base method.
func (m *MockNamespaceInterface) AddClusterScopedHandler(arg0 context.Context, arg1, arg2 string, arg3 v1.NamespaceHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedHandler", arg0, arg1, arg2, arg3)
}

// AddClusterScopedHandler indicates an expected call of AddClusterScopedHandler.
func (mr *MockNamespaceInterfaceMockRecorder) AddClusterScopedHandler(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedHandler", reflect.TypeOf((*MockNamespaceInterface)(nil).AddClusterScopedHandler), arg0, arg1, arg2, arg3)
}

// AddClusterScopedLifecycle mocks base method.
func (m *MockNamespaceInterface) AddClusterScopedLifecycle(arg0 context.Context, arg1, arg2 string, arg3 v1.NamespaceLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddClusterScopedLifecycle", arg0, arg1, arg2, arg3)
}

// AddClusterScopedLifecycle indicates an expected call of AddClusterScopedLifecycle.
func (mr *MockNamespaceInterfaceMockRecorder) AddClusterScopedLifecycle(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddClusterScopedLifecycle", reflect.TypeOf((*MockNamespaceInterface)(nil).AddClusterScopedLifecycle), arg0, arg1, arg2, arg3)
}

// AddFeatureHandler mocks base method.
func (m *MockNamespaceInterface) AddFeatureHandler(arg0 context.Context, arg1 func() bool, arg2 string, arg3 v1.NamespaceHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFeatureHandler", arg0, arg1, arg2, arg3)
}

// AddFeatureHandler indicates an expected call of AddFeatureHandler.
func (mr *MockNamespaceInterfaceMockRecorder) AddFeatureHandler(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFeatureHandler", reflect.TypeOf((*MockNamespaceInterface)(nil).AddFeatureHandler), arg0, arg1, arg2, arg3)
}

// AddFeatureLifecycle mocks base method.
func (m *MockNamespaceInterface) AddFeatureLifecycle(arg0 context.Context, arg1 func() bool, arg2 string, arg3 v1.NamespaceLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFeatureLifecycle", arg0, arg1, arg2, arg3)
}

// AddFeatureLifecycle indicates an expected call of AddFeatureLifecycle.
func (mr *MockNamespaceInterfaceMockRecorder) AddFeatureLifecycle(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFeatureLifecycle", reflect.TypeOf((*MockNamespaceInterface)(nil).AddFeatureLifecycle), arg0, arg1, arg2, arg3)
}

// AddHandler mocks base method.
func (m *MockNamespaceInterface) AddHandler(arg0 context.Context, arg1 string, arg2 v1.NamespaceHandlerFunc) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddHandler", arg0, arg1, arg2)
}

// AddHandler indicates an expected call of AddHandler.
func (mr *MockNamespaceInterfaceMockRecorder) AddHandler(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddHandler", reflect.TypeOf((*MockNamespaceInterface)(nil).AddHandler), arg0, arg1, arg2)
}

// AddLifecycle mocks base method.
func (m *MockNamespaceInterface) AddLifecycle(arg0 context.Context, arg1 string, arg2 v1.NamespaceLifecycle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddLifecycle", arg0, arg1, arg2)
}

// AddLifecycle indicates an expected call of AddLifecycle.
func (mr *MockNamespaceInterfaceMockRecorder) AddLifecycle(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLifecycle", reflect.TypeOf((*MockNamespaceInterface)(nil).AddLifecycle), arg0, arg1, arg2)
}

// Controller mocks base method.
func (m *MockNamespaceInterface) Controller() v1.NamespaceController {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Controller")
	ret0, _ := ret[0].(v1.NamespaceController)
	return ret0
}

// Controller indicates an expected call of Controller.
func (mr *MockNamespaceInterfaceMockRecorder) Controller() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Controller", reflect.TypeOf((*MockNamespaceInterface)(nil).Controller))
}

// Create mocks base method.
func (m *MockNamespaceInterface) Create(arg0 *v10.Namespace) (*v10.Namespace, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0)
	ret0, _ := ret[0].(*v10.Namespace)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockNamespaceInterfaceMockRecorder) Create(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockNamespaceInterface)(nil).Create), arg0)
}

// Delete mocks base method.
func (m *MockNamespaceInterface) Delete(arg0 string, arg1 *v11.DeleteOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockNamespaceInterfaceMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockNamespaceInterface)(nil).Delete), arg0, arg1)
}

// DeleteCollection mocks base method.
func (m *MockNamespaceInterface) DeleteCollection(arg0 *v11.DeleteOptions, arg1 v11.ListOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteCollection", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteCollection indicates an expected call of DeleteCollection.
func (mr *MockNamespaceInterfaceMockRecorder) DeleteCollection(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCollection", reflect.TypeOf((*MockNamespaceInterface)(nil).DeleteCollection), arg0, arg1)
}

// DeleteNamespaced mocks base method.
func (m *MockNamespaceInterface) DeleteNamespaced(arg0, arg1 string, arg2 *v11.DeleteOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNamespaced", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNamespaced indicates an expected call of DeleteNamespaced.
func (mr *MockNamespaceInterfaceMockRecorder) DeleteNamespaced(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNamespaced", reflect.TypeOf((*MockNamespaceInterface)(nil).DeleteNamespaced), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockNamespaceInterface) Get(arg0 string, arg1 v11.GetOptions) (*v10.Namespace, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(*v10.Namespace)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockNamespaceInterfaceMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockNamespaceInterface)(nil).Get), arg0, arg1)
}

// GetNamespaced mocks base method.
func (m *MockNamespaceInterface) GetNamespaced(arg0, arg1 string, arg2 v11.GetOptions) (*v10.Namespace, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNamespaced", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v10.Namespace)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNamespaced indicates an expected call of GetNamespaced.
func (mr *MockNamespaceInterfaceMockRecorder) GetNamespaced(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNamespaced", reflect.TypeOf((*MockNamespaceInterface)(nil).GetNamespaced), arg0, arg1, arg2)
}

// List mocks base method.
func (m *MockNamespaceInterface) List(arg0 v11.ListOptions) (*v10.NamespaceList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].(*v10.NamespaceList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockNamespaceInterfaceMockRecorder) List(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockNamespaceInterface)(nil).List), arg0)
}

// ListNamespaced mocks base method.
func (m *MockNamespaceInterface) ListNamespaced(arg0 string, arg1 v11.ListOptions) (*v10.NamespaceList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListNamespaced", arg0, arg1)
	ret0, _ := ret[0].(*v10.NamespaceList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListNamespaced indicates an expected call of ListNamespaced.
func (mr *MockNamespaceInterfaceMockRecorder) ListNamespaced(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListNamespaced", reflect.TypeOf((*MockNamespaceInterface)(nil).ListNamespaced), arg0, arg1)
}

// ObjectClient mocks base method.
func (m *MockNamespaceInterface) ObjectClient() *objectclient.ObjectClient {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ObjectClient")
	ret0, _ := ret[0].(*objectclient.ObjectClient)
	return ret0
}

// ObjectClient indicates an expected call of ObjectClient.
func (mr *MockNamespaceInterfaceMockRecorder) ObjectClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ObjectClient", reflect.TypeOf((*MockNamespaceInterface)(nil).ObjectClient))
}

// Update mocks base method.
func (m *MockNamespaceInterface) Update(arg0 *v10.Namespace) (*v10.Namespace, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0)
	ret0, _ := ret[0].(*v10.Namespace)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockNamespaceInterfaceMockRecorder) Update(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockNamespaceInterface)(nil).Update), arg0)
}

// Watch mocks base method.
func (m *MockNamespaceInterface) Watch(arg0 v11.ListOptions) (watch.Interface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Watch", arg0)
	ret0, _ := ret[0].(watch.Interface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Watch indicates an expected call of Watch.
func (mr *MockNamespaceInterfaceMockRecorder) Watch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockNamespaceInterface)(nil).Watch), arg0)
}
