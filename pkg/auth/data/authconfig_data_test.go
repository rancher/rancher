package data

import (
	"errors"
	"testing"

	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsEmptyAuthConfig(t *testing.T) {
	tests := map[string]struct {
		content map[string]any
		want    bool
	}{
		"only base fields": {
			content: map[string]any{
				"apiVersion":         "management.cattle.io/v3",
				"kind":               "AuthConfig",
				"metadata":           map[string]any{"name": "github"},
				"type":               "githubConfig",
				"enabled":            false,
				"logoutAllSupported": false,
				"status":             map[string]any{},
			},
			want: true,
		},
		"provider specific field set": {
			content: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata":   map[string]any{"name": "github"},
				"type":       "githubConfig",
				"clientId":   "some-client-id",
			},
			want: false,
		},
		"empty map": {
			content: map[string]any{},
			want:    true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, isEmptyAuthConfig(test.content))
		})
	}
}

func TestDeleteEmptyDisabledAuthConfigs(t *testing.T) {
	tests := map[string]struct {
		items       []unstructured.Unstructured
		listErr     error
		deleteErr   error
		wantDeleted []string
		wantErr     bool
	}{
		"deletes empty disabled auth configs": {
			items: []unstructured.Unstructured{
				newAuthConfigItem("github", false, nil),
				newAuthConfigItem("ldap", false, nil),
			},
			wantDeleted: []string{"github", "ldap"},
		},
		"skips enabled auth configs": {
			items: []unstructured.Unstructured{
				newAuthConfigItem("github", true, nil),
			},
			wantDeleted: nil,
		},
		"skips auth configs with provider specific data": {
			items: []unstructured.Unstructured{
				newAuthConfigItem("github", false, map[string]any{"clientId": "some-id"}),
			},
			wantDeleted: nil,
		},
		"never deletes the local auth config": {
			items: []unstructured.Unstructured{
				newAuthConfigItem(localprovider.Name, false, nil),
			},
			wantDeleted: nil,
		},
		"list error is propagated": {
			listErr: errors.New("boom"),
			wantErr: true,
		},
		"not found delete errors are ignored": {
			items: []unstructured.Unstructured{
				newAuthConfigItem("github", false, nil),
			},
			deleteErr: apierrors.NewNotFound(schema.GroupResource{Resource: "authconfigs"}, "github"),
		},
		"other delete errors are propagated": {
			items: []unstructured.Unstructured{
				newAuthConfigItem("github", false, nil),
			},
			deleteErr: errors.New("boom"),
			wantErr:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			lister := &fakeUnstructuredLister{
				list: &unstructured.UnstructuredList{Items: test.items},
				err:  test.listErr,
			}
			deleter := &fakeAuthConfigDeleter{err: test.deleteErr}

			err := deleteEmptyDisabledAuthConfigs(deleter, lister)
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantDeleted, deleter.deleted)
		})
	}
}

func newAuthConfigItem(name string, enabled bool, extraFields map[string]any) unstructured.Unstructured {
	content := map[string]any{
		"apiVersion":         "management.cattle.io/v3",
		"kind":               "AuthConfig",
		"logoutAllSupported": false,
		"metadata": map[string]any{
			"annotations": map[string]any{
				"management.cattle.io/auth-provider-cleanup": "rancher-locked",
			},
			"creationTimestamp": "2026-09-24T10:37:24Z",
			"generation":        2,
			"labels": map[string]any{
				"cattle.io/creator": "norman",
			},
			"name":            name,
			"resourceVersion": "10636",
			"uid":             "e7b7f762-0c42-456b-ac42-0b95779947ec",
		},
		"type": name + "Config",
		"status": map[string]any{
			"conditions": nil,
		},
	}

	if enabled {
		content["enabled"] = true
	}
	for k, v := range extraFields {
		content[k] = v
	}

	return unstructured.Unstructured{Object: content}
}

type fakeUnstructuredLister struct {
	list *unstructured.UnstructuredList
	err  error
}

func (f *fakeUnstructuredLister) List(opts v1.ListOptions) (runtime.Object, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

type fakeAuthConfigDeleter struct {
	deleted []string
	err     error
}

func (f *fakeAuthConfigDeleter) Delete(name string, options *v1.DeleteOptions) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, name)
	return nil
}
