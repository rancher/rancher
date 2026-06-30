package operations

import (
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CAPRKE2Adapter struct {
	controlPlane *unstructured.Unstructured
	clients      *wrangler.CAPIContext
}

func (C CAPRKE2Adapter) WaitForRegister() (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) PauseCluster(pause bool) error {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) RuntimeCommand() string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) ServerUnit() string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) DistroDataDirectory(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) ProvisioningDataDirectory(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) ConfigDirectory(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) RenderProbes(plan *corev1.Secret, supervisor bool) (map[string]planapi.Probe, error) {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) KubectlPath(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) KubeconfigPath(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) FindOrElectLeader(operation string, filter Filter) (*corev1.Secret, error) {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) GetServerURL(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) GetSupervisorPort(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) LoopbackAddress(secret *corev1.Secret) string {
	//TODO implement me
	panic("implement me")
}

func (C CAPRKE2Adapter) ToS3ArgsEnvAndFiles(secret *corev1.Secret) ([]string, []string, []planapi.File) {
	//TODO implement me
	panic("implement me")
}

