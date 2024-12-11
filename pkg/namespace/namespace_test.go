package namespace

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var errDefault = fmt.Errorf("error")

type DummyIndexer[T any] struct {
	cache.Store
	resources map[string][]T
	err       error
}

func (d *DummyIndexer[T]) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *DummyIndexer[T]) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *DummyIndexer[T]) ListIndexFuncValues(indexName string) []string {
	return []string{}
}
func (d *DummyIndexer[T]) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if d.err != nil {
		return nil, d.err
	}
	resources := d.resources[indexKey]
	var interfaces []interface{}
	for _, r := range resources {
		interfaces = append(interfaces, r)
	}
	return interfaces, nil
}

func (d *DummyIndexer[T]) GetIndexers() cache.Indexers {
	return nil
}

func (d *DummyIndexer[T]) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}

func TestNsByProjectID(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want []string
	}{
		{
			name: "Wrong type",
			obj:  &v1.Pod{},
			want: []string{},
		},
		{
			name: "Matching annotation",
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						projectIDAnnotation: "test-namespace",
					},
				},
			},
			want: []string{"test-namespace"},
		},
		{
			name: "No annotation",
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"bad-annotation": "test-namespace",
					},
				},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NsByProjectID can't return an error, no need to check
			got, _ := NsByProjectID(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NsByProjectID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrtbByRoleTemplateName(t *testing.T) {
	tests := []struct {
		name string
		prtb *v3.ProjectRoleTemplateBinding
		want []string
	}{
		{
			name: "nil prtb",
			prtb: nil,
			want: []string{},
		},
		{
			name: "return prtb roletemplate name",
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "test-roletemplate",
			},
			want: []string{"test-roletemplate"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := PrtbByRoleTemplateName(tt.prtb)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrtbByRoleTemplateName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoleTemplateEnqueueNamespace(t *testing.T) {
	tests := []struct {
		name      string
		obj       runtime.Object
		nsIndexer cache.Indexer
		prtbCache cache.Indexer
		want      []relatedresource.Key
		wantErr   bool
	}{
		{
			name: "nil object returns nil",
			obj:  nil,
			want: nil,
		},
		{
			name: "object is not a RoleTemplate",
			obj:  &v3.ProjectRoleTemplateBinding{},
			want: nil,
		},
		{
			name: "error getting PRTBs",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				err: errDefault,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no PRTBs",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{},
			},
			want: []relatedresource.Key{},
		},
		{
			name: "error getting namespaces",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"test-rt": {{ProjectName: "test-project"}},
				},
			},
			nsIndexer: &DummyIndexer[*v1.Namespace]{
				err: errDefault,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "multiple namespaces match",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			nsIndexer: &DummyIndexer[*v1.Namespace]{
				resources: map[string][]*v1.Namespace{
					"test-project": {
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-namespace2"},
						},
					},
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"test-rt": {{ProjectName: "test-project"}},
				},
			},
			want: []relatedresource.Key{
				{Name: "test-namespace"},
				{Name: "test-namespace2"},
			},
		},
		{
			name: "multiple prtbs match",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			nsIndexer: &DummyIndexer[*v1.Namespace]{
				resources: map[string][]*v1.Namespace{
					"test-project": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
					"test-project2": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace2"},
					}},
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"test-rt": {{ProjectName: "test-project"}, {ProjectName: "test-project2"}},
				},
			},
			want: []relatedresource.Key{
				{Name: "test-namespace"},
				{Name: "test-namespace2"},
			},
		},
		{
			name: "dedupe namespaces",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			nsIndexer: &DummyIndexer[*v1.Namespace]{
				resources: map[string][]*v1.Namespace{
					"test-project": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
					"test-project2": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
				},
			},
			prtbCache: &DummyIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"test-rt": {{ProjectName: "test-project"}, {ProjectName: "test-project2"}},
				},
			},
			want: []relatedresource.Key{
				{Name: "test-namespace"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NsEnqueuer{
				PrtbCache: tt.prtbCache,
				NsIndexer: tt.nsIndexer,
			}

			got, err := n.RoleTemplateEnqueueNamespace("", "", tt.obj)

			if (err != nil) != tt.wantErr {
				t.Errorf("NsEnqueuer.RoleTemplateEnqueueNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NsEnqueuer.RoleTemplateEnqueueNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}
