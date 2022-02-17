package bootstrap

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"

	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/namespace"
	v1wa "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	v1w "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

type serviceAccountCacheMock struct{ mock.Mock }

func (s *serviceAccountCacheMock) Get(namespace, name string) (*v1.ServiceAccount, error) {
	args := s.Called(namespace, name)
	return args.Get(0).(*v1.ServiceAccount), args.Error(1)
}
func (s *serviceAccountCacheMock) List(namespace string, selector labels.Selector) ([]*v1.ServiceAccount, error) {
	args := s.Called(namespace, selector)
	return args.Get(0).([]*v1.ServiceAccount), args.Error(1)
}
func (s *serviceAccountCacheMock) AddIndexer(indexName string, indexer v1w.ServiceAccountIndexer) {}
func (s *serviceAccountCacheMock) GetByIndex(indexName, key string) ([]*v1.ServiceAccount, error) {
	args := s.Called(indexName, key)
	return args.Get(0).([]*v1.ServiceAccount), args.Error(1)
}

type secretCacheMock struct{ mock.Mock }

func (s *secretCacheMock) Get(namespace, name string) (*v1.Secret, error) {
	args := s.Called(namespace, name)
	return args.Get(0).(*v1.Secret), args.Error(1)
}
func (s *secretCacheMock) List(namespace string, selector labels.Selector) ([]*v1.Secret, error) {
	args := s.Called(namespace, selector)
	return args.Get(0).([]*v1.Secret), args.Error(1)
}
func (s *secretCacheMock) AddIndexer(indexName string, indexer v1w.SecretIndexer) {}
func (s *secretCacheMock) GetByIndex(indexName, key string) ([]*v1.Secret, error) {
	args := s.Called(indexName, key)
	return args.Get(0).([]*v1.Secret), args.Error(1)
}

type deploymentCacheMock struct{ mock.Mock }

func (d *deploymentCacheMock) Get(namespace, name string) (*v1apps.Deployment, error) {
	args := d.Called(namespace, name)
	return args.Get(0).(*v1apps.Deployment), args.Error(1)
}
func (d *deploymentCacheMock) List(namespace string, selector labels.Selector) ([]*v1apps.Deployment, error) {
	args := d.Called(namespace, selector)
	return args.Get(0).([]*v1apps.Deployment), args.Error(1)
}
func (d *deploymentCacheMock) AddIndexer(indexName string, indexer v1wa.DeploymentIndexer) {}
func (d *deploymentCacheMock) GetByIndex(indexName, key string) ([]*v1apps.Deployment, error) {
	args := d.Called(indexName, key)
	return args.Get(0).([]*v1apps.Deployment), args.Error(1)
}

type machineCacheMock struct{ mock.Mock }

func (m *machineCacheMock) Get(namespace, name string) (*v1beta1.Machine, error) {
	args := m.Called(namespace, name)
	return args.Get(0).(*v1beta1.Machine), args.Error(1)
}
func (m *machineCacheMock) List(namespace string, selector labels.Selector) ([]*v1beta1.Machine, error) {
	args := m.Called(namespace, selector)
	return args.Get(0).([]*v1beta1.Machine), args.Error(1)
}
func (m *machineCacheMock) AddIndexer(indexName string, indexer capicontrollers.MachineIndexer) {}
func (m *machineCacheMock) GetByIndex(indexName, key string) ([]*v1beta1.Machine, error) {
	args := m.Called(indexName, key)
	return args.Get(0).([]*v1beta1.Machine), args.Error(1)
}

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
				os:            rke2.DefaultMachineOS,
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
				os:            rke2.WindowsMachineOS,
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

			handler := handler{
				serviceAccountCache: getServiceAccountCacheMock(tt.args.namespaceName, tt.args.secretName),
				secretCache:         getSecretCacheMock(tt.args.namespaceName, tt.args.secretName),
				deploymentCache:     getDeploymentCacheMock(),
				machineCache:        getMachineCacheMock(tt.args.namespaceName, tt.args.os),
			}

			//act
			serviceAccount, err := handler.serviceAccountCache.Get(tt.args.namespaceName, tt.args.secretName)
			machine, err := handler.machineCache.Get(tt.args.namespaceName, tt.args.os)
			secret, err := handler.getBootstrapSecret(tt.args.namespaceName, tt.args.secretName, []v1.EnvVar{}, machine)

			// assert
			a.Nil(err)
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
			a.Contains(data, "CATTLE_ROLE_NONE=true")
			a.Contains(data, fmt.Sprintf("CATTLE_TOKEN=\"%s\"", expectEncodedHash))

			switch tt.args.os {

			case rke2.DefaultMachineOS:
				a.Equal(tt.args.os, rke2.DefaultMachineOS)
				a.Contains(data, "#!/usr/bin")
				a.True(machine.GetLabels()[rke2.CattleOSLabel] == rke2.DefaultMachineOS)
				a.True(machine.GetLabels()[rke2.ControlPlaneRoleLabel] == "true")
				a.True(machine.GetLabels()[rke2.EtcdRoleLabel] == "true")
				a.True(machine.GetLabels()[rke2.WorkerRoleLabel] == "true")

			case rke2.WindowsMachineOS:
				a.Equal(tt.args.os, rke2.WindowsMachineOS)
				a.Contains(data, "Invoke-WinsInstaller")
				a.True(machine.GetLabels()[rke2.CattleOSLabel] == rke2.WindowsMachineOS)
				a.True(machine.GetLabels()[rke2.ControlPlaneRoleLabel] == "false")
				a.True(machine.GetLabels()[rke2.EtcdRoleLabel] == "false")
				a.True(machine.GetLabels()[rke2.WorkerRoleLabel] == "true")
			}
		})
	}
}

func getMachineCacheMock(namespace, os string) *machineCacheMock {
	mockMachineCache := new(machineCacheMock)
	mockMachineCache.On("Get", namespace, rke2.DefaultMachineOS).Return(&v1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os,
			Namespace: namespace,
			Labels: map[string]string{
				rke2.ControlPlaneRoleLabel: "true",
				rke2.EtcdRoleLabel:         "true",
				rke2.WorkerRoleLabel:       "true",
				rke2.CattleOSLabel:         os,
			},
		},
	}, nil)
	mockMachineCache.On("Get", namespace, rke2.WindowsMachineOS).Return(&v1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os,
			Namespace: namespace,
			Labels: map[string]string{
				rke2.ControlPlaneRoleLabel: "false",
				rke2.EtcdRoleLabel:         "false",
				rke2.WorkerRoleLabel:       "true",
				rke2.CattleOSLabel:         os,
			},
		},
	}, nil)
	return mockMachineCache
}

func getDeploymentCacheMock() *deploymentCacheMock {
	mockDeploymentCache := new(deploymentCacheMock)
	mockDeploymentCache.On("Get", namespace.System, "rancher").Return(&v1apps.Deployment{
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
	}, nil)
	return mockDeploymentCache
}

func getSecretCacheMock(namespace, secretName string) *secretCacheMock {
	mockSecretCache := new(secretCacheMock)
	mockSecretCache.On("Get", namespace, secretName).Return(&v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Immutable: nil,
		Data: map[string][]byte{
			"token": []byte("thisismytokenandiwillprotectit"),
		},
		StringData: nil,
		Type:       "",
	}, nil)
	return mockSecretCache
}

func getServiceAccountCacheMock(namespace, name string) *serviceAccountCacheMock {
	mockServiceAccountCache := new(serviceAccountCacheMock)
	mockServiceAccountCache.On("Get", namespace, name).Return(&v1.ServiceAccount{
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
	}, nil)
	return mockServiceAccountCache
}
