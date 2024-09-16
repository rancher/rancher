package planner

import (
	"encoding/base64"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestJoinURLFromAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		port     int
		expected string
	}{
		{
			name:     "ipv4",
			address:  "127.0.0.1",
			port:     9345,
			expected: "https://127.0.0.1:9345",
		},
		{
			name:     "ipv6",
			address:  "::ffff:7f00:1",
			port:     9345,
			expected: "https://[::ffff:7f00:1]:9345",
		},
		{
			name:     "hostname",
			address:  "testing.rancher.io",
			port:     9345,
			expected: "https://testing.rancher.io:9345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, joinURLFromAddress(tt.address, tt.port))
		})
	}
}

// TestJoinURLReconcilation is a unit test to specifically ensure the join URL annotation is properly reconciled on a plan secret.
func TestSetMachineJoinURL(t *testing.T) {
	t.Parallel()

	const rkeBootstrapName = "bogus-rkebootstrap"
	// IPv4 expected IP is 10.20.30.45
	periodicOutputIPv4, err := base64.StdEncoding.DecodeString("H4sIAAAAAAAAA8WQXWvCMBSG/0rJtUJSP8CCF1LnltBW1NqP3HVJhNTOSptYW/G/L07QwXaxu10cOOe8nI/3uQCWHZWuRD/jvBJ1DZwLOGQfAjg/lB6oFS+1MpJoiaIxkjQhHc6P76YmaxePcT47+7kPgzAdLOf7YdClKJj7rR/OpOcSncaowHkpmT1RdIMgTQLoxaMTfd3e9IbGURtFxGctHsctKfnbumFdefK6l8bLsQnfxAotc9wt5zOZbBqZ2ucjjUfwr3NBt5dJgvhuNZ3ePYmqMp5MLs5SuSU35mEPFFmtNpoxY32ni7U+hPILy6KSFtEHa2Ih27GHzgha29C1bGgPzIpdJgsDrX6sWJiG4M9xcDV3FOP9O+UH7WfvG+dfOP3Pz9dPl9MWAykCAAA=")
	if err != nil {
		t.Fatal(err)
	}
	// IPv6 expected IP is [::ffff:7f00:1]
	periodicOutputIPv6, err := base64.StdEncoding.DecodeString("H4sIAAAAAAAAA8WQXYuCQBSG/4p4XTBaCQp7EUqrg6OgpumdzdgyrmY4M5lG/31nC2ph9345N+e8L+fruaq4PHHRV/OSkL5iTLWu6rFsK9X65cxUxkknuLSqEfIi02ixg5NXn/ayhpHtGV69vqAagSDJF6HzuQymXAscNKJkTX0bijzTGq/uKNZNXsQaKHYB8LPVuXjffvtDkaVjmkKER8/IRtgRNxrw1J19jRlhW8jIjUDPB+R0l13SjWj6GLwGMK8NWJmlgmzSOHERDRtGSzcC2EWGP5r7sD7du8OatMhZGygFhrxzEcaQHFLNfPxW9b38TebVhXK7IxICmKlNyXgsMJYIDqKJxDGhdzybnipQHBVT0XRLX1oroGwTW9GBvpAjDiVtJDz2HLGRQkVe7epN7uGYzB+0n9Rf2g/ef/B6+5ebb1+QQBuRMQIAAA==")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		entry         *planEntry
		controlPlane  *rkev1.RKEControlPlane
		capiCluster   *capi.Cluster
		inputSecret   *corev1.Secret
		updatedSecret *corev1.Secret
	}{
		{
			name: "ipv4 rke2 control+etcd no change",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "128.0.0.1",
							},
							capi.MachineAddress{
								Type:    capi.MachineExternalIP,
								Address: "127.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://128.0.0.1:9345",
					},
				},
			},
			updatedSecret: nil,
		},
		{
			name: "ipv4 rke2 control+etcd external IP-only set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineExternalIP,
								Address: "127.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{},
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://127.0.0.1:9345",
					},
				},
			},
		},
		{
			name: "ipv4 rke2 control+etcd set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineExternalIP,
								Address: "127.0.0.1",
							},
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "128.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{},
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://128.0.0.1:9345",
					},
				},
			},
		},
		{
			name: "ipv4 rke2 control+etcd nil NodeInfo",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: nil,
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "128.0.0.1",
							},
							capi.MachineAddress{
								Type:    capi.MachineExternalIP,
								Address: "127.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAutosetDisabled: "true",
						capr.JoinURLAnnotation:      "https://127.0.0.1:9345",
					},
				},
			},
			updatedSecret: nil,
		},
		{
			name: "ipv4 rke2 control+etcd autoset disabled",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: nil,
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "222.0.0.1",
							},
							capi.MachineAddress{
								Type:    capi.MachineExternalIP,
								Address: "223.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{},
				},
			},
			updatedSecret: nil,
		},
		{
			name: "ipv4 rke2 control+etcd no change",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "128.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://128.0.0.1:9345",
					},
				},
			},
			updatedSecret: nil,
		},
		{
			name: "ipv4 k3s control+etcd set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
						Addresses: capi.MachineAddresses{
							capi.MachineAddress{
								Type:    capi.MachineInternalIP,
								Address: "128.0.0.1",
							},
						},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+k3s1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{},
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.ControlPlaneRoleLabel: "true",
						capr.EtcdRoleLabel:         "true",
						capr.WorkerRoleLabel:       "true",
						capr.InitNodeLabel:         "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://128.0.0.1:6443",
					},
				},
			},
		},

		{
			name:  "ipv4 k3s etcd-only no change",
			entry: &planEntry{},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+k3s1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://10.20.30.45:6443",
					},
				},
				Data: map[string][]byte{
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
			updatedSecret: nil,
		},
		{
			name:  "ipv6 k3s etcd-only no change",
			entry: &planEntry{},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+k3s1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://[::ffff:7f00:1]:6443",
					},
				},
				Data: map[string][]byte{
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
			updatedSecret: nil,
		},
		{
			name:  "ipv4 rke2 etcd-only no change",
			entry: &planEntry{},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://10.20.30.45:9345",
					},
				},
				Data: map[string][]byte{
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
			updatedSecret: nil,
		},
		{
			name:  "ipv6 rke2 etcd-only no change",
			entry: &planEntry{},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://[::ffff:7f00:1]:9345",
					},
				},
				Data: map[string][]byte{
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
			updatedSecret: nil,
		},

		{
			name: "ipv4 k3s etcd-only set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+k3s1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://10.20.30.45:6443",
					},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
		},
		{
			name: "ipv6 k3s etcd-only set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+k3s1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://[::ffff:7f00:1]:6443",
					},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
		},
		{
			name: "ipv4 rke2 etcd-only set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://10.20.30.45:9345",
					},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv4,
				},
			},
		},
		{
			name: "ipv6 rke2 etcd-only set url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.EtcdRoleLabel: "true",
						capr.InitNodeLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinURLAnnotation: "https://[::ffff:7f00:1]:9345",
					},
				},
				Data: map[string][]byte{
					"plan":                    []byte("{}"),
					"applied-periodic-output": periodicOutputIPv6,
				},
			},
		},

		{
			name: "ipv4 rke2 worker-only set joined-to url from empty",
			entry: &planEntry{
				Machine: &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Status: capi.MachineStatus{
						NodeInfo: &corev1.NodeSystemInfo{},
					},
					Spec: capi.MachineSpec{
						Bootstrap: capi.Bootstrap{ConfigRef: &corev1.ObjectReference{Kind: "RKEBootstrap", Name: rkeBootstrapName}},
					},
				},
			},
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.5+rke2r1",
				},
			},
			capiCluster: &capi.Cluster{
				Spec: capi.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "something",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "something",
					},
				},
			},
			inputSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.WorkerRoleLabel: "true",
					},
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"plan": []byte("{\"files\":[{\"content\":\"ewogICJzZXJ2ZXIiOiAiaHR0cHM6Ly8yOC44OS45My4yOjkzNDUiCn0=\",\"path\":\"/etc/rancher/k3s/config.yaml.d/50-rancher.yaml\"}]}"),
				},
			},
			updatedSecret: &corev1.Secret{
				Type: capr.SecretTypeMachinePlan,
				ObjectMeta: metav1.ObjectMeta{
					Name:      capr.PlanSecretFromBootstrapName(rkeBootstrapName),
					Namespace: "test",
					Labels: map[string]string{
						capr.WorkerRoleLabel: "true",
					},
					Annotations: map[string]string{
						capr.JoinedToAnnotation: "https://28.89.93.2:9345",
					},
				},
				Data: map[string][]byte{
					"plan": []byte("{\"files\":[{\"content\":\"ewogICJzZXJ2ZXIiOiAiaHR0cHM6Ly8yOC44OS45My4yOjkzNDUiCn0=\",\"path\":\"/etc/rancher/k3s/config.yaml.d/50-rancher.yaml\"}]}"),
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mp := newMockPlanner(t, InfoFunctions{
				SystemAgentImage: func() string { return "system-agent" },
				ImageResolver:    image.ResolveWithControlPlane,
			})

			n, e := SecretToNode(tt.inputSecret)
			assert.NoError(t, e)
			tt.entry.Plan = n
			isc := tt.inputSecret.DeepCopy()
			tt.entry.Metadata = &plan.Metadata{
				Labels:      isc.Labels,
				Annotations: isc.Annotations,
			}

			// if `tt.updatedSecret` is not nil, we expect that `setMachineJoinURL` is going to perform an update of the secret to set the join URL
			if tt.updatedSecret != nil {
				mp.secretClient.EXPECT().Get(tt.inputSecret.Namespace, tt.inputSecret.Name, metav1.GetOptions{}).Return(tt.inputSecret, nil)
				mp.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, tt.updatedSecret, secret)
					return secret, nil
				}).AnyTimes()
			}

			err := mp.planner.store.setMachineJoinURL(tt.entry, tt.capiCluster, tt.controlPlane)
			if tt.updatedSecret == nil {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, generic.ErrSkip, err)
			}
		})
	}
}
