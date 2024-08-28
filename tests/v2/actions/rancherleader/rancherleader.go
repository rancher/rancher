package rancherleader

import (
	"net/url"

	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
)

const (
	KubeSystemNamespace = "kube-system"
	LeaseName           = "cattle-controllers"
	LeaseSteveType      = "coordination.k8s.io.lease"
)

// GetRancherLeaderPodName is a helper function to retrieve the name of the rancher leader pod
func GetRancherLeaderPodName(client *rancher.Client) (string, error) {
	query := url.Values{"fieldSelector": {"metadata.name=" + LeaseName}}
	lease, err := client.Steve.SteveType(LeaseSteveType).NamespacedSteveClient(KubeSystemNamespace).List(query)
	if err != nil {
		return "", err
	}

	leaseSpec := &coordinationv1.LeaseSpec{}
	err = v1.ConvertToK8sType(lease.Data[0].Spec, leaseSpec)
	if err != nil {
		return "", err
	}

	leaderPodName := *leaseSpec.HolderIdentity

	return leaderPodName, nil
}
