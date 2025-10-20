package resourcequota

import (
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestReconcileNamespaces(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name        string
		err         error
		setup       func(ctrl *gomock.Controller, enqCounter *int) *reconcileController
		project     *apiv3.Project
		enqExpected int
	}{
		{
			name:    "nil project",
			project: nil,
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				return &reconcileController{}
			},
			err: nil,
		},
		{
			name: "deleted project",
			project: &apiv3.Project{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				return &reconcileController{}
			},
			err: nil,
		},
		// Unknown how to induce error for `ByIndex` call
		{
			name: "project with namespaces",
			project: &apiv3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "p-namespace",
					Name:      "p-name",
				},
			},
			enqExpected: 1,
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				nsMockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				nsMockIndexer.AddIndexers(cache.Indexers{nsByProjectIndex: nsByProjectID})
				nsMockIndexer.Add(&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "a-namespace",
						Annotations: map[string]string{
							projectIDAnnotation: "p-namespace:p-name",
						},
					},
				})

				nsMock := wranglerfake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
				nsMock.EXPECT().
					Enqueue(gomock.Any()).
					DoAndReturn(func (_ string) {
						*enqCounter = *enqCounter + 1
				})

				return &reconcileController{
					namespaces: nsMock,
					nsIndexer:  nsMockIndexer,
				}
			},
		},
		{
			name: "project without namespaces, empty used limit, ok",
			// no error, no actions on projects nor namespaces
			project: &apiv3.Project{},
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				nsMockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				nsMockIndexer.AddIndexers(cache.Indexers{nsByProjectIndex: nsByProjectID})
				return &reconcileController{
					nsIndexer: nsMockIndexer,
				}
			},
			err: nil,
		},
		{
			name: "project without namespaces, yet non-empty used limit",
			project: &apiv3.Project{
				Spec: apiv3.ProjectSpec{
					ResourceQuota: &apiv3.ProjectResourceQuota{
						UsedLimit: apiv3.ResourceQuotaLimit{
							Pods: "2500025",
						},
					},
				},
			},
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				nsMockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				nsMockIndexer.AddIndexers(cache.Indexers{nsByProjectIndex: nsByProjectID})

				projectMock := wranglerfake.NewMockControllerInterface[*apiv3.Project, *apiv3.ProjectList](ctrl)
				projectMock.EXPECT().Update(gomock.Any()).Return(nil, nil)

				return &reconcileController{
					nsIndexer: nsMockIndexer,
					projects:  projectMock,
				}
			},
		},
		{
			name: "project without namespaces, yet non-empty used limit, update error",
			project: &apiv3.Project{
				Spec: apiv3.ProjectSpec{
					ResourceQuota: &apiv3.ProjectResourceQuota{
						UsedLimit: apiv3.ResourceQuotaLimit{
							Pods: "2500025",
						},
					},
				},
			},
			setup: func(ctrl *gomock.Controller, enqCounter *int) *reconcileController {
				nsMockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
					cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				nsMockIndexer.AddIndexers(cache.Indexers{nsByProjectIndex: nsByProjectID})

				projectMock := wranglerfake.NewMockControllerInterface[*apiv3.Project, *apiv3.ProjectList](ctrl)
				projectMock.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("some error"))

				return &reconcileController{
					nsIndexer: nsMockIndexer,
					projects:  projectMock,
				}
			},
			err: fmt.Errorf("some error"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var enqCalled int
			r := tt.setup(ctrl, &enqCalled)

			_, err := r.reconcileNamespaces("dummy", tt.project)

			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.enqExpected, enqCalled)
		})
	}
}
