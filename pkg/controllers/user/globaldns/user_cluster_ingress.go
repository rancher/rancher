package globaldns

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	v1Rancher "github.com/rancher/types/apis/core/v1"
	v1beta1Rancher "github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	annotationGlobalDNS       = "rancher.io/globalDNS.hostname"
	appSelectorLabel          = "io.cattle.field/appId"
	projectSelectorLabel      = "field.cattle.io/projectId"
	UserIngressControllerName = "globaldns-useringress-controller"
)

type UserIngressController struct {
	ingresses             v1beta1Rancher.IngressInterface
	ingressLister         v1beta1Rancher.IngressLister
	globalDNSs            v3.GlobalDNSInterface
	globalDNSLister       v3.GlobalDNSLister
	appLister             projectv3.AppLister
	namespaceLister       v1Rancher.NamespaceLister
	multiclusterappLister v3.MultiClusterAppLister
}

func newUserIngressController(ctx context.Context, clusterContext *config.UserContext) *UserIngressController {
	n := &UserIngressController{
		ingresses:             clusterContext.Extensions.Ingresses(""),
		ingressLister:         clusterContext.Extensions.Ingresses("").Controller().Lister(),
		globalDNSs:            clusterContext.Management.Management.GlobalDNSs(""),
		globalDNSLister:       clusterContext.Management.Management.GlobalDNSs("").Controller().Lister(),
		appLister:             clusterContext.Management.Project.Apps("").Controller().Lister(),
		namespaceLister:       clusterContext.Core.Namespaces("").Controller().Lister(),
		multiclusterappLister: clusterContext.Management.Management.MultiClusterApps("").Controller().Lister(),
	}
	return n
}

func Register(ctx context.Context, clusterContext *config.UserContext) {
	n := newUserIngressController(ctx, clusterContext)
	clusterContext.Extensions.Ingresses("").AddHandler(ctx, UserIngressControllerName, n.sync)
}

func (ic *UserIngressController) sync(key string, obj *v1beta1.Ingress) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}
	//if there are no globaldns cr, skip this run

	if ic.noGlobalDNS() {
		logrus.Debug("UserIngressController: Skipping run, no Global DNS registered")
		return nil, nil
	}

	annotations := obj.Annotations
	logrus.Debugf("Ingress annotations %v", annotations)

	//look for globalDNS annotation, if found load the GlobalDNS if there are Ingress endpoints
	if annotations[annotationGlobalDNS] != "" && len(obj.Status.LoadBalancer.Ingress) > 0 {
		fqdnRequested := annotations[annotationGlobalDNS]
		//check if the corresponding GlobalDNS CR is present
		globalDNS, err := ic.findGlobalDNS(fqdnRequested)

		if globalDNS == nil || err != nil {
			return nil, fmt.Errorf("UserIngressController: Cannot find GlobalDNS resource for FQDN requested %v", fqdnRequested)
		}

		//check if 'multiclusterappID' on GlobalDNS CR matches the annotation on ingress OR
		if err = ic.checkForMultiClusterApp(obj, globalDNS); err != nil {
			return nil, err
		}

		//check if 'projectNames' on GlobalDNS CR matches to the user's project for multiclusterapp
		if err = ic.checkForProjects(obj, globalDNS); err != nil {
			return nil, err
		}

		//update endpoints on GlobalDNS status field
		ingressEndpoints := ic.gatherIngressEndpoints(obj.Status.LoadBalancer.Ingress)
		if obj.DeletionTimestamp != nil {
			err = ic.removeGlobalDNSEndpoints(globalDNS, ingressEndpoints)
		} else {
			err = ic.updateGlobalDNSEndpoints(globalDNS, ingressEndpoints)
		}
		return nil, err
	}
	return nil, nil
}

func (ic *UserIngressController) noGlobalDNS() bool {
	gd, err := ic.globalDNSLister.List("", labels.NewSelector())
	if err != nil {
		return true
	}

	return len(gd) == 0
}

func (ic *UserIngressController) gatherIngressEndpoints(ingressEps []v1.LoadBalancerIngress) []string {
	endpoints := []string{}
	for _, ep := range ingressEps {
		if ep.IP != "" {
			endpoints = append(endpoints, ep.IP)
		} else if ep.Hostname != "" {
			endpoints = append(endpoints, ep.Hostname)
		}
	}
	return endpoints
}

