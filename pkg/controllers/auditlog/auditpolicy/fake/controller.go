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
	Objs map[string]map[string]auditlogv1.AuditPolicy
}

func (m *MockController) AddGenericHandler(context.Context, string, generic.Handler) {
	panic("unimplemented")
}

func (m *MockController) AddGenericRemoveHandler(context.Context, string, generic.Handler) {
	panic("unimplemented")
}

func (m *MockController) Cache() generic.CacheInterface[*auditlogv1.AuditPolicy] {
	panic("unimplemented")
}

func (m *MockController) Create(*auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	panic("unimplemented")
}

func (m *MockController) Delete(string, string, *metav1.DeleteOptions) error {
	panic("unimplemented")
}

func (m *MockController) Enqueue(namespace, name string) {
	panic("unimplemented")
}

// EnqueueAfter implements v1.AuditLogController.
func (m *MockController) EnqueueAfter(namespace string, name string, duration time.Duration) {
	panic("unimplemented")
}

// Get implements v1.AuditPolicyController.
func (m *MockController) Get(namespace string, name string, options metav1.GetOptions) (*auditlogv1.AuditPolicy, error) {
	ns, ok := m.Objs[namespace]
	if !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource("auditpolicy"), fmt.Sprintf("%s/%s", namespace, name))
	}

	obj, ok := ns[name]
	if !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource("auditpolicy"), fmt.Sprintf("%s/%s", namespace, name))
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
func (m *MockController) List(namespace string, opts metav1.ListOptions) (*auditlogv1.AuditPolicyList, error) {
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
func (m *MockController) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *auditlogv1.AuditPolicy, err error) {
	panic("unimplemented")
}

// Update implements v1.AuditPolicyController.
func (m *MockController) Update(*auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	panic("unimplemented")
}

// UpdateStatus implements v1.AuditPolicyController.
func (m *MockController) UpdateStatus(obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	ns, ok := m.Objs[obj.Namespace]
	if !ok {
		return nil, errors.NewNotFound(auditlogv1.Resource("auditpolicy"), fmt.Sprintf("%s/%s", obj.Namespace, obj.Name))
	}

	ns[obj.Name] = *obj

	return obj, nil
}

// Updater implements v1.AuditPolicyController.
func (m *MockController) Updater() generic.Updater {
	panic("unimplemented")
}

// Watch implements v1.AuditPolicyController.
func (m *MockController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	panic("unimplemented")
}

// WithImpersonation implements v1.AuditPolicyController.
func (m *MockController) WithImpersonation(impersonate rest.ImpersonationConfig) (generic.ClientInterface[*auditlogv1.AuditPolicy, *auditlogv1.AuditPolicyList], error) {
	panic("unimplemented")
}
