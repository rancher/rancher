package cluster

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/nodeconfig"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/rancher/tests/v2prov/wait"
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

const ConflictMessageRegex = `\[K8s\] encountered an error while attempting to update the secret: Operation cannot be fulfilled on secrets.*: the object has been modified; please apply your changes to the latest version and try again`
const SaneConflictMessageThreshold = 3

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

		for i, mp := range cluster.Spec.RKEConfig.MachinePools {
			if mp.NodeConfig == nil {
				podConfig, err := nodeconfig.NewPodConfig(clients, cluster.Namespace)
				if err != nil {
					return nil, err
				}
				cluster.Spec.RKEConfig.MachinePools[i].NodeConfig = podConfig
			}
			if mp.Name == "" {
				cluster.Spec.RKEConfig.MachinePools[i].Name = fmt.Sprintf("pool-%d", i)
			}
		}

		if cluster.Spec.RKEConfig.Registries == nil {
			registryConfig, err := registry.CreateOrGetRegistry(clients, cluster.Namespace, "registry-cache", false)
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
			data, newErr := gatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			err = fmt.Errorf("creation wait failed on: %w\n%s", err, data)
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

func WaitForControlPlane(clients *clients.Clients, c *provisioningv1api.Cluster, errorPrefix string, rkeControlPlaneCheckFunc func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error)) (_ *rkev1.RKEControlPlane, err error) {
	defer func() {
		if err != nil {
			data, newErr := gatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			err = fmt.Errorf("%s wait failed on: %w\n%s", errorPrefix, err, data)
		}
	}()

	controlPlane, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("%s wait did not succeed : %w", errorPrefix, err)
	}

	err = wait.Object(clients.Ctx, clients.RKE.RKEControlPlane().Watch, controlPlane, func(obj runtime.Object) (bool, error) {
		controlPlane = obj.(*rkev1.RKEControlPlane)
		return rkeControlPlaneCheckFunc(controlPlane)
	})
	if err != nil {
		return nil, fmt.Errorf("%s wait did not succeed : %w", errorPrefix, err)
	}

	return controlPlane, nil
}

func WaitForDelete(clients *clients.Clients, c *provisioningv1api.Cluster) (_ *provisioningv1api.Cluster, err error) {
	defer func() {
		if err != nil {
			data, newErr := gatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			err = fmt.Errorf("deletion wait failed on: %w\n%s", err, data)
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

// getPodLogs gathers the logs from the specified pod in a manner similar to `kubectl logs`
func getPodLogs(clients *clients.Clients, podNamespace, podName string) (string, error) {
	plr := clients.K8s.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := plr.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error streaming pod logs for pod %s/%s: %v", podNamespace, podName, err)
	}
	defer stream.Close()

	reader := bufio.NewScanner(stream)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%sSnewlineG", reader.Text())
	}
	return capr.CompressInterface(logs)
}

// countPodLogRegexOccurances gathers the logs from the specified pod in a manner similar to `kubectl logs` and counts the number of times the log matches the given regex.
func countPodLogRegexOccurances(clients *clients.Clients, podNamespace, podName, regex string) (int, error) {
	count := 0
	plr := clients.K8s.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := plr.Stream(context.TODO())
	if err != nil {
		return count, fmt.Errorf("error streaming pod logs for pod %s/%s: %v", podNamespace, podName, err)
	}
	defer stream.Close()

	re := regexp.MustCompile(regex)

	reader := bufio.NewScanner(stream)
	for reader.Scan() {
		if re.Match(reader.Bytes()) {
			count++
		}
	}
	return count, nil
}

// getPodFileContents executes a corresponding `kubectl cp` and gathers data for the purposes of helping increasing debug data.
func getPodFileContents(podNamespace, podName, podPath string) (string, error) {
	destFile := fmt.Sprintf("/tmp/%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s/%s/%s", podNamespace, podName, podPath))))
	kcp := []string{
		"-n",
		podNamespace,
		"cp",
		fmt.Sprintf("%s:%s", podName, podPath),
		destFile,
	}

	cmd := exec.Command("kubectl", kcp...)
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error running kubectl -n %s cp %s:%s %s", podNamespace, podName, podPath, destFile)
		return "", nil
	}

	file, err := os.Open(destFile)
	if err != nil {
		logrus.Errorf("error retrieving destination file %s: %v", destFile, err)
		return "", nil
	}
	defer file.Close()
	reader := bufio.NewScanner(file)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%sSnewlineG", reader.Text())
	}
	return capr.CompressInterface(logs)
}