func (ic *UserIngressController) updateGlobalDNSEndpoints(globalDNS *v3.GlobalDNS, ingressEndpoints []string) error {

	originalLen := len(globalDNS.Status.Endpoints)
	globalDNS.Status.Endpoints = append(globalDNS.Status.Endpoints, ingressEndpoints...)

	if originalLen > 0 {
		//dedup the endpoints
		mapEndpoints := make(map[string]bool)
		res := []string{}
		for _, ep := range globalDNS.Status.Endpoints {
			if !mapEndpoints[ep] {
				mapEndpoints[ep] = true
				res = append(res, ep)
			}
		}
		globalDNS.Status.Endpoints = res
	}
	_, err := ic.globalDNSs.Update(globalDNS)
	if err != nil {
		return fmt.Errorf("UserIngressController: Failed to update GlobalDNS endpoints with error %v", err)
	}

	return nil
}

func (ic *UserIngressController) removeGlobalDNSEndpoints(globalDNS *v3.GlobalDNS, ingressEndpoints []string) error {
	mapRemovedEndpoints := make(map[string]bool)
	for _, ep := range ingressEndpoints {
		mapRemovedEndpoints[ep] = true
	}

	res := []string{}
	for _, ep := range globalDNS.Status.Endpoints {
		if !mapRemovedEndpoints[ep] {
			res = append(res, ep)
		}
	}
	globalDNS.Status.Endpoints = res
	_, err := ic.globalDNSs.Update(globalDNS)
	if err != nil {
		return fmt.Errorf("UserIngressController: Failed to update GlobalDNS endpoints on ingress deletion, with error %v", err)
	}

	return nil
}

func (ic *UserIngressController) findGlobalDNS(fqdnRequested string) (*v3.GlobalDNS, error) {

	allGlobalDNSs, err := ic.globalDNSLister.List("", labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("UserIngressController: Error listing GlobalDNSs %v", err)
	}

	for _, gd := range allGlobalDNSs {
		if strings.EqualFold(gd.Spec.FQDN, fqdnRequested) {
			return gd, nil
		}
	}

	return nil, nil
}

func (ic *UserIngressController) isProjectApproved(projectsApproved []string, project string) bool {
	for _, listedProject := range projectsApproved {
		split := strings.SplitN(listedProject, ":", 2)
		if len(split) != 2 {
			logrus.Errorf("UserIngressController: Error in splitting project ID %v", listedProject)
			return false
		}
		listedProjectName := split[1]
		if strings.EqualFold(listedProjectName, project) {
			return true
		}
	}
	return false
}

func (ic *UserIngressController) checkForMultiClusterApp(obj *v1beta1.Ingress, globalDNS *v3.GlobalDNS) error {
	if globalDNS.Spec.MultiClusterAppName != "" {
		ingressLabels := obj.Labels
		appID := ingressLabels[appSelectorLabel]

		if appID != "" {
			//find the app CR
			// go through all projects from multiclusterapp's targets
			split := strings.SplitN(globalDNS.Spec.MultiClusterAppName, ":", 2)
			if len(split) != 2 {
				return fmt.Errorf("error in splitting multiclusterapp ID %v", globalDNS.Spec.MultiClusterAppName)
			}
			mcappName := split[1]
			mcapp, err := ic.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
			if err != nil {
				return err
			}
			for _, t := range mcapp.Spec.Targets {
				split := strings.SplitN(t.ProjectName, ":", 2)
				if len(split) != 2 {
					return fmt.Errorf("error in splitting project ID %v", t.ProjectName)
				}
				projectNS := split[1]
				userApp, err := ic.appLister.Get(projectNS, appID)
				if err != nil {
					return fmt.Errorf("UserIngressController: Cannot find the App with the Id %v", userApp)
				}
				if !strings.EqualFold(userApp.Spec.MultiClusterAppName, globalDNS.Spec.MultiClusterAppName) {
					return fmt.Errorf("UserIngressController: Cannot configure DNS since the App is not part of MulticlusterApp %v", globalDNS.Spec.MultiClusterAppName)
				}
			}
		}
	}
	return nil
}

func (ic *UserIngressController) checkForProjects(obj *v1beta1.Ingress, globalDNS *v3.GlobalDNS) error {
	if len(globalDNS.Spec.ProjectNames) > 0 {
		ns, err := ic.namespaceLister.Get("", obj.Namespace)
		if err != nil {
			return fmt.Errorf("UserIngressController: Cannot find the App's namespace with the Id %v, error: %v", obj.Namespace, err)
		}
		nameSpaceProject := ns.ObjectMeta.Labels[projectSelectorLabel]

		if !ic.isProjectApproved(globalDNS.Spec.ProjectNames, nameSpaceProject) {
			return fmt.Errorf("UserIngressController: Cannot configure DNS since the App's project '%v' does not match GlobalDNS projectNames %v", nameSpaceProject, globalDNS.Spec.ProjectNames)
		}
	}
	return nil
}
