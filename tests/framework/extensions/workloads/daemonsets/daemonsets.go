package daemonsets

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateDaemonset is a helper function that uses the v1 steve client to create a daemonset on a namespace for a specific cluster.
func CreateDaemonset(client *rancher.Client, clusterID, daemonsetName, namespace string, template corev1.PodTemplateSpec) (*v1.SteveAPIObject, error) {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.daemonset-%v-%v", namespace, daemonsetName)

	template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	template.Spec.RestartPolicy = corev1.RestartPolicyAlways
	daemonset := &appv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonsetName,
			Namespace: namespace,
		},
		Spec: appv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: template,
		},
	}

	daemonsetResp, err := steveclient.SteveType(workloads.DaemonsetSteveType).Create(daemonset)
	if err != nil {
		return nil, err
	}

	return daemonsetResp, nil
}
