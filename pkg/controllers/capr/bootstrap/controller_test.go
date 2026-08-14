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
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/settings"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
				secretClient:        getSecretClientMock(ctrl),
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

func getSecretClientMock(ctrl *gomock.Controller) *ctrlfake.MockClientInterface[*v1.Secret, *v1.SecretList] {
	mock := ctrlfake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
	mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *v1.Secret) (*v1.Secret, error) {
		return secret, nil
	})
	return mock
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

func TestReconcileMachinePreTerminateAnnotationReleasesMachineWithoutNodeRef(t *testing.T) {
	const (
		namespace     = "fleet-default"
		bootstrapName = "bootstrap-1"
		machineName   = "machine-1"
		clusterName   = "cluster"
	)

	ctrl := gomock.NewController(t)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
	machineClient := ctrlfake.NewMockClientInterface[*capi.Machine, *capi.MachineList](ctrl)

	deletionTime := metav1.NewTime(time.Now())
	machine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              machineName,
			Namespace:         namespace,
			DeletionTimestamp: &deletionTime,
			Labels: map[string]string{
				capr.EtcdRoleLabel: "true",
			},
			Annotations: map[string]string{
				capiMachinePreTerminateAnnotation: capiMachinePreTerminateAnnotationOwner,
			},
		},
	}

	machineCache.EXPECT().Get(namespace, machineName).Return(machine, nil)
	machineClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(updated *capi.Machine) (*capi.Machine, error) {
		assert.NotContains(t, updated.Annotations, capiMachinePreTerminateAnnotation)
		return updated, nil
	})

	h := &handler{
		machineCache:  machineCache,
		machineClient: machineClient,
	}
	bootstrap := &rkev1.RKEBootstrap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: capi.GroupVersion.String(),
				Kind:       "Machine",
				Name:       machineName,
			}},
		},
		Spec: rkev1.RKEBootstrapSpec{
			ClusterName: clusterName,
		},
	}

	result, err := h.reconcileMachinePreTerminateAnnotation(bootstrap)
	assert.NoError(t, err)
	assert.Same(t, bootstrap, result)
}

