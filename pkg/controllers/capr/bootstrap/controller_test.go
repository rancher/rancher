package bootstrap

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func Test_getBootstrapSecret(t *testing.T) {
	type args struct {
		secretName    string
		os            string
		namespaceName string
		path          string
		command       string
		body          string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking Linux Install Script",
			args: args{
				os:            capr.DefaultMachineOS,
				secretName:    "mybestlinuxsecret",
				command:       "sh",
				namespaceName: "myfavoritelinuxnamespace",
				path:          "/system-agent-install.sh",
				body:          "#!/usr/bin/env sh",
			},
		},
		{
			name: "Checking Windows Install Script",
			args: args{
				os:            capr.WindowsMachineOS,
				secretName:    "mybestwindowssecret",
				command:       "powershell",
				namespaceName: "myfavoritewindowsnamespace",
				path:          "/wins-agent-install.ps1",
				body:          "Invoke-WinsInstaller @PSBoundParameters",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			expectHash := sha256.Sum256([]byte("thisismytokenandiwillprotectit"))
			expectEncodedHash := base64.URLEncoding.EncodeToString(expectHash[:])
			a := assert.New(t)
			ctrl := gomock.NewController(t)
			handler := handler{
				serviceAccountCache: getServiceAccountCacheMock(ctrl, tt.args.namespaceName, tt.args.secretName),
				secretCache:         getSecretCacheMock(ctrl, tt.args.namespaceName, tt.args.secretName),
				deploymentCache:     getDeploymentCacheMock(ctrl),
				machineCache:        getMachineCacheMock(ctrl, tt.args.namespaceName, tt.args.os),
				k8s:                 fake.NewSimpleClientset(),
			}

			//act
			err := settings.ServerURL.Set("localhost")
			a.Nil(err)
			err = settings.SystemAgentInstallScript.Set("https://raw.githubusercontent.com/rancher/system-agent/main/install.sh")
			a.Nil(err)
			err = settings.SystemAgentInstallerImage.Set("rancher/system-agent-installer-")
			a.Nil(err)

			serviceAccount, err := handler.serviceAccountCache.Get(tt.args.namespaceName, tt.args.secretName)
			a.Nil(err)
			machine, err := handler.machineCache.Get(tt.args.namespaceName, tt.args.os)
			a.Nil(err)
			secret, err := handler.getBootstrapSecret(tt.args.namespaceName, tt.args.secretName, []v1.EnvVar{}, machine, nil, "")
			a.Nil(err)

			// assert
			a.NotNil(secret)
			a.NotNil(serviceAccount)
			a.NotNil(machine)
			a.NotNil(expectHash)
			a.NotEmpty(expectEncodedHash)

			a.Equal(tt.args.secretName, secret.Name)
			a.Equal(tt.args.namespaceName, secret.Namespace)
			a.Equal(tt.args.secretName, serviceAccount.Name)
			a.Equal(tt.args.namespaceName, serviceAccount.Namespace)
			a.Equal(tt.args.os, machine.Name)
			a.Equal(tt.args.namespaceName, machine.Namespace)

			a.Equal("rke.cattle.io/bootstrap", string(secret.Type))
			data := string(secret.Data["value"])
			a.Contains(data, fmt.Sprintf("CATTLE_TOKEN=\"%s\"", expectEncodedHash))

			switch tt.args.os {

			case capr.DefaultMachineOS:
				a.Equal(tt.args.os, capr.DefaultMachineOS)
				a.Contains(data, "#!/usr/bin")
				a.True(machine.GetLabels()[capr.CattleOSLabel] == capr.DefaultMachineOS)
				a.True(machine.GetLabels()[capr.ControlPlaneRoleLabel] == "true")
				a.True(machine.GetLabels()[capr.EtcdRoleLabel] == "true")
				a.True(machine.GetLabels()[capr.WorkerRoleLabel] == "true")
				a.Contains(data, "CATTLE_SERVER=localhost")
				a.Contains(data, "CATTLE_ROLE_NONE=true")

			case capr.WindowsMachineOS:
				a.Equal(tt.args.os, capr.WindowsMachineOS)
				a.Contains(data, "Invoke-WinsInstaller")
				a.True(machine.GetLabels()[capr.CattleOSLabel] == capr.WindowsMachineOS)
				a.True(machine.GetLabels()[capr.ControlPlaneRoleLabel] == "false")
				a.True(machine.GetLabels()[capr.EtcdRoleLabel] == "false")
				a.True(machine.GetLabels()[capr.WorkerRoleLabel] == "true")
				a.Contains(data, "$env:CATTLE_SERVER=\"localhost\"")
				a.Contains(data, "CATTLE_ROLE_NONE=\"true\"")
				a.Contains(data, "$env:CSI_PROXY_URL")
				a.Contains(data, "$env:CSI_PROXY_VERSION")
				a.Contains(data, "$env:CSI_PROXY_KUBELET_PATH")
			}
		})
	}
}

