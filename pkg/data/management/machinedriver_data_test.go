package management

import (
	"sort"
	"strings"
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

func TestGetAnnotations(t *testing.T) {
	testCases := []struct {
		name                string
		inputDriver         *v3.NodeDriver
		driverName          string
		expectedAnnotations map[string]string
	}{
		{
			name:        "known drivers when nodeDriver object is nil",
			inputDriver: nil,
			driverName:  Amazonec2driver,
			expectedAnnotations: map[string]string{
				"fileToFieldAliases":      "sshKeyContents:sshKeypath,userdata:userdata",
				"privateCredentialFields": "secretKey",
				"publicCredentialFields":  "accessKey",
			},
		},
		{
			name: "known drivers with additional annotations",
			inputDriver: &v3.NodeDriver{
				ObjectMeta: v1.ObjectMeta{
					Name: DigitalOceandriver,
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				Spec: v3.NodeDriverSpec{},
			},
			driverName: DigitalOceandriver,
			expectedAnnotations: map[string]string{
				"foo":                     "bar",
				"fileToFieldAliases":      "sshKeyContents:sshKeyPath,userdata:userdata",
				"privateCredentialFields": "accessToken",
			},
		},
		{
			name: "conflicting annotations overwritten by DriverData",
			inputDriver: &v3.NodeDriver{
				ObjectMeta: v1.ObjectMeta{
					Name: GoogleDriver,
					Annotations: map[string]string{
						"fileToFieldAliases": "foo:bar",
					},
				},
			},
			driverName: GoogleDriver,
			expectedAnnotations: map[string]string{
				"fileToFieldAliases":      "authEncodedJson:authEncodedJson,userdata:userdata",
				"privateCredentialFields": "authEncodedJson",
			},
		},
		{
			name: "custom node driver with annotations",
			inputDriver: &v3.NodeDriver{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                    "bar",
						"baz":                    "baz",
						"fileToFieldAliases":     "userdata:userdata",
						"publicCredentialFields": "publicKey",
						"defaults":               "clusterType:imported,port:123",
					},
				},
				Spec: v3.NodeDriverSpec{},
			},
			driverName: "testDriver",
			expectedAnnotations: map[string]string{
				"foo":                    "bar",
				"baz":                    "baz",
				"fileToFieldAliases":     "userdata:userdata",
				"publicCredentialFields": "publicKey",
				"defaults":               "clusterType:imported,port:123",
			},
		},
		{
			name: "custom driver with no annotations",
			inputDriver: &v3.NodeDriver{
				ObjectMeta: v1.ObjectMeta{
					Name: "testDriver",
				},
			},
			driverName:          "testDriver",
			expectedAnnotations: map[string]string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			annotations := getAnnotations(tc.inputDriver, tc.driverName)
			// annotations like fileToFieldAliases or defaults aren't expected to be sorted;
			// sorting is done only for test comparison.
			sortedAnnotations := make(map[string]string, len(annotations))
			for k, v := range annotations {
				sortedAnnotations[k] = splitAndSort(v)
			}
			assert.Equal(t, tc.expectedAnnotations, sortedAnnotations)
		})
	}
}

// Helper: splitAndSort normalizes a comma-separated string by sorting its parts alphabetically.
func splitAndSort(s string) string {
	parts := strings.Split(s, ",")
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