// gatherDebugData gathers debug data that is relevant to the current cluster and returns a gzip compressed + base64 encoded string of the json.
func gatherDebugData(clients *clients.Clients, c *provisioningv1api.Cluster) (string, error) {
	newC, newErr := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if newErr != nil {
		logrus.Errorf("failed to get cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
		newC = nil
	}

	newControlPlane, newErr := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if newErr != nil {
		logrus.Errorf("failed to get controlplane %s/%s to print error: %v", c.Namespace, c.Name, newErr)
		newControlPlane = nil
	}

	runtime := capr.GetRuntime(newControlPlane.Spec.KubernetesVersion)

	var rkeBootstraps []*rkev1.RKEBootstrap
	var infraMachines []*unstructured.Unstructured

	var podLogs = make(map[string]map[string]string)

	machines, newErr := Machines(clients, c)
	if newErr != nil {
		logrus.Errorf("failed to get machines for %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	} else {
		for _, machine := range machines.Items {
			rb, newErr := clients.RKE.RKEBootstrap().Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Errorf("failed to get RKEBootstrap %s/%s to print error: %v", c.Namespace, c.Name, newErr)
			} else {
				rkeBootstraps = append(rkeBootstraps, rb)
			}
			im, newErr := clients.Dynamic.Resource(schema.GroupVersionResource{
				Group:    machine.Spec.InfrastructureRef.GroupVersionKind().Group,
				Version:  machine.Spec.InfrastructureRef.GroupVersionKind().Version,
				Resource: strings.ToLower(fmt.Sprintf("%ss", machine.Spec.InfrastructureRef.GroupVersionKind().Kind)),
			}).Namespace(machine.Spec.InfrastructureRef.Namespace).Get(context.TODO(), machine.Spec.InfrastructureRef.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Errorf("failed to get %s %s/%s to print error: %v", machine.Spec.InfrastructureRef.GroupVersionKind().String(), machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name, newErr)
			} else {
				infraMachines = append(infraMachines, im)
				if machine.Spec.InfrastructureRef.GroupVersionKind().Kind == "PodMachine" {
					// In the case of a podmachine, the pod name will be strings.ReplaceAll(infra.meta.GetName(), ".", "-")
					podName := strings.ReplaceAll(im.GetName(), ".", "-")
					podLogs[podName] = populatePodLogs(clients, runtime, im.GetNamespace(), podName)
				}
			}
		}
	}

	customPods, newErr := clients.Core.Pod().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "custom-cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to list custommachine pods: %v", newErr)
	} else {
		for _, pod := range customPods.Items {
			podLogs[pod.Name] = populatePodLogs(clients, runtime, pod.Namespace, pod.Name)
		}
	}

	rkeBootstrapTemplates, newErr := clients.RKE.RKEBootstrapTemplate().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to get rkebootstrap templates for %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	}

	mgmtCluster, newErr := clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	if apierrors.IsNotFound(newErr) {
		mgmtCluster = nil
	} else if newErr != nil {
		logrus.Errorf("failed to get mgmt cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	}

	var infraCluster *unstructured.Unstructured
	var machineDeployments *capi.MachineDeploymentList
	var machineSets *capi.MachineSetList

	capiCluster, newErr := clients.CAPI.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(newErr) {
		capiCluster = nil
	} else if newErr != nil {
		logrus.Errorf("failed to get capi cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	} else {
		infraCluster, newErr = clients.Dynamic.Resource(schema.GroupVersionResource{
			Group:    capiCluster.Spec.InfrastructureRef.GroupVersionKind().Group,
			Version:  capiCluster.Spec.InfrastructureRef.GroupVersionKind().Version,
			Resource: strings.ToLower(fmt.Sprintf("%ss", capiCluster.Spec.InfrastructureRef.GroupVersionKind().Kind)),
		}).Namespace(capiCluster.Spec.InfrastructureRef.Namespace).Get(context.TODO(), capiCluster.Spec.InfrastructureRef.Name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Errorf("failed to get %s %s/%s to print error: %v", capiCluster.Spec.InfrastructureRef.GroupVersionKind().String(), capiCluster.Spec.InfrastructureRef.Namespace, capiCluster.Spec.InfrastructureRef.Name, newErr)
			infraCluster = nil
		}
		machineDeployments, newErr = clients.CAPI.MachineDeployment().List(c.Namespace, metav1.ListOptions{
			LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
		})
		if newErr != nil {
			logrus.Error(newErr)
			machineDeployments = nil
		}
		machineSets, newErr = clients.CAPI.MachineSet().List(c.Namespace, metav1.ListOptions{
			LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
		})
		if newErr != nil {
			logrus.Error(newErr)
			machineSets = nil
		}
	}

	snapshots, newErr := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{})
	if newErr != nil {
		logrus.Error(newErr)
		snapshots = nil
	}

	return capr.CompressInterface(map[string]interface{}{
		"cluster":               newC,
		"rkecontrolplane":       newControlPlane,
		"mgmtCluster":           mgmtCluster,
		"capiCluster":           capiCluster,
		"machineDeployments":    machineDeployments,
		"machineSets":           machineSets,
		"machines":              machines,
		"rkeBootstraps":         rkeBootstraps,
		"rkeBootstrapTemplates": rkeBootstrapTemplates,
		"infraCluster":          infraCluster,
		"infraMachines":         infraMachines,
		"podLogs":               podLogs,
		"snapshots":             snapshots,
	})
}

// populatePodLogs creates a map[string]string of logs that correspond to the pod in question. If the pod is an RKE2 pod, it also collects the kubelet logs from the pod filesystem.
func populatePodLogs(clients *clients.Clients, runtime, podNamespace, podName string) map[string]string {
	var logMap = make(map[string]string)

	logs, newErr := getPodLogs(clients, podNamespace, podName)
	if newErr != nil {
		logrus.Errorf("error while retrieving pod logs: %v", newErr)
	} else {
		logMap["logs"] = logs
	}

	if runtime == capr.RuntimeRKE2 {
		kubeletLogs, newErr := getPodFileContents(podNamespace, podName, fmt.Sprintf("/var/lib/rancher/rke2/agent/logs/kubelet.log"))
		if newErr != nil {
			logrus.Errorf("error while retrieving pod kubelet logs: %v", newErr)
		} else {
			logMap["kubeletLogs"] = kubeletLogs
		}
	}

	return logMap
}

// EnsureMinimalConflictsWithThreshold walks through pod logs and ensures a minimal number of conflict messages have occurred.
func EnsureMinimalConflictsWithThreshold(clients *clients.Clients, c *provisioningv1api.Cluster, threshold int) error {
	customPods, newErr := clients.Core.Pod().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "custom-cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to list custommachine pods: %v", newErr)
	} else {
		for _, pod := range customPods.Items {
			count, err := countPodLogRegexOccurances(clients, pod.Namespace, pod.Name, ConflictMessageRegex)
			if err != nil {
				return err
			}
			if count > threshold {
				return fmt.Errorf("pod %s/%s had %d occurances of conflicts which was greater than the threshold of %d", pod.Namespace, pod.Name, count, threshold)
			}
		}
	}

	machines, newErr := Machines(clients, c)
	if newErr != nil {
		logrus.Errorf("failed to get machines for %s/%s to count conflicts: %v", c.Namespace, c.Name, newErr)
	}
	for _, machine := range machines.Items {
		im, newErr := clients.Dynamic.Resource(schema.GroupVersionResource{
			Group:    machine.Spec.InfrastructureRef.GroupVersionKind().Group,
			Version:  machine.Spec.InfrastructureRef.GroupVersionKind().Version,
			Resource: strings.ToLower(fmt.Sprintf("%ss", machine.Spec.InfrastructureRef.GroupVersionKind().Kind)),
		}).Namespace(machine.Spec.InfrastructureRef.Namespace).Get(context.TODO(), machine.Spec.InfrastructureRef.Name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Errorf("failed to get %s %s/%s to print error: %v", machine.Spec.InfrastructureRef.GroupVersionKind().String(), machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name, newErr)
		} else {
			if machine.Spec.InfrastructureRef.GroupVersionKind().Kind == "PodMachine" {
				// In the case of a podmachine, the pod name will be strings.ReplaceAll(infra.meta.GetName(), ".", "-")
				podName := strings.ReplaceAll(im.GetName(), ".", "-")
				count, err := countPodLogRegexOccurances(clients, im.GetNamespace(), podName, ConflictMessageRegex)
				if err != nil {
					return err
				}
				if count > threshold {
					return fmt.Errorf("pod %s/%s had %d occurances of conflicts which was greater than the threshold of %d", im.GetNamespace(), podName, count, threshold)
				}
			}
		}
	}
	return nil
}
