package cluster

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/integration/pkg/namespace"
	"github.com/rancher/rancher/tests/integration/pkg/nodeconfig"
	"github.com/rancher/rancher/tests/integration/pkg/registry"
	"github.com/rancher/rancher/tests/integration/pkg/wait"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func New(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*provisioningv1api.Cluster, error) {
	cluster = cluster.DeepCopy()
	if cluster.Namespace == "" {
		newNs, err := namespace.Random(clients)
		if err != nil {
			return nil, err
		}
		cluster.Namespace = newNs.Name
	}

	if cluster.Name == "" && cluster.GenerateName == "" {
		cluster.GenerateName = "test-cluster-"
	}

	if cluster.Spec.KubernetesVersion == "" {
		cluster.Spec.KubernetesVersion = defaults.SomeK8sVersion
	}

	if cluster.Spec.RKEConfig != nil {
		if cluster.Spec.RKEConfig.MachineGlobalConfig.Data == nil {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data = map[string]interface{}{}
		}
		for k, v := range defaults.CommonClusterConfig {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data[k] = v
		}

		for i, np := range cluster.Spec.RKEConfig.MachinePools {
			if np.NodeConfig == nil {
				podConfig, err := nodeconfig.NewPodConfig(clients, cluster.Namespace)
				if err != nil {
					return nil, err
				}
				cluster.Spec.RKEConfig.MachinePools[i].NodeConfig = podConfig
			}
			if np.Name == "" {
				cluster.Spec.RKEConfig.MachinePools[i].Name = fmt.Sprintf("pool-%d", i)
			}
		}

		if cluster.Spec.RKEConfig.Registries == nil {
			registryConfig, err := registry.GetCache(clients, cluster.Namespace)
			if err != nil {
				return nil, err
			}
			cluster.Spec.RKEConfig.Registries = &registryConfig
		}
	}

	c, err := clients.Provisioning.Cluster().Create(cluster)
	if err != nil {
		return nil, err
	}

	clients.OnClose(func() {
		clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	})

	return c, nil
}

func Machines(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*capi.MachineList, error) {
	return clients.CAPI.Machine().List(cluster.Namespace, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func MachineSets(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*unstructured.UnstructuredList, error) {
	return clients.Dynamic.Resource(schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "machinesets",
	}).Namespace(cluster.Namespace).List(clients.Ctx, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func PodInfraMachines(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*unstructured.UnstructuredList, error) {
	return clients.Dynamic.Resource(schema.GroupVersionResource{
		Group:    "rke-machine.cattle.io",
		Version:  "v1",
		Resource: "podmachines",
	}).Namespace(cluster.Namespace).List(clients.Ctx, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func WaitForCreate(clients *clients.Clients, c *provisioningv1api.Cluster) (_ *provisioningv1api.Cluster, err error) {
	defer func() {
		if err != nil {
			c, newErr := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Errorf("failed to get cluster %s/%s to print error: %v", c.Namespace, c.Name, err)
				return
			}

			machines, newErr := Machines(clients, c)
			if newErr != nil {
				logrus.Errorf("failed to get machines for %s/%s to print error: %v", c.Namespace, c.Name, err)
			}

			var plans []*corev1.Secret
			for _, machine := range machines.Items {
				if machine.Spec.Bootstrap.ConfigRef == nil {
					continue
				}

				planSAs, err := clients.Core.ServiceAccount().List(machine.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s,%s=%s", rke2.MachineNameLabel, machine.Name,
					rke2.RoleLabel, rke2.RolePlan),
				})
				if err != nil {
					continue
				}

				if len(planSAs.Items) != 1 {
					continue
				}

				secret, err := clients.Core.Secret().Get(machine.Namespace, planSAs.Items[0].Labels[rke2.PlanSecret], metav1.GetOptions{})
				if err == nil {
					plans = append(plans, secret)
				}
			}

			data, _ := json.Marshal(map[string]interface{}{
				"cluster":  c,
				"machines": machines,
				"plans":    plans,
			})
			err = fmt.Errorf("creation wait failed on %s: %w", data, err)
		}
	}()

	err = wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return c.Status.ClusterName != "", nil
	})
	if err != nil {
		return nil, fmt.Errorf("mgmt cluster not assigned: %w", err)
	}

	err = wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return c.Status.Ready, nil
	})
	if err != nil {
		return nil, fmt.Errorf("prov cluster is not ready: %w", err)
	}

	machines, err := Machines(clients, c)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines.Items {
		err = wait.Object(clients.Ctx, clients.CAPI.Machine().Watch, &machine, func(obj runtime.Object) (bool, error) {
			machine = *obj.(*capi.Machine)
			return machine.Status.NodeRef != nil, nil
		})
		if err != nil {
			return nil, fmt.Errorf("noderef not assigned to %s/%s: %w", machine.Namespace, machine.Name, err)
		}
	}

	mgmtCluster, err := clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.Object(clients.Ctx, func(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		return clients.Mgmt.Cluster().Watch(opts)
	}, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return condition.Cond("Ready").IsTrue(mgmtCluster), nil
	})
	if err != nil {
		return nil, fmt.Errorf("mgmt cluster is not ready: %w", err)
	}

	return c, nil
}

