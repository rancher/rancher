package bootstrap

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	ctrlfake "github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_getBootstrapSecret(t *testing.T) {
	type args struct {
		secretName    string
		os            string
		namespaceName string
		path          string
		command       string
		body          string
		provider      string
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
		{
			name: "Checking Linux Install Script cloud-config",
			args: args{
				os:            capr.DefaultMachineOS,
				secretName:    "mybestlinuxsecret",
				command:       "sh",
				namespaceName: "myfavoritelinuxnamespace",
				path:          "/system-agent-install.sh",
				body:          "#!/usr/bin/env sh",
				provider:      "test",
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
				rkeBootstrap:        getBootstrapControllerMock(ctrl, tt.args.namespaceName, tt.args.os),
				k8s:                 fake.NewSimpleClientset(),
			}

			//act
			err := settings.ServerURL.Set("localhost")
			a.Nil(err)

			serviceAccount, err := handler.serviceAccountCache.Get(tt.args.namespaceName, tt.args.secretName)
			a.Nil(err)
			machine, err := handler.machineCache.Get(tt.args.namespaceName, tt.args.os)
			a.Nil(err)
			bootstrap, err := handler.rkeBootstrap.Get(tt.args.namespaceName, tt.args.os, metav1.GetOptions{})
			a.Nil(err)
			if tt.args.provider != "" {
				bootstrap.Annotations[capr.ProviderIDPrefixAnnotation] = tt.args.provider
			}
			secret, err := handler.getBootstrapSecret(tt.args.namespaceName, tt.args.secretName, &rkev1.RKEControlPlane{}, machine, bootstrap)

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
			if tt.args.provider != "" {
				a.Equal(data, `## template: jinja
#cloud-config
runcmd:
- sh /usr/local/custom_script/install.sh
write_files:
- content: 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
  encoding: gzip+b64
  path: /usr/local/custom_script/install.sh
  permissions: "0644"
- content: 'kubelet-arg+: ''provider-id=test://{{ ds.meta_data["instance_id"] }}'''
  path: /etc/rancher/rke2/config.yaml.d/40-provider-id.yaml
  permissions: "0644"
`)
				return
			}
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

func getBootstrapControllerMock(ctrl *gomock.Controller, namespace, os string) *ctrlfake.MockControllerInterface[*rkev1.RKEBootstrap, *rkev1.RKEBootstrapList] {
	mockBootstrapController := ctrlfake.NewMockControllerInterface[*rkev1.RKEBootstrap, *rkev1.RKEBootstrapList](ctrl)
	mockBootstrapController.EXPECT().Get(namespace, capr.DefaultMachineOS, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*rkev1.RKEBootstrap, error) {
		return &rkev1.RKEBootstrap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      os,
				Namespace: namespace,
				Labels: map[string]string{
					capr.ControlPlaneRoleLabel: "true",
					capr.EtcdRoleLabel:         "true",
					capr.WorkerRoleLabel:       "true",
					capr.CattleOSLabel:         os,
				},
				Annotations: map[string]string{},
			},
		}, nil
	}).AnyTimes()
	mockBootstrapController.EXPECT().Get(namespace, capr.WindowsMachineOS, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*rkev1.RKEBootstrap, error) {
		return &rkev1.RKEBootstrap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      os,
				Namespace: namespace,
				Labels: map[string]string{
					capr.ControlPlaneRoleLabel: "false",
					capr.EtcdRoleLabel:         "false",
					capr.WorkerRoleLabel:       "true",
					capr.CattleOSLabel:         os,
				},
				Annotations: map[string]string{},
			},
		}, nil
	}).AnyTimes()
	return mockBootstrapController
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

func getSecretCacheMock(ctrl *gomock.Controller, namespace, secretName string) *ctrlfake.MockCacheInterface[*v1.Secret] {
	mockSecretCache := ctrlfake.NewMockCacheInterface[*v1.Secret](ctrl)
	mockSecretCache.EXPECT().Get(namespace, secretName).DoAndReturn(func(namespace, name string) (*v1.Secret, error) {
		return &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": secretName,
				},
			},
			Immutable: nil,
			Data: map[string][]byte{
				"token": []byte("thisismytokenandiwillprotectit"),
			},
			StringData: nil,
			Type:       "kubernetes.io/service-account-token",
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
