package scoped

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/norman/types"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeStore struct {
}

func (f fakeStore) Context() types.StorageContext {
	return ""
}
func (f fakeStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (f fakeStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return nil, nil
}
func (f fakeStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}
func (f fakeStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (f fakeStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (f fakeStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}

func TestStoreCreate(t *testing.T) {
	store := fakeStore{}

	p := fakes.ProjectListerMock{}

	tests := []struct {
		name    string
		key     string
		getFunc func(string, string) (*v3.Project, error)
		data    map[string]interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "nil data returns nil",
			data: nil,
			want: nil,
		},
		{
			name: "project: set namespace no backing namespace",
			key:  "projectId",
			getFunc: func(s1, s2 string) (*v3.Project, error) {
				p := v3.Project{
					ObjectMeta: v1.ObjectMeta{
						Name:      "project-XYZ",
						Namespace: "cluster-ABC",
					},
				}
				return &p, nil
			},
			data: map[string]interface{}{
				"projectId": "cluster-ABC:project-XYZ",
			},
			want: map[string]interface{}{
				"projectId":   "cluster-ABC:project-XYZ",
				"namespaceId": "project-XYZ",
			},
		},
		{
			name: "project: set namespace to backing namespace",
			key:  "projectId",
			getFunc: func(s1, s2 string) (*v3.Project, error) {
				p := v3.Project{
					ObjectMeta: v1.ObjectMeta{
						Name:      "project-XYZ",
						Namespace: "cluster-ABC",
					},
					Status: apisv3.ProjectStatus{
						BackingNamespace: "c-ABC-p-XYZ",
					},
				}
				return &p, nil
			},
			data: map[string]interface{}{
				"projectId": "cluster-ABC:project-XYZ",
			},
			want: map[string]interface{}{
				"projectId":   "cluster-ABC:project-XYZ",
				"namespaceId": "c-ABC-p-XYZ",
			},
		},
		{
			name: "error getting project",
			key:  "projectId",
			getFunc: func(s1, s2 string) (*v3.Project, error) {
				return nil, fmt.Errorf("error")
			},
			data: map[string]interface{}{
				"projectId": "cluster-ABC:project-XYZ",
			},
			wantErr: true,
		},
		{
			name: "cluster: set namespaceId",
			key:  "clusterId",
			data: map[string]interface{}{
				"clusterId": "cluster-ABC",
			},
			want: map[string]interface{}{
				"clusterId":   "cluster-ABC",
				"namespaceId": "cluster-ABC",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p.GetFunc = tt.getFunc
			s := &Store{
				Store:        store,
				key:          tt.key,
				projectCache: &p,
			}
			got, err := s.Create(nil, nil, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Store.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Store.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}