func getMachineCacheMock(ctrl *gomock.Controller, namespace, os string) *ctrlfake.MockCacheInterface[*capi.Machine] {
	mockMachineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
	mockMachineCache.EXPECT().Get(namespace, capr.DefaultMachineOS).DoAndReturn(func(namespace, name string) (*capi.Machine, error) {
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      os,
				Namespace: namespace,
				Labels: map[string]string{
					capr.ControlPlaneRoleLabel: "true",
					capr.EtcdRoleLabel:         "true",
					capr.WorkerRoleLabel:       "true",
					capr.CattleOSLabel:         os,
				},
			},
			Spec: capi.MachineSpec{
				InfrastructureRef: capi.ContractVersionedObjectReference{
					APIGroup: capr.RKEMachineAPIGroup,
				},
			},
		}, nil
	}).AnyTimes()

	mockMachineCache.EXPECT().Get(namespace, capr.WindowsMachineOS).DoAndReturn(func(namespace, name string) (*capi.Machine, error) {
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      os,
				Namespace: namespace,
				Labels: map[string]string{
					capr.ControlPlaneRoleLabel: "false",
					capr.EtcdRoleLabel:         "false",
					capr.WorkerRoleLabel:       "true",
					capr.CattleOSLabel:         os,
				},
			},
			Spec: capi.MachineSpec{
				InfrastructureRef: capi.ContractVersionedObjectReference{
					APIGroup: capr.RKEMachineAPIGroup,
				},
			},
		}, nil
	}).AnyTimes()
	return mockMachineCache
}

func getDeploymentCacheMock(ctrl *gomock.Controller) *ctrlfake.MockCacheInterface[*v1apps.Deployment] {
	mockDeploymentCache := ctrlfake.NewMockCacheInterface[*v1apps.Deployment](ctrl)
	mockDeploymentCache.EXPECT().Get(namespace.System, "rancher").DoAndReturn(func(namespace, name string) (*v1apps.Deployment, error) {
		return &v1apps.Deployment{
			Spec: v1apps.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name: "rancher",
								Ports: []v1.ContainerPort{
									{
										HostPort: 8080,
									},
								},
							},
						},
					},
				},
			},
		}, nil
	}).AnyTimes()
	return mockDeploymentCache
}

func getSecretCacheMock(ctrl *gomock.Controller, namespace, saName string) *ctrlfake.MockCacheInterface[*v1.Secret] {
	mockSecretCache := ctrlfake.NewMockCacheInterface[*v1.Secret](ctrl)
	selector := labels.Set{"cattle.io/service-account.name": saName}.AsSelector()
	mockSecretCache.EXPECT().List(namespace, selector).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
		return []*v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      saName + "-secret",
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": saName,
					},
					Labels: map[string]string{
						"cattle.io/service-account.name": saName,
					},
				},
				Immutable: nil,
				Data: map[string][]byte{
					"token": []byte("thisismytokenandiwillprotectit"),
				},
				StringData: nil,
				Type:       "kubernetes.io/service-account-token",
			},
		}, nil
	}).AnyTimes()
	return mockSecretCache
}

func getServiceAccountCacheMock(ctrl *gomock.Controller, namespace, name string) *ctrlfake.MockCacheInterface[*v1.ServiceAccount] {
	mockServiceAccountCache := ctrlfake.NewMockCacheInterface[*v1.ServiceAccount](ctrl)
	mockServiceAccountCache.EXPECT().Get(namespace, name).DoAndReturn(func(namespace, name string) (*v1.ServiceAccount, error) {
		return &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Secrets: []v1.ObjectReference{
				{
					Namespace: namespace,
					Name:      name,
				},
			},
		}, nil
	}).AnyTimes()
	return mockServiceAccountCache
}

