package rbac

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_createOrUpdateProjectNS(t *testing.T) {
	fakeDB := make(map[string]*v1.Namespace)
	namespaces := &corefakes.NamespaceInterfaceMock{
		CreateFunc: func(in *v1.Namespace) (*v1.Namespace, error) {
			if _, ok := fakeDB[in.Name]; ok {
				return nil, apierrors.NewAlreadyExists(schema.GroupResource{
					Resource: "namespaces",
				}, in.Name)
			}
			fakeDB[in.Name] = in
			return in, nil
		},
	}
	nsLister := &corefakes.NamespaceListerMock{
		GetFunc: func(_, name string) (*v1.Namespace, error) {
			if ns, ok := fakeDB[name]; ok {
				return ns, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{
				Resource: "namespaces",
			}, name)
		},
	}
	m := &manager{
		namespaces: namespaces,
		nsLister:   nsLister,
	}

	p := newProjectLifecycle(m)
	err := p.ensureProjectNS(&v3.Project{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
	assert.Nil(t, err)
	assert.NotNil(t, fakeDB["test"])
	err = p.ensureProjectNS(&v3.Project{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
	assert.Nil(t, err)
	fakeDB["test-existing"] = &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-existing"}}
	err = p.ensureProjectNS(&v3.Project{ObjectMeta: metav1.ObjectMeta{Name: "test-existing"}})
	assert.Error(t, err)
}
