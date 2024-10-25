package globalroles

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	longName           = "long-name--------------------------------------------------------"
	safeConcatLongName = "long-name------------------------------------------------cc9244"
)

func Test_grNsIndexer(t *testing.T) {
	t.Parallel()
	gr := &v3.GlobalRole{
		NamespacedRules: map[string][]v1.PolicyRule{
			"ns1": nil,
			"ns2": nil,
		},
	}
	res, resErr := grNsIndexer(gr)
	require.NoError(t, resErr)
	require.Len(t, res, 2)
	require.Contains(t, res, "ns1")
	require.Contains(t, res, "ns2")

	gr.NamespacedRules = map[string][]v1.PolicyRule{}
	res, resErr = grNsIndexer(gr)
	require.NoError(t, resErr)
	require.Len(t, res, 0)

	gr.NamespacedRules = nil
	res, resErr = grNsIndexer(gr)
	require.NoError(t, resErr)
	require.Len(t, res, 0)
}

func Test_grbGrIndexer(t *testing.T) {
	t.Parallel()
	grb := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
		},
		GlobalRoleName: "test-gr",
	}
	res, resErr := grbGrIndexer(grb)
	require.NoError(t, resErr)
	require.Equal(t, []string{"test-gr"}, res)
}

func Test_grSafeConcatIndexer(t *testing.T) {
	t.Parallel()
	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gr1",
		},
	}
	res, resErr := grSafeConcatIndexer(gr)
	require.NoError(t, resErr)
	require.Equal(t, []string{"gr1"}, res)

	// Make sure it returns the concatenated name
	gr.SetName(longName)
	res, resErr = grSafeConcatIndexer(gr)
	require.NoError(t, resErr)
	require.Equal(t, []string{safeConcatLongName}, res)
}

func Test_grbSafeConcatIndexer(t *testing.T) {
	t.Parallel()
	grb := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "grb1",
		},
	}
	res, resErr := grbSafeConcatIndexer(grb)
	require.NoError(t, resErr)
	require.Equal(t, []string{"grb1"}, res)

	// Make sure it returns the concatenated name
	grb.SetName(longName)
	res, resErr = grbSafeConcatIndexer(grb)
	require.NoError(t, resErr)
	require.Equal(t, []string{safeConcatLongName}, res)
}

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

func Test_roleEnqueueGR(t *testing.T) {
	t.Parallel()
	type testState struct {
		grCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}
	tests := []struct {
		name        string
		stateSetup  func(state testState)
		inputObject runtime.Object
		wantKeys    []relatedresource.Key
		wantError   bool
	}{
		{
			name:        "object is nil",
			inputObject: nil,
			wantError:   false,
		},
		{
			name:        "object is not a role",
			inputObject: &v3.GlobalRole{},
			wantError:   false,
		},
		{
			name: "role does not have owning GR",
			inputObject: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-role",
				},
			},
			wantError: false,
		},
		{
			name: "GR get fails",
			inputObject: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-role",
					Labels: map[string]string{grOwnerLabel: "test-GR"},
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().GetByIndex(grSafeConcatIndex, "test-GR").Return(nil, fmt.Errorf("error"))
			},
			wantError: true,
		},
		{
			name: "GR gets enqueued",
			inputObject: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-role",
					Labels: map[string]string{grOwnerLabel: "test-GR"},
				},
			},
			stateSetup: func(state testState) {
				gr := &v3.GlobalRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-GR",
					},
				}
				state.grCacheMock.EXPECT().GetByIndex(grSafeConcatIndex, "test-GR").Return([]*v3.GlobalRole{gr}, nil)
			},
			wantKeys:  []relatedresource.Key{{Name: "test-GR"}},
			wantError: false,
		},
		{
			name: "GR with long name gets enqueued",
			inputObject: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-role",
					Labels: map[string]string{grOwnerLabel: safeConcatLongName},
				},
			},
			stateSetup: func(state testState) {
				gr := &v3.GlobalRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: longName,
					},
				}
				state.grCacheMock.EXPECT().GetByIndex(grSafeConcatIndex, safeConcatLongName).Return([]*v3.GlobalRole{gr}, nil)
			},
			wantKeys:  []relatedresource.Key{{Name: longName}},
			wantError: false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			grCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
			state := testState{
				grCacheMock: grCache,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			enqueuer := globalRBACEnqueuer{
				grCache: grCache,
			}
			res, resErr := enqueuer.roleEnqueueGR("", "", test.inputObject)
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

func Test_roleBindingEnqueueGRB(t *testing.T) {
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
			name:        "object is nil",
			inputObject: nil,
			wantError:   false,
		},
		{
			name:        "object is not a roleBinding",
			inputObject: &v3.GlobalRole{},
			wantError:   false,
		},
		{
			name: "roleBinding does not have owning GR",
			inputObject: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-roleBinding",
				},
			},
			wantError: false,
		},
		{
			name: "GRB get fails",
			inputObject: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-roleBinding",
					Labels: map[string]string{grbOwnerLabel: "test-GRB"},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().GetByIndex(grbSafeConcatIndex, "test-GRB").Return(nil, fmt.Errorf("error"))
			},
			wantError: true,
		},
		{
			name: "GRB gets enqueued",
			inputObject: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-roleBinding",
					Labels: map[string]string{grbOwnerLabel: "test-GRB"},
				},
			},
			stateSetup: func(state testState) {
				grb := &v3.GlobalRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-GRB",
					},
				}
				state.grbCacheMock.EXPECT().GetByIndex(grbSafeConcatIndex, "test-GRB").Return([]*v3.GlobalRoleBinding{grb}, nil)
			},
			wantKeys:  []relatedresource.Key{{Name: "test-GRB"}},
			wantError: false,
		},
		{
			name: "GRB with long name gets enqueued",
			inputObject: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-roleBinding",
					Labels: map[string]string{grbOwnerLabel: safeConcatLongName},
				},
			},
			stateSetup: func(state testState) {
				grb := &v3.GlobalRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: longName,
					},
				}
				state.grbCacheMock.EXPECT().GetByIndex(grbSafeConcatIndex, safeConcatLongName).Return([]*v3.GlobalRoleBinding{grb}, nil)
			},
			wantKeys:  []relatedresource.Key{{Name: longName}},
			wantError: false,
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
			res, resErr := enqueuer.roleBindingEnqueueGRB("", "", test.inputObject)
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

