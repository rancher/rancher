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

func Test_removeKontainerDriverByURLPrefix(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakes.KontainerDriverInterfaceMock
		prefix     string
		wantDelete bool
	}{
		{
			name: "inactive_match_delete",
			client: &fakes.KontainerDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.KontainerDriver, error) {
					return &v3.KontainerDriver{
						Spec: v3.KontainerDriverSpec{
							Active: false,
							URL:    "https://foo.test/foo/kontainer-engine-driver-foo",
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
			client: &fakes.KontainerDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.KontainerDriver, error) {
					return &v3.KontainerDriver{
						Spec: v3.KontainerDriverSpec{
							Active: false,
							URL:    "https://bar.test/foo/kontainer-engine-driver-foo",
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
			client: &fakes.KontainerDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.KontainerDriver, error) {
					return &v3.KontainerDriver{
						Spec: v3.KontainerDriverSpec{
							Active: true,
							URL:    "https://foo.test/foo/kontainer-engine-driver-foo",
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
			client: &fakes.KontainerDriverInterfaceMock{
				GetFunc: func(name string, opts v1.GetOptions) (*v3.KontainerDriver, error) {
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
			creator := driverCreator{
				drivers: tt.client,
			}
			creator.deleteKontainerDriver("", tt.prefix)

			if tt.wantDelete {
				assert.Equal(t, 1, len(tt.client.DeleteCalls()))
			} else {
				assert.Equal(t, 0, len(tt.client.DeleteCalls()))
			}
		})
	}
}
