package namespace

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var errDefault = fmt.Errorf("error")

type DummyIndexer struct {
	cache.Store
	namespaces map[string][]*v1.Namespace
	err        error
}

func (d *DummyIndexer) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *DummyIndexer) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *DummyIndexer) ListIndexFuncValues(indexName string) []string {
	return []string{}
}
func (d *DummyIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if d.err != nil {
		return nil, d.err
	}
	namespaces := d.namespaces[indexKey]
	var interfaces []interface{}
	for _, ns := range namespaces {
		interfaces = append(interfaces, ns)
	}
	return interfaces, nil
}

func (d *DummyIndexer) GetIndexers() cache.Indexers {
	return nil
}

func (d *DummyIndexer) AddIndexers(newIndexers cache.Indexers) error {
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
		prtbCache func() ([]*v3.ProjectRoleTemplateBinding, error)
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
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) { return nil, errDefault },
			want:      nil,
			wantErr:   true,
		},
		{
			name: "no PRTBs",
			obj: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rt",
				},
			},
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) {
				return []*v3.ProjectRoleTemplateBinding{}, nil
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
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) {
				return []*v3.ProjectRoleTemplateBinding{
					{
						ProjectName: "test-project",
					},
				}, nil
			},
			nsIndexer: &DummyIndexer{
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
			nsIndexer: &DummyIndexer{
				namespaces: map[string][]*v1.Namespace{
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
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) {
				return []*v3.ProjectRoleTemplateBinding{
					{
						ProjectName: "test-project",
					},
				}, nil
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
			nsIndexer: &DummyIndexer{
				namespaces: map[string][]*v1.Namespace{
					"test-project": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
					"test-project2": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace2"},
					}},
				},
			},
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) {
				return []*v3.ProjectRoleTemplateBinding{
					{
						ProjectName: "test-project",
					},
					{
						ProjectName: "test-project2",
					},
				}, nil
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
			nsIndexer: &DummyIndexer{
				namespaces: map[string][]*v1.Namespace{
					"test-project": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
					"test-project2": {{
						ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
					}},
				},
			},
			prtbCache: func() ([]*v3.ProjectRoleTemplateBinding, error) {
				return []*v3.ProjectRoleTemplateBinding{
					{
						ProjectName: "test-project",
					},
					{
						ProjectName: "test-project2",
					},
				}, nil
			},
			want: []relatedresource.Key{
				{Name: "test-namespace"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			prtbCache := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			if tt.prtbCache != nil {
				prtbCache.EXPECT().GetByIndex(gomock.Any(), gomock.Any()).Return(tt.prtbCache())
			}

			n := &NsEnqueuer{
				PrtbCache: prtbCache,
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
