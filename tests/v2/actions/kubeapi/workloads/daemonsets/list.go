package daemonsets

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DaemonSetList is a struct that contains a list of daemonsets.
type DaemonSetList struct {
	Items []appv1.DaemonSet
}

// ListDaemonsets is a helper function that uses the dynamic client to list daemonsets in a cluster with its list options.
func ListDaemonsets(client *rancher.Client, clusterID, namespace string, listOpts metav1.ListOptions) (*DaemonSetList, error) {
	daemonsetList := new(DaemonSetList)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	daemonsetResource := dynamicClient.Resource(DaemonSetGroupVersionResource).Namespace(namespace)
	daemonsets, err := daemonsetResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredDaemonset := range daemonsets.Items {
		newDaemonset := &appv1.DaemonSet{}

		err := scheme.Scheme.Convert(&unstructuredDaemonset, newDaemonset, unstructuredDaemonset.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		daemonsetList.Items = append(daemonsetList.Items, *newDaemonset)
	}

	return daemonsetList, nil
}

// Names is a method that accepts DaemonSetList as a receiver,
// returns each daemonset name in the list as a new slice of strings.
func (list *DaemonSetList) Names() []string {
	var daemonsetNames []string

	for _, daemonset := range list.Items {
		daemonsetNames = append(daemonsetNames, daemonset.Name)
	}

	return daemonsetNames
}
