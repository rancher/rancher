package systeminfo

import (
	"fmt"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	fleetv1alpha1 "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type Retriever struct {
	fleetClusterCache fleetv1alpha1.ClusterCache
}

// NewRetriever returns a new instance of the systeminfo.Retriever
func NewRetriever(clients *wrangler.Context) *Retriever {
	if clients == nil {
		return nil
	}
	return &Retriever{
		fleetClusterCache: clients.Fleet.Cluster().Cache(),
	}
}

// GetSystemPodLabelSelectors returns a slice of strings that contains system pod label selectors in the format of namespace:labelSelector, delimited by :
func (r *Retriever) GetSystemPodLabelSelectors(controlPlane *v1.RKEControlPlane) []string {
	if controlPlane == nil {
		return []string{}
	}
	labelSelectors := []string{
		"cattle-system:app=cattle-cluster-agent",                               // See pkg/systemtemplate/template.go -- notably hard coded namespace as the systemtemplate is hard coded (as of 05/11/2023)
		"cattle-system:app=kube-api-auth",                                      // See pkg/systemtemplate/template.go -- notably hard coded namespace as the systemtemplate is hard coded @ Line 402 & 407 (as of 05/11/2023)
		fmt.Sprintf("%s:app=rancher-webhook", namespace.System),                // See pkg/controllers/dashboard/systemcharts/controller.go Line 115=
		"cattle-system:upgrade.cattle.io/controller=system-upgrade-controller", // See: https://github.com/rancher/charts/blob/872076dc31642eaa9d2a781a08069d2fc2436f9b/charts/system-upgrade-controller/102.0.0%2Bup0.4.0/templates/deployment.yaml#L5
	}

	fc, err := r.fleetClusterCache.Get(controlPlane.Namespace, controlPlane.Name)
	if err != nil {
		logrus.Errorf("error retrieving fleet cluster %s/%s: %v", controlPlane.Namespace, controlPlane.Name, err)
		// Don't return here so we don't erroneously block pod cleanup
	} else {
		cfsNamespace := "cattle-fleet-system"
		if fc.Spec.AgentNamespace != "" {
			cfsNamespace = fc.Spec.AgentNamespace
		}
		labelSelectors = append(labelSelectors, fmt.Sprintf("%s:app=fleet-agent", cfsNamespace))
	}
	return labelSelectors
}
