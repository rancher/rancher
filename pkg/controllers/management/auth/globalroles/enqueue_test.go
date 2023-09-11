package globalroles

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_enqueueGRBs(t *testing.T) {
	t.Parallel()
	type testState struct {
		grbCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	}
	tests := []struct {
		name        string
		stateSetup  func(state testState)
		inputObject runtime.Object
		wantKeys    []relatedresource.Key
		wantError   bool
	}{
		{
			name: "no inherited roles",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
			},
			stateSetup: func(state testState) {
				grbs := []*v3.GlobalRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb-1",
						},
						GlobalRoleName: "test-gr",
						UserName:       "u-123xyz",
					},
				}
				state.grbCacheMock.EXPECT().GetByIndex(grbGrIndex, "test-gr").Return(grbs, nil)

			},
			wantKeys: []relatedresource.Key{{Name: "test-grb-1"}},
		},
		{
			name: "empty inherited roles",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
				InheritedClusterRoles: []string{},
			},
			stateSetup: func(state testState) {
				grbs := []*v3.GlobalRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb-1",
						},
						GlobalRoleName: "test-gr",
						UserName:       "u-123xyz",
					},
				}
				state.grbCacheMock.EXPECT().GetByIndex(grbGrIndex, "test-gr").Return(grbs, nil)

			},
			wantKeys: []relatedresource.Key{{Name: "test-grb-1"}},
		},

		{
			name: "inherited roles",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
				InheritedClusterRoles: []string{"test-role"},
			},
			stateSetup: func(state testState) {
				grbs := []*v3.GlobalRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb-1",
						},
						GlobalRoleName: "test-gr",
						UserName:       "u-123xyz",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb-2",
						},
						GlobalRoleName: "test-gr",
						UserName:       "u-123abc",
					},
				}
				state.grbCacheMock.EXPECT().GetByIndex(grbGrIndex, "test-gr").Return(grbs, nil)
			},
			wantKeys: []relatedresource.Key{{Name: "test-grb-1"}, {Name: "test-grb-2"}},
		},
		{
			name: "inherited roles, indexer error",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
				InheritedClusterRoles: []string{"test-role"},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().GetByIndex(grbGrIndex, "test-gr").Return(nil, fmt.Errorf("server not available"))
			},
			wantError: true,
		},
		{
			name: "inherited roles, no grbs",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
				InheritedClusterRoles: []string{"test-role"},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().GetByIndex(grbGrIndex, "test-gr").Return([]*v3.GlobalRoleBinding{}, nil)
			},
			wantKeys: nil,
		},
		{
			name:        "input not a global role",
			inputObject: &v3.ClusterRoleTemplateBinding{},
			wantError:   false,
			wantKeys:    nil,
		},
		{
			name:        "nil input",
			inputObject: nil,
			wantKeys:    nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			grbCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
			state := testState{
				grbCacheMock: grbCache,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			enqueuer := globalRBACEnqueuer{
				grbCache: grbCache,
			}
			res, resErr := enqueuer.enqueueGRBs("", "", test.inputObject)
			require.Len(t, res, len(test.wantKeys))
			for _, key := range test.wantKeys {
				require.Contains(t, res, key)
			}
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
		})
	}
}

