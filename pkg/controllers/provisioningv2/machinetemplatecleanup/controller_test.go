package machinetemplatecleanup

import (
	"fmt"
	"testing"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	wfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func Test_cleanupOrLabelObject(t *testing.T) {
	scheme := runtime.NewScheme()

	h := &handler{
		ctx: t.Context(),
	}

	now, err := time.Parse(time.RFC3339, "2026-09-30T15:30:00Z")
	require.NoError(t, err)

	tests := []struct {
		name          string
		obj           unstructured.Unstructured
		expectDeleted bool
		label         bool
		expectedLabel bool
	}{
		{
			name: "InfraCluster has no owners but is too recent",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: "v1beta2",
					}.String(),
					"kind": "AWSCluster",
					"metadata": map[string]interface{}{
						"name":              "foocluster",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"resourceVersion":   "1",
						"creationTimestamp": "2026-09-30T15:00:00Z",
					},
				},
			},
			expectDeleted: false,
			label:         false,
			expectedLabel: false,
		},
		{
			name: "InfraCluster has owners",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: "v1beta2",
					}.String(),
					"kind": "AWSCluster",
					"metadata": map[string]interface{}{
						"name":              "foocluster",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"resourceVersion":   "1",
						"creationTimestamp": "2026-09-30T14:00:00Z",
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       "owner-cluster",
								"uid":        "owner-uid",
							},
						},
					},
				},
			},
			expectDeleted: false,
			label:         false,
			expectedLabel: false,
		},
		{
			name: "InfraCluster has no owners and is sufficiently old",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: "v1beta2",
					}.String(),
					"kind": "AWSCluster",
					"metadata": map[string]interface{}{
						"name":              "foocluster",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"resourceVersion":   "1",
						"creationTimestamp": "2026-09-30T14:29:00Z",
					},
				},
			},
			expectDeleted: true,
			label:         false,
			expectedLabel: false,
		},
		{
			name: "Template has owners, should be labeled",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: "v1beta2",
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "footemplate",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"resourceVersion":   "1",
						"creationTimestamp": "2026-09-30T14:29:00Z",
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       "owner-cluster",
								"uid":        "owner-uid",
							},
						},
					},
				},
			},
			expectDeleted: false,
			label:         true,
			expectedLabel: true,
		},
		{
			name: "Template has no owners and is recent, should not be labeled or deleted",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: "v1beta2",
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "footemplate",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"resourceVersion":   "1",
						"creationTimestamp": "2026-09-30T15:00:00Z",
					},
				},
			},
			expectDeleted: false,
			label:         true,
			expectedLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			infraClusterClient := dynamicClient.Resource(schema.GroupVersionResource{
				Group:    capi.GroupVersionInfrastructure.Group,
				Version:  "v1beta2",
				Resource: "awsclusters",
			})

			_, err = infraClusterClient.Namespace(
				tt.obj.GetNamespace()).Create(t.Context(),
				&tt.obj,
				metav1.CreateOptions{},
			)
			require.NoError(t, err)

			// Clear create call.
			dynamicClient.ClearActions()

			err := h.cleanupOrLabelObject(&tt.obj, infraClusterClient, tt.label, now)
			require.NoError(t, err)

			if tt.expectDeleted {
				require.Len(t, dynamicClient.Actions(), 1)
				assert.Equal(t, "delete", dynamicClient.Actions()[0].GetVerb())
			} else if tt.expectedLabel {
				require.Len(t, dynamicClient.Actions(), 1)
				assert.Equal(t, "update", dynamicClient.Actions()[0].GetVerb())
			} else {
				assert.Len(t, dynamicClient.Actions(), 0)
			}
		})
	}
}