func WaitForDelete(clients *clients.Clients, c *provisioningv1api.Cluster) (_ *provisioningv1api.Cluster, err error) {
	defer func() {
		if err != nil {
			newC, newErr := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(newErr) {
				newC = nil
			} else if newErr != nil {
				logrus.Errorf("failed to get cluster %s/%s to print error: %v", c.Namespace, c.Name, err)
				return
			}

			machines, newErr := Machines(clients, c)
			if newErr != nil {
				logrus.Errorf("failed to get machines for %s/%s to print error: %v", c.Namespace, c.Name, err)
			}

			mgmtCluster, newErr := clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
			if apierrors.IsNotFound(newErr) {
				mgmtCluster = nil
			} else if newErr != nil {
				logrus.Errorf("failed to get mgmt cluster %s/%s to print error: %v", c.Namespace, c.Name, err)
			}

			capiCluster, newErr := clients.CAPI.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(newErr) {
				capiCluster = nil
			} else if newErr != nil {
				logrus.Errorf("failed to get capi cluster %s/%s to print error: %v", c.Namespace, c.Name, err)
			}

			data, _ := json.Marshal(map[string]interface{}{
				"cluster":     newC,
				"mgmtCluster": mgmtCluster,
				"capiCluster": capiCluster,
				"machines":    machines,
			})
			err = fmt.Errorf("deletion wait failed on %s: %w", data, err)
		}
	}()

	err = wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return !c.DeletionTimestamp.IsZero(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("cluster not deleted: %w", err)
	}

	machines, err := Machines(clients, c)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines.Items {
		gvk := schema.FromAPIVersionAndKind(machine.Spec.InfrastructureRef.APIVersion, machine.Spec.InfrastructureRef.Kind)
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		}
		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return clients.Dynamic.Resource(gvr).Namespace(machine.Spec.InfrastructureRef.Namespace).Get(clients.Ctx, machine.Spec.InfrastructureRef.Name, metav1.GetOptions{})
		}); err != nil {
			return nil, fmt.Errorf("infra machine %s/%s not deleted: %w", machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name, err)
		}

		if machine.Spec.Bootstrap.ConfigRef != nil {
			if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
				return clients.RKE.RKEBootstrap().Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
			}); err != nil {
				return nil, fmt.Errorf("bootstrap config %s/%s not deleted: %w", machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, err)
			}
		}

		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return clients.Batch.Job().Get(machine.Namespace, machineprovision.GetJobName(machine.Name), metav1.GetOptions{})
		}); err != nil {
			return nil, fmt.Errorf("machine provision job %s/%s not deleted: %w", machine.Namespace, machineprovision.GetJobName(machine.Name), err)
		}

		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return clients.CAPI.Machine().Get(machine.Namespace, machine.Name, metav1.GetOptions{})
		}); err != nil {
			return nil, fmt.Errorf("machine not deleted: %w", err)
		}
	}

	if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	}); err != nil {
		return nil, fmt.Errorf("mgmt cluster not cleaned up: %w", err)
	}

	if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	}); err != nil {
		return nil, fmt.Errorf("cluster not cleaned up: %w", err)
	}

	return c, nil
}

func CustomCommand(clients *clients.Clients, c *provisioningv1api.Cluster) (string, error) {
	err := wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return c.Status.ClusterName != "", nil
	})
	if err != nil {
		return "", err
	}

	for i := 0; i < 15; i++ {
		tokens, err := clients.Mgmt.ClusterRegistrationToken().List(c.Status.ClusterName, metav1.ListOptions{})
		if err != nil || len(tokens.Items) == 0 || tokens.Items[0].Status.NodeCommand == "" {
			time.Sleep(time.Second)
			continue
		}
		return strings.Replace(tokens.Items[0].Status.InsecureNodeCommand, "curl", "curl --retry-connrefused --retry-delay 5 --retry 30", -1), nil
	}

	return "", fmt.Errorf("timeout getting custom command")
}