func Test_clusterEnqueueGRs(t *testing.T) {
	t.Parallel()
	type testState struct {
		grCacheMock       *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		clusterClientMock *fake.MockNonNamespacedClientInterface[*v3.Cluster, *v3.ClusterList]
	}
	inheritedGR := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
		},
		InheritedClusterRoles: []string{"test-role"},
	}
	noInheritedGR := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-no-gr",
		},
	}
	emptyInheritedGR := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-no-gr",
		},
		InheritedClusterRoles: []string{},
	}
	tests := []struct {
		name        string
		stateSetup  func(state testState)
		inputObject runtime.Object
		wantKeys    []relatedresource.Key
		wantError   bool
	}{
		{
			name: "cluster already synced",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Annotations: map[string]string{
						initialSyncAnnotation: "true",
					},
				},
			},
			stateSetup: func(state testState) {
				// current implementation doesn't call this, but we want to setup the state in case
				// call order changes slightly
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{&inheritedGR}, nil).AnyTimes()
			},
			wantKeys: nil,
		},
		{
			name: "cluster not synced",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{&inheritedGR, &noInheritedGR, &emptyInheritedGR}, nil)
				state.clusterClientMock.EXPECT().Get("test-cluster", gomock.Any()).Return(
					&v3.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-cluster",
							Annotations: map[string]string{
								"some-annotation": "here",
							},
						},
					}, nil)
				updCluster := v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						Annotations: map[string]string{
							"some-annotation":     "here",
							initialSyncAnnotation: "true",
						},
					},
				}
				state.clusterClientMock.EXPECT().Update(&updCluster).Return(&updCluster, nil)
			},
			wantKeys: []relatedresource.Key{{Name: inheritedGR.Name}},
		},
		{
			name: "cluster not synced - no annotations",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{&inheritedGR, &noInheritedGR, &emptyInheritedGR}, nil)
				state.clusterClientMock.EXPECT().Get("test-cluster", gomock.Any()).Return(
					&v3.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-cluster",
						},
					}, nil)
				updCluster := v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						Annotations: map[string]string{
							initialSyncAnnotation: "true",
						},
					},
				}
				state.clusterClientMock.EXPECT().Update(&updCluster).Return(&updCluster, nil)
			},
			wantKeys: []relatedresource.Key{{Name: inheritedGR.Name}},
		},
		{
			name: "input not a cluster",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gr",
				},
				InheritedClusterRoles: []string{"test-role"},
			},
			wantError: false,
			wantKeys:  nil,
		},
		{
			name:        "nil input",
			inputObject: nil,
			wantKeys:    nil,
		},
		{
			name: "cluster not synced, gr list error",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("server unavailable"))
			},
			wantKeys:  nil,
			wantError: true,
		},
		{
			name: "cluster not synced, get cluster error",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{&inheritedGR}, nil)
				state.clusterClientMock.EXPECT().Get("test-cluster", gomock.Any()).Return(nil, fmt.Errorf("server unavailable"))
			},
			wantKeys: []relatedresource.Key{{Name: inheritedGR.Name}},
		},
		{
			name: "cluster not synced, update cluster error",
			inputObject: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{&inheritedGR}, nil)
				state.clusterClientMock.EXPECT().Get("test-cluster", gomock.Any()).Return(&v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						Annotations: map[string]string{
							"some-annotation": "here",
						},
					},
				}, nil)
				state.clusterClientMock.EXPECT().Update(&v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						Annotations: map[string]string{
							"some-annotation":     "here",
							initialSyncAnnotation: "true",
						},
					},
				}).Return(nil, fmt.Errorf("server unavailable"))
			},
			wantKeys: []relatedresource.Key{{Name: inheritedGR.Name}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			grCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
			clusterClient := fake.NewMockNonNamespacedClientInterface[*v3.Cluster, *v3.ClusterList](ctrl)
			state := testState{
				grCacheMock:       grCache,
				clusterClientMock: clusterClient,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			enqueuer := globalRBACEnqueuer{
				grCache:       grCache,
				clusterClient: clusterClient,
			}
			res, resErr := enqueuer.clusterEnqueueGRs("", "", test.inputObject)
			require.Len(t, res, len(test.wantKeys))
			for _, key := range test.wantKeys {
				require.Contains(t, res, key)
			}
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
		})
	}
}

func Test_crtbEnqueueGRB(t *testing.T) {
	t.Parallel()
	type testState struct {
		grbCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	}
	testGrb := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
		},
	}
	tests := []struct {
		name        string
		stateSetup  func(state testState)
		inputObject runtime.Object
		wantKeys    []relatedresource.Key
		wantError   bool
	}{
		{
			name: "crtb not owned by grb",
			inputObject: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crtb",
				},
			},
			wantKeys: nil,
		},
		{
			name: "crtb owned by existing grb",
			inputObject: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crtb",
					Labels: map[string]string{
						grbOwnerLabel: testGrb.Name,
					},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().Get(testGrb.Name).Return(&testGrb, nil)
			},
			wantKeys: []relatedresource.Key{{Name: testGrb.Name}},
		},
		{
			name: "crtb owned by non-existent grb",
			inputObject: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crtb",
					Labels: map[string]string{
						grbOwnerLabel: testGrb.Name,
					},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().Get(testGrb.Name).Return(nil, apierrors.NewNotFound(schema.GroupResource{
					Group:    v3.SchemeGroupVersion.Group,
					Resource: v3.GlobalRoleResourceName,
				}, testGrb.Name))
			},
			wantKeys: nil,
		},
		{
			name: "crtb owned by grb, error when confirming grb existence",
			inputObject: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crtb",
					Labels: map[string]string{
						grbOwnerLabel: testGrb.Name,
					},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().Get(testGrb.Name).Return(nil, fmt.Errorf("server unavailable"))
			},
			wantError: true,
		},
		{
			name: "invalid input object type",
			inputObject: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
			},
			wantKeys:  nil,
			wantError: false,
		},
		{
			name:        "nil input object",
			inputObject: nil,
			wantKeys:    nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			grbCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
			state := testState{
				grbCacheMock: grbCache,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			enqueuer := globalRBACEnqueuer{
				grbCache: grbCache,
			}
			res, resErr := enqueuer.crtbEnqueueGRB("", "", test.inputObject)
			require.Len(t, res, len(test.wantKeys))
			for _, key := range test.wantKeys {
				require.Contains(t, res, key)
			}
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
		})
	}
}
