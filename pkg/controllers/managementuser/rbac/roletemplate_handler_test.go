package rbac

import (
	"errors"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type enqueuedKey struct {
	namespace string
	name      string
}

type fakePRTBController struct {
	normanv3.ProjectRoleTemplateBindingController
	enqueued []enqueuedKey
}

func (c *fakePRTBController) Enqueue(namespace, name string) {
	c.enqueued = append(c.enqueued, enqueuedKey{namespace: namespace, name: name})
}

type fakePRTBClient struct {
	normanv3.ProjectRoleTemplateBindingInterface
	controller *fakePRTBController
}

func (c *fakePRTBClient) Controller() normanv3.ProjectRoleTemplateBindingController {
	return c.controller
}

type fakeCRTBController struct {
	normanv3.ClusterRoleTemplateBindingController
	enqueued []enqueuedKey
}

func (c *fakeCRTBController) Enqueue(namespace, name string) {
	c.enqueued = append(c.enqueued, enqueuedKey{namespace: namespace, name: name})
}

type fakeCRTBClient struct {
	normanv3.ClusterRoleTemplateBindingInterface
	controller *fakeCRTBController
}

func (c *fakeCRTBClient) Controller() normanv3.ClusterRoleTemplateBindingController {
	return c.controller
}

func TestPRTBRoleTemplateEnqueuerSync(t *testing.T) {
	indexErr := errors.New("index failed")
	deletedAt := metav1.Now()
	tests := []struct {
		name           string
		roleTemplate   *mgmtv3.RoleTemplate
		featureEnabled bool
		indexErr       error
		prtbs          []*mgmtv3.ProjectRoleTemplateBinding
		wantObjectNil  bool
		wantErr        string
		wantEnqueued   []enqueuedKey
	}{
		{
			name:          "nil roletemplate returns nil",
			wantObjectNil: true,
		},
		{
			name: "deleted roletemplate does not enqueue",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{
				Name:              "rt-1",
				DeletionTimestamp: &deletedAt,
			}},
			wantObjectNil: true,
		},
		{
			name:           "aggregated roletemplates feature skips enqueue",
			roleTemplate:   &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			featureEnabled: true,
			wantObjectNil:  true,
		},
		{
			name:         "index error is returned",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			indexErr:     indexErr,
			wantErr:      indexErr.Error(),
		},
		{
			name:         "referencing prtbs are enqueued",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			prtbs: []*mgmtv3.ProjectRoleTemplateBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "p-ns-1", Name: "prtb-1"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "p-ns-2", Name: "prtb-2"}},
			},
			wantEnqueued: []enqueuedKey{
				{namespace: "p-ns-1", name: "prtb-1"},
				{namespace: "p-ns-2", name: "prtb-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := features.AggregatedRoleTemplates.Enabled()
			features.AggregatedRoleTemplates.Set(tt.featureEnabled)
			t.Cleanup(func() { features.AggregatedRoleTemplates.Set(prev) })

			controller := &fakePRTBController{}
			enqueuer := &prtbRoleTemplateEnqueuer{
				m: &manager{
					workload: &config.UserContext{ClusterName: "c-1"},
					prtbIndexer: &FakeResourceIndexer[*mgmtv3.ProjectRoleTemplateBinding]{
						resources: map[string][]*mgmtv3.ProjectRoleTemplateBinding{"c-1-rt-1": tt.prtbs},
						err:       tt.indexErr,
						index:     rtbByClusterAndRoleTemplateIndex,
					},
				},
				prtbClient: &fakePRTBClient{controller: controller},
			}

			obj, err := enqueuer.sync("", tt.roleTemplate)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantObjectNil {
				assert.Nil(t, obj)
			} else {
				assert.Equal(t, tt.roleTemplate, obj)
			}
			assert.Equal(t, tt.wantEnqueued, controller.enqueued)
		})
	}
}

func TestCRTBRoleTemplateEnqueuerSync(t *testing.T) {
	indexErr := errors.New("index failed")
	deletedAt := metav1.Now()
	tests := []struct {
		name           string
		roleTemplate   *mgmtv3.RoleTemplate
		featureEnabled bool
		indexErr       error
		crtbs          []*mgmtv3.ClusterRoleTemplateBinding
		wantObjectNil  bool
		wantErr        string
		wantEnqueued   []enqueuedKey
	}{
		{
			name:          "nil roletemplate returns nil",
			wantObjectNil: true,
		},
		{
			name: "deleted roletemplate does not enqueue",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{
				Name:              "rt-1",
				DeletionTimestamp: &deletedAt,
			}},
			wantObjectNil: true,
		},
		{
			name:           "aggregated roletemplates feature skips enqueue",
			roleTemplate:   &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			featureEnabled: true,
			wantObjectNil:  true,
		},
		{
			name:         "index error is returned",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			indexErr:     indexErr,
			wantErr:      indexErr.Error(),
		},
		{
			name:         "referencing crtbs are enqueued",
			roleTemplate: &mgmtv3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-1"}},
			crtbs: []*mgmtv3.ClusterRoleTemplateBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-1", Name: "crtb-1"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-1", Name: "crtb-2"}},
			},
			wantEnqueued: []enqueuedKey{
				{namespace: "c-1", name: "crtb-1"},
				{namespace: "c-1", name: "crtb-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := features.AggregatedRoleTemplates.Enabled()
			features.AggregatedRoleTemplates.Set(tt.featureEnabled)
			t.Cleanup(func() { features.AggregatedRoleTemplates.Set(prev) })

			controller := &fakeCRTBController{}
			enqueuer := &crtbRoleTemplateEnqueuer{
				m: &manager{
					workload: &config.UserContext{ClusterName: "c-1"},
					crtbIndexer: &FakeResourceIndexer[*mgmtv3.ClusterRoleTemplateBinding]{
						resources: map[string][]*mgmtv3.ClusterRoleTemplateBinding{"c-1-rt-1": tt.crtbs},
						err:       tt.indexErr,
						index:     rtbByClusterAndRoleTemplateIndex,
					},
				},
				crtbClient: &fakeCRTBClient{controller: controller},
			}

			obj, err := enqueuer.sync("", tt.roleTemplate)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantObjectNil {
				assert.Nil(t, obj)
			} else {
				assert.Equal(t, tt.roleTemplate, obj)
			}
			assert.Equal(t, tt.wantEnqueued, controller.enqueued)
		})
	}
}