func Test_cleanupInfraMachineTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	capiCache := wfake.NewMockCacheInterface[*capi.Cluster](ctrl)
	provController := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)

	mdClient := wfake.NewMockClientInterface[*capi.MachineDeployment, *capi.MachineDeploymentList](ctrl)
	msClient := wfake.NewMockClientInterface[*capi.MachineSet, *capi.MachineSetList](ctrl)

	templateGVK := schema.GroupVersionKind{
		Group:   capi.GroupVersionInfrastructure.Group,
		Version: capi.GroupVersionInfrastructure.Version,
		Kind:    "AWSMachineTemplate",
	}

	templateGVR := templateGVK.GroupVersion().WithResource("awsmachinetemplates")

	h := handler{
		ctx: t.Context(),

		provClusterController: provController,

		capiClusterCache: capiCache,

		machineDeploymentClient: mdClient,
		machineSetClient:        msClient,
	}

	h.initGVRs()

	_, err := h.OnCRD("", &apiextv1.CustomResourceDefinition{
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: templateGVR.Group,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:   templateGVR.Version,
					Served: true,
				},
			},
		},
		Status: apiextv1.CustomResourceDefinitionStatus{
			AcceptedNames: apiextv1.CustomResourceDefinitionNames{
				Kind:   templateGVK.Kind,
				Plural: templateGVR.Resource,
			},
		},
	})
	require.NoError(t, err)

	const namespace = "fleet-default"
	cluster := &provv1.Cluster{
		Name:      "foo-cluster",
		Namespace: namespace,
		UID:       "prov-cluster-uid",
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				InfrastructureRef: &v1.ObjectReference{
					APIVersion: capi.GroupVersionInfrastructure.String(),
					Kind:       "AWSCluster",
					Name:       "foo-cluster",
				},
				MachinePools: []provv1.RKEMachinePool{
					{
						NodeConfig: &v1.ObjectReference{
							APIVersion: templateGVK.GroupVersion().String(),
							Name:       "template-a",
						},
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		Name:      "foo-cluster",
		Namespace: namespace,
		UID:       "capi-cluster-uid",
		OwnerReferences: []metav1.OwnerReference{
			{
				Name:       cluster.Name,
				APIVersion: provv1.SchemeGroupVersion.String(),
				Kind:       "Cluster",
				UID:        cluster.UID,
			},
		},
	}

	provController.EXPECT().Get(cluster.Namespace, cluster.Name, metav1.GetOptions{}).Return(cluster, nil).AnyTimes()
	capiCache.EXPECT().Get(capiCluster.Namespace, capiCluster.Name).Return(capiCluster, nil).AnyTimes()

	mdClient.EXPECT().List(cluster.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, cluster.Name),
		Limit:         pageSize,
		Continue:      "",
	}).Return(&capi.MachineDeploymentList{
		Items: []capi.MachineDeployment{
			{
				Spec: capi.MachineDeploymentSpec{
					Template: capi.MachineTemplateSpec{
						Spec: capi.MachineSpec{
							InfrastructureRef: capi.ContractVersionedObjectReference{
								Name: "template-b",
							},
						},
					},
				},
			},
		},
	}, nil).AnyTimes()

	msClient.EXPECT().List(cluster.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, cluster.Name),
		Limit:         pageSize,
		Continue:      "",
	}).Return(&capi.MachineSetList{
		Items: []capi.MachineSet{
			{
				Spec: capi.MachineSetSpec{
					Template: capi.MachineTemplateSpec{
						Spec: capi.MachineSpec{
							InfrastructureRef: capi.ContractVersionedObjectReference{
								Name: "template-c",
							},
						},
					},
				},
			},
		},
	}, nil).AnyTimes()

	now, err := time.Parse(time.RFC3339, "2026-09-30T15:30:00Z")
	require.NoError(t, err)

	tests := []struct {
		name          string
		obj           *unstructured.Unstructured
		expectDeleted bool
	}{
		{
			name: "old and unused, delete",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: capi.GroupVersionInfrastructure.Version,
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "template-d",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"creationTimestamp": "2026-09-30T14:29:00Z",
						"labels": map[string]interface{}{
							CleanupEnabledLabelKey: "true",
							capr.ClusterNameLabel:  cluster.Name,
						},
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       capiCluster.Name,
								"uid":        string(capiCluster.UID),
							},
						},
					},
				},
			},
			expectDeleted: true,
		},
		{
			name: "recent and unused, don't delete",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: capi.GroupVersionInfrastructure.Version,
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "template-d",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"creationTimestamp": "2026-09-30T15:00:00Z",
						"labels": map[string]interface{}{
							CleanupEnabledLabelKey: "true",
							capr.ClusterNameLabel:  cluster.Name,
						},
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       capiCluster.Name,
								"uid":        string(capiCluster.UID),
							},
						},
					},
				},
			},
			expectDeleted: false,
		},
		{
			name: "used by prov cluster, don't delete",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: capi.GroupVersionInfrastructure.Version,
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "template-a",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"creationTimestamp": "2026-09-30T14:29:00Z",
						"labels": map[string]interface{}{
							CleanupEnabledLabelKey: "true",
							capr.ClusterNameLabel:  cluster.Name,
						},
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       capiCluster.Name,
								"uid":        string(capiCluster.UID),
							},
						},
					},
				},
			},
			expectDeleted: false,
		},
		{
			name: "used by md, don't delete",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: capi.GroupVersionInfrastructure.Version,
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "template-b",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"creationTimestamp": "2026-09-30T14:29:00Z",
						"labels": map[string]interface{}{
							CleanupEnabledLabelKey: "true",
							capr.ClusterNameLabel:  cluster.Name,
						},
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       capiCluster.Name,
								"uid":        string(capiCluster.UID),
							},
						},
					},
				},
			},
			expectDeleted: false,
		},
		{
			name: "used by ms, don't delete",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": schema.GroupVersion{
						Group:   capi.GroupVersionInfrastructure.Group,
						Version: capi.GroupVersionInfrastructure.Version,
					}.String(),
					"kind": "AWSMachineTemplate",
					"metadata": map[string]interface{}{
						"name":              "template-c",
						"namespace":         "fleet-default",
						"uid":               "uid-1234",
						"creationTimestamp": "2026-09-30T14:29:00Z",
						"labels": map[string]interface{}{
							CleanupEnabledLabelKey: "true",
							capr.ClusterNameLabel:  cluster.Name,
						},
						"ownerReferences": []interface{}{
							map[string]interface{}{
								"apiVersion": "cluster.x-k8s.io/v1beta2",
								"kind":       "Cluster",
								"name":       capiCluster.Name,
								"uid":        string(capiCluster.UID),
							},
						},
					},
				},
			},
			expectDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			scheme := runtime.NewScheme()
			dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
				templateGVR: templateGVK.Kind + "List",
			})

			h.dynamicClient = dynamicClient

			infraClusterClient := dynamicClient.Resource(templateGVR).Namespace(namespace)

			_, err = infraClusterClient.Create(t.Context(), tt.obj, metav1.CreateOptions{})
			require.NoError(t, err)

			dynamicClient.ClearActions()

			err = h.cleanupInfraMachineTemplates(cluster, now)
			require.NoError(t, err)

			expectedActionLen := 1
			if tt.expectDeleted {
				expectedActionLen++
			}

			require.Len(t, dynamicClient.Actions(), expectedActionLen)

			assert.Equal(t, "list", dynamicClient.Actions()[0].GetVerb())

			if tt.expectDeleted {
				assert.Equal(t, "delete", dynamicClient.Actions()[1].GetVerb())
			}
		})
	}
}
