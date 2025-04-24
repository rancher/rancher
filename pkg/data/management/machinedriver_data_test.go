package management

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_removeMachineDriverByURLPrefix(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakes.NodeDriverInterfaceMock
		prefix     string
		wantDelete bool
	}{
		{
			name: "inactive_match_delete",
			client: &fakes.NodeDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.NodeDriver, error) {
					return &v3.NodeDriver{
						Spec: v3.NodeDriverSpec{
							Active: false,
							URL:    "https://foo.test/foo/docker-machine-driver-foo",
						},
					}, nil
				},
				DeleteFunc: func(name string, options *v1.DeleteOptions) error {
					return nil
				},
			},
			prefix:     "https://foo.test",
			wantDelete: true,
		},
		{
			name: "inactive_nomatch_nodelete",
			client: &fakes.NodeDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.NodeDriver, error) {
					return &v3.NodeDriver{
						Spec: v3.NodeDriverSpec{
							Active: false,
							URL:    "https://bar.test/foo/docker-machine-driver-foo",
						},
					}, nil
				},
				DeleteFunc: func(name string, options *v1.DeleteOptions) error {
					return nil
				},
			},
			prefix: "https://foo.test",
		},
		{
			name: "active_match_nodelete",
			client: &fakes.NodeDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.NodeDriver, error) {
					return &v3.NodeDriver{
						Spec: v3.NodeDriverSpec{
							Active: true,
							URL:    "https://foo.test/foo/docker-machine-driver-foo",
						},
					}, nil
				},
				DeleteFunc: func(name string, options *v1.DeleteOptions) error {
					return nil
				},
			},
			prefix: "https://foo.test",
		},
		{
			name: "get_notfound_nodelete",
			client: &fakes.NodeDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.NodeDriver, error) {
					return nil, errors.NewNotFound(schema.GroupResource{}, "")
				},
				DeleteFunc: func(name string, options *v1.DeleteOptions) error {
					return nil
				},
			},
			prefix: "https://foo.test",
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			deleteMachineDriver("", tt.prefix, tt.client)

			if tt.wantDelete {
				assert.Equal(t, 1, len(tt.client.DeleteCalls()))
			} else {
				assert.Equal(t, 0, len(tt.client.DeleteCalls()))
			}
		})
	}
}
