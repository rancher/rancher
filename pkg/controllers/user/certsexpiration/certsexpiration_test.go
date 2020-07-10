package certsexpiration

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
)

func TestDeleteUnusedCerts(t *testing.T) {
	tests := []struct {
		name                          string
		certs                         map[string]v3.CertExpiration
		rancherKubernetesEngineConfig *rketypes.RancherKubernetesEngineConfig
		expectNewCerts                map[string]v3.CertExpiration
	}{
		{
			name: "Keep valid etcd certs",
			certs: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3": v3.CertExpiration{},
				"kube-etcd-172-17-0-4": v3.CertExpiration{},
				"kube-etcd-172-17-0-5": v3.CertExpiration{},
				"kube-node":            v3.CertExpiration{},
				"kube-apiserver":       v3.CertExpiration{},
				"kube-proxy":           v3.CertExpiration{},
			},
			rancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
				Services: rketypes.RKEConfigServices{
					Kubelet: rketypes.KubeletService{
						GenerateServingCertificate: true,
					},
				},
				Nodes: []rketypes.RKEConfigNode{
					{
						Address: "172.17.0.3",
						Role: []string{
							"etcd",
						},
					},
					{
						Address: "172.17.0.4",
						Role: []string{
							"etcd",
						},
					},
					{
						Address: "172.17.0.5",
						Role: []string{
							"etcd",
						},
					},
				},
			},
			expectNewCerts: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3": v3.CertExpiration{},
				"kube-etcd-172-17-0-4": v3.CertExpiration{},
				"kube-etcd-172-17-0-5": v3.CertExpiration{},
				"kube-node":            v3.CertExpiration{},
				"kube-apiserver":       v3.CertExpiration{},
				"kube-proxy":           v3.CertExpiration{},
			},
		},
		{
			name: "Keep valid kubelet certs",
			certs: map[string]v3.CertExpiration{
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-kubelet-172-17-0-3": v3.CertExpiration{},
				"kube-kubelet-172-17-0-5": v3.CertExpiration{},
				"kube-etcd-172-17-0-5":    v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
			rancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
				Services: rketypes.RKEConfigServices{
					Kubelet: rketypes.KubeletService{
						GenerateServingCertificate: true,
					},
				},
				Nodes: []rketypes.RKEConfigNode{
					{
						Address: "172.17.0.3",
						Role: []string{
							"worker",
						},
					},
					{
						Address: "172.17.0.4",
						Role: []string{
							"worker",
						},
					},
					{
						Address: "172.17.0.5",
						Role: []string{
							"etcd",
						},
					},
				},
			},
			expectNewCerts: map[string]v3.CertExpiration{
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-kubelet-172-17-0-3": v3.CertExpiration{},
				"kube-kubelet-172-17-0-5": v3.CertExpiration{},
				"kube-etcd-172-17-0-5":    v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
		},
		{
			name: "Remove unused etcd certs",
			certs: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3":    v3.CertExpiration{},
				"kube-etcd-172-17-0-4":    v3.CertExpiration{},
				"kube-etcd-172-17-0-5":    v3.CertExpiration{},
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-kubelet-172-17-0-5": v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
			rancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
				Services: rketypes.RKEConfigServices{
					Kubelet: rketypes.KubeletService{
						GenerateServingCertificate: true,
					},
				},
				Nodes: []rketypes.RKEConfigNode{
					{
						Address: "172.17.0.5",
						Role: []string{
							"etcd",
							"woker",
						},
					},
					{
						Address: "172.17.0.4",
						Role: []string{
							"woker",
						},
					},
				},
			},
			expectNewCerts: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-5":    v3.CertExpiration{},
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-kubelet-172-17-0-5": v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
		},
		{
			name: "Remove unused kubelet certs",
			certs: map[string]v3.CertExpiration{
				"kube-kubelet-172-17-0-1": v3.CertExpiration{},
				"kube-etcd-172-17-0-3":    v3.CertExpiration{},
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-3": v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
			rancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
				Services: rketypes.RKEConfigServices{
					Kubelet: rketypes.KubeletService{
						GenerateServingCertificate: true,
					},
				},
				Nodes: []rketypes.RKEConfigNode{
					{
						Address: "172.17.0.3",
						Role: []string{
							"etcd",
							"woker",
						},
					},
					{
						Address: "172.17.0.4",
						Role: []string{
							"woker",
						},
					},
				},
			},
			expectNewCerts: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3":    v3.CertExpiration{},
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-3": v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
		},
		{
			name: "Clean up kubelet certs when GenerateServingCertificate is disabled",
			certs: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3":    v3.CertExpiration{},
				"kube-node":               v3.CertExpiration{},
				"kube-kubelet-172-17-0-3": v3.CertExpiration{},
				"kube-kubelet-172-17-0-4": v3.CertExpiration{},
				"kube-apiserver":          v3.CertExpiration{},
				"kube-proxy":              v3.CertExpiration{},
			},
			rancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
				Services: rketypes.RKEConfigServices{
					Kubelet: rketypes.KubeletService{
						GenerateServingCertificate: false,
					},
				},
				Nodes: []rketypes.RKEConfigNode{
					{
						Address: "172.17.0.3",
						Role: []string{
							"etcd",
							"woker",
						},
					},
					{
						Address: "172.17.0.4",
						Role: []string{
							"woker",
						},
					},
				},
			},
			expectNewCerts: map[string]v3.CertExpiration{
				"kube-etcd-172-17-0-3": v3.CertExpiration{},
				"kube-node":            v3.CertExpiration{},
				"kube-apiserver":       v3.CertExpiration{},
				"kube-proxy":           v3.CertExpiration{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteUnusedCerts(tt.certs, tt.rancherKubernetesEngineConfig)
			assert.Equal(t, true, reflect.DeepEqual(tt.certs, tt.expectNewCerts))
		})
	}
}