func Test_namespaceEnqueueGR(t *testing.T) {
	t.Parallel()
	type testState struct {
		grCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}
	tests := []struct {
		name        string
		stateSetup  func(state testState)
		inputObject runtime.Object
		wantKeys    []relatedresource.Key
		wantError   bool
	}{
		{
			name:        "object is nil",
			inputObject: nil,
			wantError:   false,
		},
		{
			name:        "object is not a namespace",
			inputObject: &v3.GlobalRole{},
			wantError:   false,
		},
		{
			name: "namespace not in a NamespacedRule",
			inputObject: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().GetByIndex(grNsIndex, "ns1").Return([]*v3.GlobalRole{}, nil)
			},
			wantError: false,
		},
		{
			name: "get namespace fails",
			inputObject: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().GetByIndex(grNsIndex, "ns1").Return(nil, fmt.Errorf("error"))
			},
			wantError: true,
		},
		{
			name: "multiple grs are returned",
			inputObject: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
				},
			},
			stateSetup: func(state testState) {
				grs := []*v3.GlobalRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gr1",
						},
						NamespacedRules: map[string][]v1.PolicyRule{
							"ns1": nil,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gr2",
						},
						NamespacedRules: map[string][]v1.PolicyRule{
							"ns1": nil,
						},
					},
				}
				state.grCacheMock.EXPECT().GetByIndex(grNsIndex, "ns1").Return(grs, nil)
			},
			wantKeys: []relatedresource.Key{
				{Name: "gr1"},
				{Name: "gr2"},
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			grCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
			state := testState{
				grCacheMock: grCache,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			enqueuer := globalRBACEnqueuer{
				grCache: grCache,
			}
			res, resErr := enqueuer.namespaceEnqueueGR("", "", test.inputObject)
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
func TestClusterRoleEnqueueGR(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		grCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}

	tests := map[string]struct {
		obj              runtime.Object
		stateSetup       func(state testState)
		wantKeys         []relatedresource.Key
		wantErrorMessage string
	}{
		"enqueue grb if cr contains the label authz.management.cattle.io/grb-owner": {
			obj: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						grOwnerLabel: "gr",
					},
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().GetByIndex(grSafeConcatIndex, "gr").Return([]*v3.GlobalRole{{
					ObjectMeta: metav1.ObjectMeta{Name: "gr"},
				}}, nil)
			},
			wantKeys: []relatedresource.Key{
				{
					Name: "gr",
				},
			},
		},
		"don't enqueue grb if cr doesn't contain the label authz.management.cattle.io/grb-owner": {
			obj:      &v1.ClusterRole{},
			wantKeys: nil,
		},
		"nil obj": {
			obj:      nil,
			wantKeys: nil,
		},
		"can't cast obj to ClusterRole": {
			obj:      &v1.Role{},
			wantKeys: nil,
		},
		"can't get GlobalRole from cache": {
			obj: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						grOwnerLabel: "gr",
					},
				},
			},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().GetByIndex(grSafeConcatIndex, "gr").Return(nil, errors.New("unexpected error"))
			},
			wantKeys:         nil,
			wantErrorMessage: "unable to get GlobalRole gr for RoleBinding : unexpected error",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			state := testState{
				grCacheMock: fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			g := globalRBACEnqueuer{
				grCache: state.grCacheMock,
			}
			keys, err := g.clusterRoleEnqueueGR("", "", test.obj)
			if test.wantErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.wantErrorMessage)
			}
			assert.Equal(t, test.wantKeys, keys)
		})
	}
}

