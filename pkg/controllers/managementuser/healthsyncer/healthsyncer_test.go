package healthsyncer

import (
	"context"
	"net"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestIsAPIUp(t *testing.T) {
	type args struct {
		ctx context.Context
		k8s func() kubernetes.Interface
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
				k8s: func() kubernetes.Interface {
					return fake.NewSimpleClientset(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
				},
			},
			wantErr: false,
		},
		{
			name: "api-down",
			args: args{
				ctx: context.Background(),
				k8s: func() kubernetes.Interface {
					result := fake.NewSimpleClientset(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
					result.PrependReactor("get", "namespaces", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, net.UnknownNetworkError("unknown network error")
					})
					return result
				},
			},
			wantErr: true,
		},
		{
			name: "api-timeout",
			args: args{
				ctx: context.Background(),
				k8s: func() kubernetes.Interface {
					result := fake.NewSimpleClientset()
					result.PrependReactor("get", "namespaces", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, context.DeadlineExceeded
					})
					return result
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := IsAPIUp(tt.args.ctx, tt.args.k8s()); (err != nil) != tt.wantErr {
				t.Errorf("IsAPIUp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
