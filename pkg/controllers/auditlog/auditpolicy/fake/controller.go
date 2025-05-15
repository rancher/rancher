package fake

import (
	"context"
	"fmt"
	"time"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var _ v1.AuditPolicyController = &MockController{}

type MockController struct {
	Objs map[string]auditlogv1.AuditPolicy
}

func (m *MockController) AddGenericHandler(context.Context, string, generic.Handler) {
	panic("unimplemented")
}

func (m *MockController) AddGenericRemoveHandler(context.Context, string, generic.Handler) {
	panic("unimplemented")
}

func (m *MockController) Cache() generic.NonNamespacedCacheInterface[*auditlogv1.AuditPolicy] {
	panic("unimplemented")
}

func (m *MockController) Create(obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if _, ok := m.Objs[obj.Name]; ok {
		return nil, errors.NewAlreadyExists(auditlogv1.Resource(auditlogv1.AuditPolicyResourceName), obj.Name)
	}

	m.Objs[obj.Name] = *obj

	return obj, nil
}

func (m *MockController) Delete(string, *metav1.DeleteOptions) error {
	panic("unimplemented")
}

func (m *MockController) Enqueue(string) {
	panic("unimplemented")
}

// EnqueueAfter implements v1.AuditLogController.
func (m *MockController) EnqueueAfter(string, time.Duration) {
	panic("unimplemented")
}

// Get implements v1.AuditPolicyController.
func (m *MockController) Get(name string, options metav1.GetOptions) (*auditlogv1.AuditPolicy, error) {
	obj, ok := m.Objs[name]
	if !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource(auditlogv1.AuditPolicyResourceName), name)
	}

	return &obj, nil
}

// GroupVersionKind implements v1.AuditPolicyController.
func (m *MockController) GroupVersionKind() schema.GroupVersionKind {
	panic("unimplemented")
}

// Informer implements v1.AuditPolicyController.
func (m *MockController) Informer() cache.SharedIndexInformer {
	panic("unimplemented")
}

// List implements v1.AuditPolicyController.
func (m *MockController) List(opts metav1.ListOptions) (*auditlogv1.AuditPolicyList, error) {
	panic("unimplemented")
}

// OnChange implements v1.AuditPolicyController.
func (m *MockController) OnChange(ctx context.Context, name string, sync generic.ObjectHandler[*auditlogv1.AuditPolicy]) {
	panic("unimplemented")
}

// OnRemove implements v1.AuditPolicyController.
func (m *MockController) OnRemove(ctx context.Context, name string, sync generic.ObjectHandler[*auditlogv1.AuditPolicy]) {
	panic("unimplemented")
}

// Patch implements v1.AuditPolicyController.
func (m *MockController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *auditlogv1.AuditPolicy, err error) {
	panic("unimplemented")
}

// Update implements v1.AuditPolicyController.
func (m *MockController) Update(obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if _, ok := m.Objs[obj.Name]; !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource(auditlogv1.AuditPolicyResourceName), obj.Name)
	}

	m.Objs[obj.Name] = *obj

	return obj, nil
}

// UpdateStatus implements v1.AuditPolicyController.
func (m *MockController) UpdateStatus(obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	existing, ok := m.Objs[obj.Name]
	if !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource(auditlogv1.AuditPolicyResourceName), fmt.Sprintf("%s/%s", obj.Namespace, obj.Name))
	}

	existing.Status = obj.Status

	m.Objs[obj.Name] = existing

	return obj, nil
}

// Updater implements v1.AuditPolicyController.
func (m *MockController) Updater() generic.Updater {
	panic("unimplemented")
}

// Watch implements v1.AuditPolicyController.
func (m *MockController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("unimplemented")
}

// WithImpersonation implements v1.AuditPolicyController.
func (m *MockController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*auditlogv1.AuditPolicy, *auditlogv1.AuditPolicyList], error) {
	panic("unimplemented")
}
