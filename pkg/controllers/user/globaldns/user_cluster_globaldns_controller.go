package globaldns

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	v1beta1Rancher "github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserGlobalDNSController struct {
	ingresses             v1beta1Rancher.IngressInterface
	ingressLister         v1beta1Rancher.IngressLister
	globalDNSs            v3.GlobalDNSInterface
	globalDNSLister       v3.GlobalDNSLister
	multiclusterappLister v3.MultiClusterAppLister
	clusterName           string
}

func newUserGlobalDNSController(clusterContext *config.UserContext) *UserGlobalDNSController {
	g := UserGlobalDNSController{
		ingresses:             clusterContext.Extensions.Ingresses(""),
		ingressLister:         clusterContext.Extensions.Ingresses("").Controller().Lister(),
		globalDNSs:            clusterContext.Management.Management.GlobalDNSs(""),
		globalDNSLister:       clusterContext.Management.Management.GlobalDNSs("").Controller().Lister(),
		multiclusterappLister: clusterContext.Management.Management.MultiClusterApps("").Controller().Lister(),
		clusterName:           clusterContext.ClusterName,
	}
	return &g
}

func (g *UserGlobalDNSController) sync(key string, obj *v3.GlobalDNS) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	// This controller only deals with GlobalDNS created with MultiClusterAppID set
	if obj.Spec.MultiClusterAppName == "" {
		return nil, nil
	}

	// If multiclusterappID is set, look for ingresses in the projects of multiclusterapp's targets
	// Get multiclusterapp by name set on GlobalDNS spec
	mcappName, err := getMultiClusterAppName(obj.Spec.MultiClusterAppName)
	if err != nil {
		return nil, err
	}
	mcapp, err := g.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
	if err != nil {
		return nil, err
	}

	// go through target projects which are part of the current cluster
	for _, t := range mcapp.Spec.Targets {
		split := strings.SplitN(t.ProjectName, ":", 2)
		if len(split) != 2 {
			return mcapp, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
		}
		// check if the target project in this iteration is same as the cluster in current context
		if split[0] != g.clusterName {
			logrus.Debugf("Continuing since target is not for current cluster since project %v doesn't belong in cluster %v", split[1], g.clusterName)
			continue
		}

		updated, err := g.updateGlobalDNSEndpointsForTarget(&t, obj)
		if err != nil {
			return updated, err
		}
	}
	return nil, err
}

func (g *UserGlobalDNSController) updateGlobalDNSEndpointsForTarget(t *v3.Target, obj *v3.GlobalDNS) (*v3.GlobalDNS, error) {
	// each target will have appName, this appName is also the namespace in which all workloads for this app are created
	ingresses, err := g.ingressLister.List(t.AppName, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	if len(ingresses) == 0 {
		return nil, nil
	}

	for _, ing := range ingresses {
		if gdns, ok := ing.Annotations[annotationGlobalDNS]; ok {
			// check if the globalDNS in annotation is same as the FQDN set on the GlobalDNS
			if gdns != obj.Spec.FQDN {
				continue
			}
			//update endpoints on GlobalDNS status field
			ingressEndpoints := gatherIngressEndpoints(ing.Status.LoadBalancer.Ingress)
			updatedGDNS, err := g.updateGlobalDNSEndpoints(obj, ingressEndpoints)
			if err != nil {
				return updatedGDNS, err
			}
		}
	}
	return nil, nil
}

func (g *UserGlobalDNSController) updateGlobalDNSEndpoints(globalDNS *v3.GlobalDNS, ingressEndpoints []string) (*v3.GlobalDNS, error) {
	globalDNS = prepareGlobalDNSForUpdate(globalDNS, ingressEndpoints)
	updated, err := g.globalDNSs.Update(globalDNS)
	if err != nil {
		return updated, fmt.Errorf("UserGlobalDNSController: Failed to update GlobalDNS endpoints with error %v", err)
	}
	return nil, nil
}

func getMultiClusterAppName(multiClusterAppName string) (string, error) {
	split := strings.SplitN(multiClusterAppName, ":", 2)
	if len(split) != 2 {
		return "", fmt.Errorf("error in splitting multiclusterapp ID %v", multiClusterAppName)
	}
	mcappName := split[1]
	return mcappName, nil
}