func TestReplacementEtcdMachineReady(t *testing.T) {
	const (
		deletingMachineName      = "machine-1"
		deletingMachineNamespace = "fleet-default"
		clusterName              = "cluster"
	)

	// planSecretOption changes one property of a replacement candidate.
	type planSecretOption func(*v1.Secret)

	withNoInitNodeLabel := func(s *v1.Secret) { delete(s.Labels, capr.InitNodeLabel) }
	withNoEtcdRoleLabel := func(s *v1.Secret) { delete(s.Labels, capr.EtcdRoleLabel) }
	withNoJoinURL := func(s *v1.Secret) { delete(s.Annotations, capr.JoinURLAnnotation) }
	withNoProbesPassedAnnotation := func(s *v1.Secret) { delete(s.Annotations, planapi.PlanProbesPassedAnnotation) }
	withDeletingSecret := func(s *v1.Secret) { s.DeletionTimestamp = &metav1.Time{Time: time.Now()} }
	withWrongType := func(s *v1.Secret) { s.Type = v1.SecretTypeOpaque }

	// newPlanSecret returns a candidate that satisfies all plan-secret checks by default.
	newPlanSecret := func(machineName, machineNamespace, joinURL string, opts ...planSecretOption) *v1.Secret {
		labels := map[string]string{
			capr.MachineNameLabel: machineName,
			capr.EtcdRoleLabel:    "true",
			capr.InitNodeLabel:    "true",
		}
		if machineNamespace != "" {
			labels[capr.MachineNamespaceLabel] = machineNamespace
		}
		annotations := map[string]string{
			planapi.PlanProbesPassedAnnotation: time.Now().UTC().Format(time.RFC3339),
		}
		if joinURL != "" {
			annotations[capr.JoinURLAnnotation] = joinURL
		}
		s := &v1.Secret{
			Type: capr.SecretTypeMachinePlan,
			ObjectMeta: metav1.ObjectMeta{
				Labels:      labels,
				Annotations: annotations,
			},
		}
		for _, opt := range opts {
			opt(s)
		}
		return s
	}

	// newReadyMachine returns a candidate that satisfies all Machine checks by default.
	newReadyMachine := func(name, namespace, clusterName string) *capi.Machine {
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					capr.EtcdRoleLabel: "true",
				},
			},
			Spec: capi.MachineSpec{
				ClusterName: clusterName,
			},
			Status: capi.MachineStatus{
				NodeRef: capi.MachineNodeReference{Name: "node-2"},
				Conditions: []metav1.Condition{{
					Type:   capi.MachineNodeReadyCondition,
					Status: metav1.ConditionTrue,
					Reason: capi.MachineNodeReadyReason,
				}},
			},
		}
	}

	tests := []struct {
		name              string
		planSecrets       []*v1.Secret
		setupMachineCache func(*ctrlfake.MockCacheInterface[*capi.Machine])
		expected          bool
		expectErr         bool
	}{
		{
			name: "fully reconciled elected init etcd candidate is accepted",
			planSecrets: []*v1.Secret{
				newPlanSecret(deletingMachineName, "", "https://10.0.0.1:9345"),
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(newReadyMachine("machine-2", "fleet-default", clusterName), nil)
			},
			expected: true,
		},
		{
			name: "candidate without InitNodeLabel is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withNoInitNodeLabel),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "candidate without EtcdRoleLabel on plan secret is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withNoEtcdRoleLabel),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "candidate missing joinURL is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withNoJoinURL),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "candidate missing PlanProbesPassedAnnotation is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withNoProbesPassedAnnotation),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "deleting plan secret is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withDeletingSecret),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "wrong secret type is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345", withWrongType),
			},
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "machine from another cluster is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(newReadyMachine("machine-2", "fleet-default", "other-cluster"), nil)
			},
			expected: false,
		},
		{
			name: "machine without the etcd role is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				delete(m.Labels, capr.EtcdRoleLabel)
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "missing NodeReady condition is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				m.Status.Conditions = nil
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "false NodeReady condition is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				m.Status.Conditions = []metav1.Condition{{
					Type:   capi.MachineNodeReadyCondition,
					Status: metav1.ConditionFalse,
					Reason: capi.MachineNodeNotReadyReason,
				}}
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "unknown NodeReady condition is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				m.Status.Conditions = []metav1.Condition{{
					Type:   capi.MachineNodeReadyCondition,
					Status: metav1.ConditionUnknown,
					Reason: capi.MachineNodeReadyUnknownReason,
				}}
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "missing NodeRef is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				m.Status.NodeRef = capi.MachineNodeReference{}
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "deleting candidate machine is rejected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
			},
			setupMachineCache: func(machineCache *ctrlfake.MockCacheInterface[*capi.Machine]) {
				m := newReadyMachine("machine-2", "fleet-default", clusterName)
				m.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				machineCache.EXPECT().Get("fleet-default", "machine-2").Return(m, nil)
			},
			expected: false,
		},
		{
			name: "deleting machine is never selected as its own replacement",
			planSecrets: []*v1.Secret{
				// The deleting machine otherwise satisfies every candidate check.
				newPlanSecret(deletingMachineName, deletingMachineNamespace, "https://10.0.0.1:9345"),
			},
			// Reject the candidate before reading the machine cache.
			setupMachineCache: func(_ *ctrlfake.MockCacheInterface[*capi.Machine]) {},
			expected:          false,
		},
		{
			name: "machine cache error propagates",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "", "https://10.0.0.2:9345"),
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

			bootstrap := &rkev1.RKEBootstrap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: deletingMachineNamespace,
				},
				Spec: rkev1.RKEBootstrapSpec{
					ClusterName: clusterName,
				},
			}
			deletingMachine := &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deletingMachineName,
					Namespace: deletingMachineNamespace,
				},
				Spec: capi.MachineSpec{
					ClusterName: clusterName,
				},
			}

			actual, err := h.replacementEtcdMachineReady(bootstrap, deletingMachine, tt.planSecrets)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestMachineStillJoinedToJoinURL(t *testing.T) {
	newPlanSecret := func(machineName, joinedTo string) *v1.Secret {
		annotations := map[string]string{}
		if joinedTo != "" {
			annotations[capr.JoinedToAnnotation] = joinedTo
		}
		return &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					capr.MachineNameLabel: machineName,
				},
				Annotations: annotations,
			},
		}
	}

	tests := []struct {
		name         string
		planSecrets  []*v1.Secret
		joinURL      string
		expectedName string
		expectedOk   bool
	}{
		{
			name: "matching non-empty URL is detected",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "https://10.0.0.1:9345"),
			},
			joinURL:      "https://10.0.0.1:9345",
			expectedName: "machine-2",
			expectedOk:   true,
		},
		{
			name: "non-matching URL is ignored",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "https://10.0.0.9:9345"),
			},
			joinURL:    "https://10.0.0.1:9345",
			expectedOk: false,
		},
		{
			name: "empty join URL does not match",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", "https://10.0.0.1:9345"),
			},
			joinURL:    "",
			expectedOk: false,
		},
		{
			name: "empty joined-to annotation does not match",
			planSecrets: []*v1.Secret{
				newPlanSecret("machine-2", ""),
			},
			joinURL:    "https://10.0.0.1:9345",
			expectedOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ok := machineStillJoinedToJoinURL(tt.planSecrets, tt.joinURL)
			assert.Equal(t, tt.expectedOk, ok)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}
