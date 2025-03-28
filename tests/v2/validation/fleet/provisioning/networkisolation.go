package provisioning

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"

	extensionpods "github.com/rancher/shepherd/extensions/workloads/pods"
)

const defaultNamespace = "default"

// testPNI is a set of in-order instructions that gets the IP from a pod in an existing deployment/project/namespace,
// then creates a new deployment in the default project/namespace which tries to ping the existing pod.
// This errors if the new pod can ping the existing one, meaning that PNI is not working.
// Not designed to work with a deployment already in the default namespace!
func testPNI(client *rancher.Client, clusterName, deploymentNamespace, deploymentName string) error {
	// get one of the fleet deployment's pod IPs to use in the ping command later
	fleetPodNames, err := pods.GetPodNamesFromDeployment(client, clusterName, deploymentNamespace, deploymentName)
	if err != nil {
		return err
	}
	if len(fleetPodNames) <= 0 {
		return errors.New("unable to find pod(s) in deployment")
	}

	firstPodObject, err := pods.GetPodByName(client, clusterName, deploymentNamespace, fleetPodNames[0])
	if err != nil {
		return err
	}

	corePod := &corev1.Pod{}
	err = steveV1.ConvertToK8sType(firstPodObject, corePod)
	if err != nil {
		return err
	}

	// create a new workload / pod to ping that's hardened-compliant
	allowPrivilegeEscalation := false
	runAsNonRoot := true
	userID := int64(1000)
	container := corev1.Container{
		Name:            "test-pni" + namegenerator.RandStringLower(3),
		Image:           "redis:7-alpine",
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts:    nil,
		EnvFrom:         []corev1.EnvFromSource{},
		Command:         []string{"ping", "-c", "1", corePod.Status.PodIP},
		Args:            nil,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			RunAsNonRoot:             &runAsNonRoot,
			RunAsUser:                &userID,
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					corev1.Capability("ALL"),
				},
			},
		},
	}
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{container},
			RestartPolicy:    corev1.RestartPolicyAlways,
			Volumes:          nil,
			ImagePullSecrets: []corev1.LocalObjectReference{},
			NodeSelector:     nil,
		},
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterName, "test-d"+namegenerator.RandStringLower(3), defaultNamespace, podTemplate, 1) // jobs.CreateJob(tt.client, clusterName, "test-pni-d", defaultNamespace, podTemplate)
	if err != nil {
		return err
	}
	// Begin Validation

	err = deployments.WatchAndWaitDeployments(client, clusterName, defaultNamespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDeployment.Name,
	})
	if err != nil {
		return err
	}

	errList := extensionpods.StatusPods(client, clusterName)
	if len(errList) > 0 {
		return errors.New("error in pod(s) after created hardened deployment")
	}

	jobPodName, err := pods.GetPodNamesFromDeployment(client, clusterName, defaultNamespace, createdDeployment.Name)
	if err != nil {
		return err
	}

	jobPodObject, err := pods.GetPodByName(client, clusterName, defaultNamespace, jobPodName[0])
	if err != nil {
		return err
	}

	downstreamClient, err := client.Steve.ProxyDownstream(clusterName)
	if err != nil {
		return err
	}

	var podLogs string

	// pod will come to an active state breifly when healthy. i.e. when sending the ping, before receiving the failure
	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.OneMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		podObject, err := downstreamClient.SteveType(extensionpods.PodResourceSteveType).ByID(defaultNamespace + "/" + jobPodObject.Name)
		if err != nil {
			return false, err
		}

		return extensionpods.IsPodReady(podObject)
	})
	if err != nil {
		return err
	}

	// wait for pod to error out from ping. sometimes the correct error logs don't show up on time, so we have to wait for the pod to officially fail
	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.OneMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		podObject, err := downstreamClient.SteveType(extensionpods.PodResourceSteveType).ByID(defaultNamespace + "/" + jobPodObject.Name)
		if err != nil {
			return false, err
		}

		done, err = areContainersReady(podObject)
		if err != nil {
			return
		}
		return false, nil

	})
	if err == nil {
		return errors.New("pod did not error out, PNI is not working correctly")
	}

	errorRgx := regexp.MustCompile(regexp.QuoteMeta("received, 100"))
	podLogs, _ = kubeconfig.GetPodLogs(client, clusterName, jobPodObject.Name, defaultNamespace, "")

	if matches := errorRgx.FindStringSubmatch(podLogs); len(matches) > 0 {
		return nil
	}

	return errors.New("pod logs appear to be able to ping another project" + podLogs)

}

func areContainersReady(pod *steveV1.SteveAPIObject) (bool, error) {
	podStatus := &corev1.PodStatus{}
	err := steveV1.ConvertToK8sType(pod.Status, podStatus)
	if err != nil {
		return false, err
	}

	if podStatus.ContainerStatuses == nil || len(podStatus.ContainerStatuses) == 0 {
		return false, nil
	}

	phase := podStatus.Phase

	if phase == corev1.PodPending {
		return false, nil
	}
	var errorMessage string
	for _, containerStatus := range podStatus.ContainerStatuses {
		// Rancher deploys multiple hlem-operation jobs to do the same task. If one job succeeds, the others end in a terminated status.
		if containerStatus.State.Terminated != nil {
			errorMessage += fmt.Sprintf("Container Terminated:\n%s: %s: pod is %s\n", containerStatus.State.Terminated.Reason, pod.Name, phase)
		}
	}

	if errorMessage != "" {
		return true, errors.New(errorMessage)
	}

	return true, nil
}

// updateNamespaceWithNewProject is a helper which moves a fleet repo's deployment in a downstream cluster to a project
func updateNamespaceWithNewProject(client *rancher.Client, clusterName string, repoStatus *v1alpha1.GitRepoStatus) error {

	hardenedNS, err := namespaces.GetNamespaceByName(client, clusterName, repoStatus.Resources[0].Namespace)
	if err != nil {
		return err
	}

	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return err
	}

	// create a project, then add the hardened (fleet added) namespace to the new project
	namespaceResource := dynamicClient.Resource(namespaces.NamespaceGroupVersionResource).Namespace("")

	createdProject, err := client.Management.Project.Create(projects.NewProjectConfig(clusterName))
	if err != nil {
		return err
	}
	hardenedProjectName := strings.Split(createdProject.ID, ":")[1]

	hardenedNS.Annotations["field.cattle.io/projectId"] = createdProject.ID

	_, err = namespaceResource.Update(context.TODO(), unstructured.MustToUnstructured(hardenedNS), metav1.UpdateOptions{}, "")
	if err != nil {
		return err
	}

	return projects.WaitForProjectIDUpdate(client, clusterName, hardenedProjectName, hardenedNS.Name)
}
