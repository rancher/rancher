package bootstrap

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
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
	"sigs.k8s.io/yaml"
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
			secret, err := handler.getBootstrapSecret(tt.args.namespaceName, tt.args.secretName, []v1.EnvVar{}, bootstrap, machine)
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
			value := secret.Data["value"]

			a.True(strings.HasPrefix(string(value), "## template: jinja\n#cloud-config"))

			data := map[string]any{}
			err = yaml.Unmarshal(value, &data)
			a.Nil(err)

			a.Contains(data, "write_files")
			scriptData := (data["write_files"].([]any)[0]).(map[string]any)

			a.Contains(scriptData, "content")
			content := scriptData["content"].(string)

			a.Equal("gzip+b64", scriptData["encoding"])

			decompressed, err := capr.DecompressBytes(content)
			a.Nil(err)

			decompressedString := string(decompressed)

			a.Contains(decompressedString, fmt.Sprintf("CATTLE_TOKEN=\"%s\"", expectEncodedHash))

			switch tt.args.os {

			case capr.DefaultMachineOS:
				a.Equal(tt.args.os, capr.DefaultMachineOS)
				a.Contains(data, "runcmd")
				a.Equal("sh /usr/local/custom_script/install.sh", data["runcmd"].([]any)[0])
				a.Contains(decompressedString, "#!/usr/bin/env sh")
				a.Contains(decompressedString, "#!/bin/sh")
				a.True(machine.GetLabels()[capr.CattleOSLabel] == capr.DefaultMachineOS)
				a.True(machine.GetLabels()[capr.ControlPlaneRoleLabel] == "true")
				a.True(machine.GetLabels()[capr.EtcdRoleLabel] == "true")
				a.True(machine.GetLabels()[capr.WorkerRoleLabel] == "true")
				a.Contains(decompressedString, "CATTLE_SERVER=localhost")
				a.Contains(decompressedString, "CATTLE_ROLE_NONE=true")
				a.Equal((data["write_files"]).([]any)[1].(map[string]any)["path"].(string), "/etc/rancher/rke2/config.yaml.d/40-provider-id.yaml")
				a.Equal((data["write_files"]).([]any)[1].(map[string]any)["content"].(string), `kubelet-arg+: 'provider-id=digitalocean://{{ ds.meta_data["instance_id"] }}'`)

			case capr.WindowsMachineOS:
				a.Equal(tt.args.os, capr.WindowsMachineOS)
				a.Contains(data, "runcmd")
				a.Equal("powershell C:\\install.ps1", data["runcmd"].([]any)[0])
				a.Contains(decompressedString, "Invoke-WinsInstaller")
				a.True(machine.GetLabels()[capr.CattleOSLabel] == capr.WindowsMachineOS)
				a.True(machine.GetLabels()[capr.ControlPlaneRoleLabel] == "false")
				a.True(machine.GetLabels()[capr.EtcdRoleLabel] == "false")
				a.True(machine.GetLabels()[capr.WorkerRoleLabel] == "true")
				a.Contains(decompressedString, "$env:CATTLE_SERVER=\"localhost\"")
				a.Contains(decompressedString, "CATTLE_ROLE_NONE=\"true\"")
				a.Contains(decompressedString, "$env:CSI_PROXY_URL")
				a.Contains(decompressedString, "$env:CSI_PROXY_VERSION")
				a.Contains(decompressedString, "$env:CSI_PROXY_KUBELET_PATH")
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
			},
			Spec: rkev1.RKEBootstrapSpec{
				Files: []rkev1.CloudInitFile{
					{
						Path:    "/etc/rancher/rke2/config.yaml.d/40-provider-id.yaml",
						Content: `kubelet-arg+: 'provider-id=digitalocean://{{ ds.meta_data["instance_id"] }}'`,
					},
				},
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
