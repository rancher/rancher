package machineprovisioning

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_Operation_SetC_MP_DataDirectories(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mp-data-directories",
		},
		Spec: provisioningv1.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					DataDirectories: rkev1.DataDirectories{
						// Should be a path under "/var/lib/rancher" since it must be a volume
						SystemAgent:  "/var/lib/rancher/testing/system-agent",
						Provisioning: "/var/lib/rancher/testing/provisioning",
						K8sDistro:    "/var/lib/rancher/testing/k8s-distro",
					},
				},
				MachinePools: []provisioningv1.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		logrus.Errorf("failed to get machines for %s/%s to print error: %v", c.Namespace, c.Name, err)
	} else {
		for _, machine := range machines.Items {
			im, newErr := clients.Dynamic.Resource(schema.GroupVersionResource{
				Group:    machine.Spec.InfrastructureRef.GroupVersionKind().Group,
				Version:  machine.Spec.InfrastructureRef.GroupVersionKind().Version,
				Resource: strings.ToLower(fmt.Sprintf("%ss", machine.Spec.InfrastructureRef.GroupVersionKind().Kind)),
			}).Namespace(machine.Spec.InfrastructureRef.Namespace).Get(context.TODO(), machine.Spec.InfrastructureRef.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Errorf("failed to get %s %s/%s to print error: %v", machine.Spec.InfrastructureRef.GroupVersionKind().String(), machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name, newErr)
			} else if machine.Spec.InfrastructureRef.GroupVersionKind().Kind == "PodMachine" {
				// In the case of a podmachine, the pod name will be strings.ReplaceAll(infra.meta.GetName(), ".", "-")
				podName := strings.ReplaceAll(im.GetName(), ".", "-")
				validateDirectory(t, im.GetNamespace(), podName, c.Spec.RKEConfig.DataDirectories.SystemAgent)
				validateDirectory(t, im.GetNamespace(), podName, c.Spec.RKEConfig.DataDirectories.Provisioning)
				validateDirectory(t, im.GetNamespace(), podName, c.Spec.RKEConfig.DataDirectories.K8sDistro)
			}
		}
	}
}

func validateDirectory(t *testing.T, namespace string, name, path string) {
	kcp := []string{
		"-n",
		namespace,
		"exec",
		name,
		"--",
		"test",
		"-d",
		path,
	}

	cmd := exec.Command("kubectl", kcp...)
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		t.Errorf("directory %s should not exist for pod %s/%s", path, namespace, name)
		t.Fail()
	} else if err != nil {
		t.Fatal(fmt.Errorf("failed to exec kubectl command: %v", cmd))
	}
}
