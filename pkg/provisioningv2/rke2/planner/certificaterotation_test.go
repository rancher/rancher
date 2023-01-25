package planner

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/stretchr/testify/assert"
)

func Test_shouldRotateEntry(t *testing.T) {
	tests := []struct {
		name               string
		services           []string
		rotateWorker       bool
		rotateControlPlane bool
		rotateETCD         bool
	}{
		{name: "rke2-server", services: []string{"rke2-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "k3s-server", services: []string{"k3s-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "api-server", services: []string{"api-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "kubelet", services: []string{"kubelet"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "kube-proxy", services: []string{"kube-proxy"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "auth-proxy", services: []string{"auth-proxy"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "controller-manager", services: []string{"controller-manager"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "scheduler", services: []string{"scheduler"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "rke2-controller", services: []string{"rke2-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "k3s-controller", services: []string{"k3s-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "admin", services: []string{"admin"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "cloud-controller", services: []string{"cloud-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "etcd", services: []string{"etcd"}, rotateWorker: false, rotateControlPlane: false, rotateETCD: true},
		{name: "many", services: []string{"etcd", "cloud-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: true},
		{name: "none", services: []string{}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
	}

	workerRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{rke2.WorkerRoleLabel: "true"}}}
	controlPlaneRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{rke2.ControlPlaneRoleLabel: "true"}}}
	etcdRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{rke2.EtcdRoleLabel: "true"}}}
	allRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{rke2.WorkerRoleLabel: "true", rke2.ControlPlaneRoleLabel: "true", rke2.EtcdRoleLabel: "true"}}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.rotateWorker, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, workerRoleEntry))
			assert.Equal(t, tt.rotateControlPlane, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, controlPlaneRoleEntry))
			assert.Equal(t, tt.rotateETCD, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, etcdRoleEntry))
			assert.True(t, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, allRoleEntry))
		})
	}
}