func TestShouldCreateBootstrapSecret(t *testing.T) {
	tests := []struct {
		phase    capi.MachinePhase
		expected bool
	}{
		{
			phase:    capi.MachinePhasePending,
			expected: true,
		},
		{
			phase:    capi.MachinePhaseProvisioning,
			expected: true,
		},
		{
			phase:    capi.MachinePhaseProvisioned,
			expected: true,
		},
		{
			phase:    capi.MachinePhaseRunning,
			expected: true,
		},
		{
			phase:    capi.MachinePhaseDeleting,
			expected: false,
		},
		{
			phase:    capi.MachinePhaseDeleted,
			expected: false,
		},
		{
			phase:    capi.MachinePhaseFailed,
			expected: false,
		},
		{
			phase:    capi.MachinePhaseUnknown,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			actual := shouldCreateBootstrapSecret(tt.phase)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestReplacementEtcdMachineReady(t *testing.T) {
	newPlanSecret := func(machineName, machineNamespace, joinURL string, labels, annotations map[string]string) *v1.Secret {
		resultLabels := map[string]string{
			capr.MachineNameLabel: machineName,
		}
		for k, v := range labels {
			resultLabels[k] = v
		}
		if machineNamespace != "" {
			resultLabels[capr.MachineNamespaceLabel] = machineNamespace
		}

		resultAnnotations := map[string]string{}
		if joinURL != "" {
			resultAnnotations[capr.JoinURLAnnotation] = joinURL
		}
		for k, v := range annotations {
			resultAnnotations[k] = v
		}

		return &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      resultLabels,
				Annotations: resultAnnotations,
			},
		}
	}
	newReadyNode := func(name string, labels map[string]string) *v1.Node {
		return &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				}},
			},
		}
	}
	newNotReadyNode := func(name string, labels map[string]string) *v1.Node {
		return &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{{
					Type:   v1.NodeReady,
					Status: v1.ConditionFalse,
				}},
			},
		}
	}

	tests := []struct {
		name              string
		planSecrets       []*v1.Secret
		setupMachineCache func(*ctrlfake.MockCacheInterface[*capi.Machine])
		downstreamNodes   []*v1.Node
		expected          bool
		expectErr         bool
	}{
		{
			name: "replacement etcd machine ready through noderef",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-1", "", "https://10.0.0.1:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, nil),
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(&capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-2",
						Namespace: "fleet-default",
						UID:       types.UID("machine-2-uid"),
					},
					Status: capi.MachineStatus{
						NodeRef: capi.MachineNodeReference{Name: "node-2"},
					},
				}, nil)
			},
			downstreamNodes: []*v1.Node{
				newReadyNode("node-2", nil),
			},
			expected: true,
		},
		{
			name: "replacement etcd machine ready through machine uid label fallback",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(&capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-2",
						Namespace: "fleet-default",
						UID:       types.UID("machine-2-uid"),
					},
				}, nil)
			},
			downstreamNodes: []*v1.Node{
				newReadyNode("node-2", map[string]string{
					capr.MachineUIDLabel: "machine-2-uid",
				}),
			},
			expected: true,
		},
		{
			name: "replacement etcd machine missing joinURL",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "replacement etcd machine missing passed probes",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, nil),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "replacement etcd machine node not ready",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(&capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-2",
						Namespace: "fleet-default",
					},
					Status: capi.MachineStatus{
						NodeRef: capi.MachineNodeReference{Name: "node-2"},
					},
				}, nil)
			},
			downstreamNodes: []*v1.Node{
				newNotReadyNode("node-2", nil),
			},
			expected: false,
		},
		{
			name: "replacement etcd machine deleting",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(&capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "machine-2",
						Namespace:         "fleet-default",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
				}, nil)
			},
			expected: false,
		},
		{
			name: "machine cache error bubbles up",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", map[string]string{
					capr.EtcdRoleLabel: "true",
					capr.InitNodeLabel: "true",
				}, map[string]string{
					capr.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
				}),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(nil, fmt.Errorf("boom"))
			},
			expected:  false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
			tt.setupMachineCache(machineCache)

			h := &handler{
				machineCache: machineCache,
			}
			downstream := fake.NewSimpleClientset()

			bootstrap := &rkev1.RKEBootstrap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fleet-default",
				},
				Spec: rkev1.RKEBootstrapSpec{
					ClusterName: "cluster",
				},
			}
			deletingMachine := &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: "fleet-default",
				},
			}

			if len(tt.downstreamNodes) > 0 {
				runtimeObjects := make([]runtime.Object, 0, len(tt.downstreamNodes))
				for _, node := range tt.downstreamNodes {
					runtimeObjects = append(runtimeObjects, node)
				}
				downstream = fake.NewClientset(runtimeObjects...)
			}

			actual, err := h.replacementEtcdMachineReady(bootstrap, deletingMachine, tt.planSecrets, downstream)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}
