package globaldns

import (
	"fmt"
	"strings"

	v1coreRancher "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserGlobalDNSController struct {
	ingressLister         ingresswrapper.CompatLister
	globalDNSs            v3.GlobalDnsInterface
	multiclusterappLister v3.MultiClusterAppLister
	namespaceLister       v1coreRancher.NamespaceLister
	clusterName           string
}

func newUserGlobalDNSController(clusterContext *config.UserContext) *UserGlobalDNSController {
	g := UserGlobalDNSController{
		ingressLister:         ingresswrapper.NewCompatLister(clusterContext.Networking, clusterContext.Extensions, clusterContext.K8sClient),
		globalDNSs:            clusterContext.Management.Management.GlobalDnses(""),
		multiclusterappLister: clusterContext.Management.Management.MultiClusterApps("").Controller().Lister(),
		namespaceLister:       clusterContext.Core.Namespaces("").Controller().Lister(),
		clusterName:           clusterContext.ClusterName,
	}
	return &g
}

func (g *UserGlobalDNSController) sync(key string, obj *v3.GlobalDns) (runtime.Object, error) {

	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	var targetEndpoints []string
	var err error

	if obj.Spec.MultiClusterAppName != "" {
		targetEndpoints, err = g.reconcileMultiClusterApp(obj)
	} else if len(obj.Spec.ProjectNames) > 0 {
		targetEndpoints, err = g.reconcileProjects(obj)
	}

	if err != nil {
		return nil, err
	}

	//compare with the clusterEndpoints and find endpoints to update and remove.
	return g.refreshGlobalDNSEndpoints(obj, targetEndpoints)
}

func (g *UserGlobalDNSController) reconcileMultiClusterApp(obj *v3.GlobalDns) ([]string, error) {
	// If multiclusterappID is set, look for ingresses in the projects of multiclusterapp's targets
	// Get multiclusterapp by name set on GlobalDNS spec
	mcappName, err := getMultiClusterAppName(obj.Spec.MultiClusterAppName)
	if err != nil {
		return nil, err
	}

	mcapp, err := g.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
	if err != nil && k8serrors.IsNotFound(err) {
		logrus.Debugf("UserGlobalDNSController: Object Not found Error %v, while listing MulticlusterApp by name %v", err, mcappName)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("UserGlobalDNSController: Error %v Listing MulticlusterApp by name %v", err, mcappName)
	}

	// go through target projects which are part of the current cluster and find all ingresses
	var allIngresses []*ingresswrapper.CompatIngress

	for _, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		// check if the target project in this iteration is same as the cluster in current context
		if split[0] != g.clusterName {
			continue
		}

		// each target will have appName, this appName is also the namespace in which all workloads for this app are created
		var err error
		allIngresses, err = g.ingressLister.List(t.AppName, labels.NewSelector())
		if err != nil {
			return nil, err
		}
	}

	//gather endpoints
	return g.fetchGlobalDNSEndpointsForIngresses(allIngresses, obj)
}

func (g *UserGlobalDNSController) reconcileProjects(obj *v3.GlobalDns) ([]string, error) {
	// go through target projects which are part of the current cluster and find all ingresses
	var allIngresses []*ingresswrapper.CompatIngress

	allNamespaces, err := g.namespaceLister.List("", labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("UserGlobalDNSController: Error listing cluster namespaces %v", err)
	}

	for _, projectNameSet := range obj.Spec.ProjectNames {
		split := strings.SplitN(projectNameSet, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("UserGlobalDNSController: Error in splitting project Name %v", projectNameSet)
		}
		// check if the project in this iteration belongs to the same cluster in current context
		if split[0] != g.clusterName {
			continue
		}
		projectID := split[1]
		//list all namespaces in this project and list all ingresses within each namespace
		var namespacesInProject []string
		for _, namespace := range allNamespaces {
			nameSpaceProject := namespace.ObjectMeta.Labels[projectSelectorLabel]
			if strings.EqualFold(projectID, nameSpaceProject) {
				namespacesInProject = append(namespacesInProject, namespace.Name)
			}
		}
		for _, namespace := range namespacesInProject {
			var err error
			allIngresses, err = g.ingressLister.List(namespace, labels.NewSelector())
			if err != nil {
				return nil, err
			}
		}
	}
	//gather endpoints
	return g.fetchGlobalDNSEndpointsForIngresses(allIngresses, obj)
}

func (g *UserGlobalDNSController) fetchGlobalDNSEndpointsForIngresses(ingresses []*ingresswrapper.CompatIngress, obj *v3.GlobalDns) ([]string, error) {
	if len(ingresses) == 0 {
		return nil, nil
	}

	var allEndpoints []string
	//gather endpoints from all ingresses
	for _, ing := range ingresses {
		if gdns, ok := ing.GetAnnotations()[annotationGlobalDNS]; ok {
			// check if the globalDNS in annotation is same as the FQDN set on the GlobalDNS
			if gdns != obj.Spec.FQDN {
				continue
			}
			//gather endpoints from the ingress
			ingressEndpoints := gatherIngressEndpoints(ing.Status.LoadBalancer.Ingress)
			allEndpoints = append(allEndpoints, ingressEndpoints...)
		}
	}
	return allEndpoints, nil
}

func (g *UserGlobalDNSController) refreshGlobalDNSEndpoints(globalDNS *v3.GlobalDns, ingressEndpointsForCluster []string) (*v3.GlobalDns, error) {

	globalDNSToUpdate := globalDNS.DeepCopy()
	uniqueEndpointsForCluster := dedupEndpoints(ingressEndpointsForCluster)

	if len(globalDNSToUpdate.Status.ClusterEndpoints) == 0 {
		globalDNSToUpdate.Status.ClusterEndpoints = make(map[string][]string)
	}

	clusterEps := globalDNSToUpdate.Status.ClusterEndpoints[g.clusterName]
	if ifEndpointsDiffer(clusterEps, uniqueEndpointsForCluster) {
		globalDNSToUpdate.Status.ClusterEndpoints[g.clusterName] = uniqueEndpointsForCluster
		reconcileGlobalDNSEndpoints(globalDNSToUpdate)

		updated, err := g.globalDNSs.Update(globalDNSToUpdate)
		if err != nil {
			return updated, fmt.Errorf("UserGlobalDNSController: Failed to update GlobalDNS endpoints with error %v", err)
		}
		return updated, nil
	}
	return nil, nil
}
