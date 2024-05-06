package healthsyncer

import (
	"context"
	"net"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func TestIsAPIUp(t *testing.T) {
	type args struct {
		ctx context.Context
		ns  typedv1.NamespaceInterface
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "api-up",
			args: args{
				ctx: context.Background(),
				ns: mockNamespaces{
					getter: func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
						return &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}, nil
					},
				},
			},
			wantErr: false,
		},
		{
			name: "api-down",
			args: args{
				ctx: context.Background(),
				ns: mockNamespaces{
					getter: func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
						return nil, net.UnknownNetworkError("unknown network error")
					},
				},
			},
			wantErr: true,
		},
		{
			name: "api-timeout",
			args: args{
				ctx: context.Background(),
				ns: mockNamespaces{
					getter: func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
						return nil, context.DeadlineExceeded
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := IsAPIUp(tt.args.ctx, tt.args.ns); (err != nil) != tt.wantErr {
				t.Errorf("IsAPIUp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type mockNamespaces struct {
	getter func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error)
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
	panic("implement me")
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
