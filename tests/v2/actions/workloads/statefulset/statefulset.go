package statefulset

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/shepherd/clients/rancher"
	unstruc "github.com/rancher/shepherd/extensions/unstructured"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	nginxImageName = "public.ecr.aws/docker/library/nginx"
)

var StatefulsetGroupVersionResource = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "statefulsets",
}

// CreateStatefulset is a helper to create a statefulset
func CreateStatefulset(client *rancher.Client, clusterID, namespaceName string, template corev1.PodTemplateSpec, replicas int32) (*appv1.StatefulSet, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	statefulsetName := namegen.AppendRandomString("teststatefulset")
	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.statefulset-%v-%v", namespaceName, statefulsetName)

	template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	statefulsetTemplate := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulsetName,
			Namespace: namespaceName,
		},
		Spec: appv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: template,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}

	statefulsetResource := dynamicClient.Resource(StatefulsetGroupVersionResource).Namespace(namespaceName)

	unstructuredResp, err := statefulsetResource.Create(context.TODO(), unstruc.MustToUnstructured(statefulsetTemplate), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newStatefulset := &appv1.StatefulSet{}
	err = scheme.Scheme.Convert(unstructuredResp, &appv1.StatefulSet{}, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newStatefulset, err
}
