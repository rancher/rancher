package project_cluster

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

var (
	errDefault    = fmt.Errorf("error")
	errNsNotFound = apierrors.NewNotFound(v1.Resource("namespace"), "")

	defaultNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-namespace",
		},
	}
	terminatingNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "terminating-namespace",
		},
		Status: v1.NamespaceStatus{
			Phase: v1.NamespaceTerminating,
		},
	}
)

func Test_deleteNamespace(t *testing.T) {
	tests := []struct {
		name         string
		nsGetFunc    func(context.Context, string, metav1.GetOptions) (*v1.Namespace, error)
		nsDeleteFunc func(context.Context, string, metav1.DeleteOptions) error
		wantErr      bool
	}{
		{
			name: "error getting namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errDefault
			},
			wantErr: true,
		},
		{
			name: "namespace not found",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errNsNotFound
			},
		},
		{
			name: "namespace is terminating",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return terminatingNamespace.DeepCopy(), nil
			},
		},
		{
			name: "successfully delete namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return nil
			},
		},
		{
			name: "deleting namespace not found",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return errNsNotFound
			},
		},
		{
			name: "error deleting namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return errDefault
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsClientFake := mockNamespaces{
				getter:  tt.nsGetFunc,
				deleter: tt.nsDeleteFunc,
			}
			if err := deleteNamespace("", "", nsClientFake); (err != nil) != tt.wantErr {
				t.Errorf("deleteNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type mockNamespaces struct {
	getter  func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error)
	deleter func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (m mockNamespaces) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	return m.getter(ctx, name, opts)
}

func (m mockNamespaces) Create(ctx context.Context, namespace *v1.Namespace, opts metav1.CreateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Update(ctx context.Context, namespace *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) UpdateStatus(ctx context.Context, namespace *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return m.deleter(ctx, name, opts)
}

func (m mockNamespaces) List(ctx context.Context, opts metav1.ListOptions) (*v1.NamespaceList, error) {
	panic("implement me")
}

func (m mockNamespaces) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m mockNamespaces) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) Apply(ctx context.Context, namespace *applycorev1.NamespaceApplyConfiguration, opts metav1.ApplyOptions) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) ApplyStatus(ctx context.Context, namespace *applycorev1.NamespaceApplyConfiguration, opts metav1.ApplyOptions) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) Finalize(ctx context.Context, item *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}