func TestClusterRoleBindingEnqueueGRB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		grbCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	}

	tests := map[string]struct {
		obj              runtime.Object
		stateSetup       func(state testState)
		wantKeys         []relatedresource.Key
		wantErrorMessage string
	}{
		"enqueue grb if cr contains the label authz.management.cattle.io/grb-fw-owner": {
			obj: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						grbOwnerLabel: "grb",
					},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().GetByIndex(grbSafeConcatIndex, "grb").Return([]*v3.GlobalRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{Name: "grb"},
				}}, nil)
			},
			wantKeys: []relatedresource.Key{
				{
					Name: "grb",
				},
			},
		},
		"don't enqueue grb if cr doesn't contain the label authz.management.cattle.io/grb-fw-owner": {
			obj:      &v1.ClusterRoleBinding{},
			wantKeys: nil,
		},
		"nil obj": {
			obj:      nil,
			wantKeys: nil,
		},
		"can't cast obj to ClusterRoleBinding": {
			obj:      &v1.Role{},
			wantKeys: nil,
		},
		"can't get GlobalRoleBinding from cache": {
			obj: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						grbOwnerLabel: "grb",
					},
				},
			},
			stateSetup: func(state testState) {
				state.grbCacheMock.EXPECT().GetByIndex(grbSafeConcatIndex, "grb").Return(nil, errors.New("unexpected error"))
			},
			wantKeys:         nil,
			wantErrorMessage: "unable to get GlobalRoleBinding grb for ClusterRoleBinding : unexpected error",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			state := testState{
				grbCacheMock: fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			g := globalRBACEnqueuer{
				grbCache: state.grbCacheMock,
			}
			keys, err := g.clusterRoleBindingEnqueueGRB("", "", test.obj)
			if test.wantErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.wantErrorMessage)
			}
			assert.Equal(t, test.wantKeys, keys)
		})
	}
}

func TestFleetWorkspaceEnqueueGRB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		grCacheMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}

	tests := map[string]struct {
		obj            runtime.Object
		stateSetup     func(state testState)
		wantKeys       []relatedresource.Key
		wantErrMessage string
	}{
		"enqueue just the GlobalRoles with InheritedFleetWorkspacePermissions": {
			obj: &v3.FleetWorkspace{},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.GlobalRole{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "gr1"},
						InheritedFleetWorkspacePermissions: &v3.FleetWorkspacePermission{
							ResourceRules: []v1.PolicyRule{
								{
									Verbs:     []string{"*"},
									APIGroups: []string{"fleet.cattle.io"},
									Resources: []string{"*"},
								},
							},
							WorkspaceVerbs: []string{"get"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "gr2"},
					},
				}, nil)
			},
			wantKeys: []relatedresource.Key{
				{
					Name: "gr1",
				},
			},
		},
		"error listing GlobalRoles": {
			obj: &v3.FleetWorkspace{},
			stateSetup: func(state testState) {
				state.grCacheMock.EXPECT().List(labels.Everything()).Return(nil, errors.New("unexpected error"))
			},
			wantErrMessage: "unable to list current GlobalRoles: unexpected error",
		},
		"nil obj": {
			obj: nil,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				grCacheMock: fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			g := globalRBACEnqueuer{
				grCache: state.grCacheMock,
			}
			keys, err := g.fleetWorkspaceEnqueueGR("", "", test.obj)
			if test.wantErrMessage != "" {
				assert.EqualError(t, err, test.wantErrMessage)
			}
			assert.Equal(t, test.wantKeys, keys)
		})
	}
}
