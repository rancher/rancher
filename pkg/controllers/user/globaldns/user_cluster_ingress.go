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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	annotationGlobalDNS         = "rancher.io/globalDNS.hostname"
	appSelectorLabel            = "io.cattle.field/appId"
	projectSelectorLabel        = "field.cattle.io/projectId"
	UserIngressControllerName   = "globaldns-useringress-controller"
	UserGlobalDNSControllerName = "user-controller-watching-globaldns"
)

type UserIngressController struct {
	ingressLister         v1beta1Rancher.IngressLister
	globalDNSs            v3.GlobalDNSInterface
	globalDNSLister       v3.GlobalDNSLister
	appLister             projectv3.AppLister
	namespaceLister       v1Rancher.NamespaceLister
	multiclusterappLister v3.MultiClusterAppLister
	clusterName           string
}

func newUserIngressController(ctx context.Context, clusterContext *config.UserContext) *UserIngressController {
	n := &UserIngressController{
		ingressLister:         clusterContext.Extensions.Ingresses("").Controller().Lister(),
		globalDNSs:            clusterContext.Management.Management.GlobalDNSs(""),
		globalDNSLister:       clusterContext.Management.Management.GlobalDNSs("").Controller().Lister(),
		appLister:             clusterContext.Management.Project.Apps("").Controller().Lister(),
		namespaceLister:       clusterContext.Core.Namespaces("").Controller().Lister(),
		multiclusterappLister: clusterContext.Management.Management.MultiClusterApps("").Controller().Lister(),
		clusterName:           clusterContext.ClusterName,
	}
	return n
}

func Register(ctx context.Context, clusterContext *config.UserContext) {
	n := newUserIngressController(ctx, clusterContext)
	clusterContext.Extensions.Ingresses("").AddHandler(ctx, UserIngressControllerName, n.sync)
	g := newUserGlobalDNSController(clusterContext)
	clusterContext.Management.Management.GlobalDNSs("").AddHandler(ctx, UserGlobalDNSControllerName, g.sync)
}

func (ic *UserIngressController) sync(key string, obj *v1beta1.Ingress) (runtime.Object, error) {
	if obj == nil {
		return ic.reconcileAllGlobalDNSs()
	}
	//if there are no globaldns cr, skip this run. If error, return error

	noGlobalDNS, err := ic.noGlobalDNS()
	if err != nil {
		return nil, err
	}

	if noGlobalDNS {
		return nil, nil
	}

	annotations := obj.Annotations
	//look for globalDNS annotation, if found load the GlobalDNS if there are Ingress endpoints
	if annotations[annotationGlobalDNS] != "" && len(obj.Status.LoadBalancer.Ingress) > 0 {
		fqdnRequested := annotations[annotationGlobalDNS]
		//check if the corresponding GlobalDNS CR is present
		globalDNS, err := ic.findGlobalDNS(fqdnRequested)

		if globalDNS == nil || (err != nil && k8serrors.IsNotFound(err)) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("UserIngressController: Error %v while finding  GlobalDNS resource for FQDN requested %v", err, fqdnRequested)
		}

		//check if 'multiclusterappID' or targetProjects on GlobalDNS CR matches the ingress's app/project
		if obj.DeletionTimestamp == nil {
			var checkPassed bool
			if globalDNS.Spec.MultiClusterAppName != "" {
				checkPassed, err = ic.checkForMultiClusterApp(obj, globalDNS)
			} else if len(globalDNS.Spec.ProjectNames) > 0 {
				//check if 'projectNames' on GlobalDNS CR matches to the user's project for multiclusterapp
				checkPassed, err = ic.checkForProjects(obj, globalDNS)
			}
			if err != nil {
				return nil, err
			}
			if !checkPassed {
				logrus.Debugf("UserIngressController: Not enqueing update on globaldns %v, for ingress key %v", globalDNS, key)
				return nil, nil
			}
		}
		//update endpoints on GlobalDNS status field by enqueueing update on GlobalDNS
		ic.globalDNSs.Controller().Enqueue(namespace.GlobalNamespace, globalDNS.Name)

		return nil, err
	}
	return nil, nil
}

func (ic *UserIngressController) noGlobalDNS() (bool, error) {
	gd, err := ic.globalDNSLister.List(namespace.GlobalNamespace, labels.NewSelector())
	if err != nil {
		return true, err
	}

	return len(gd) == 0, nil
}

func (ic *UserIngressController) reconcileAllGlobalDNSs() (runtime.Object, error) {
	globalDNSList, err := ic.globalDNSLister.List(namespace.GlobalNamespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}

	for _, globalDNSObj := range globalDNSList {
		//call update on each GlobalDNS obj that refers to this current cluster
		targetsCluster, err := ic.doesGlobalDNSTargetCurrentCluster(globalDNSObj)
		if err != nil {
			return nil, err
		}
		if targetsCluster {
			//enqueue it to the globalDNS controller
			ic.globalDNSs.Controller().Enqueue(namespace.GlobalNamespace, globalDNSObj.Name)
		}
	}
	return nil, nil
}

func (ic *UserIngressController) doesGlobalDNSTargetCurrentCluster(globalDNS *v3.GlobalDNS) (bool, error) {
	var targetProjectNames []string

	if globalDNS.Spec.MultiClusterAppName != "" {
		mcappName, err := getMultiClusterAppName(globalDNS.Spec.MultiClusterAppName)
		if err != nil {
			return false, err
		}
		mcapp, err := ic.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
		if err != nil {
			return false, err
		}
		for _, t := range mcapp.Spec.Targets {
			targetProjectNames = append(targetProjectNames, t.ProjectName)
		}
	} else if len(globalDNS.Spec.ProjectNames) > 0 {
		targetProjectNames = append(targetProjectNames, globalDNS.Spec.ProjectNames...)
	}

	// go through target projects and check if its part of the current cluster
	for _, projectNameSet := range targetProjectNames {
		split := strings.SplitN(projectNameSet, ":", 2)
		if len(split) != 2 {
			return false, fmt.Errorf("UserIngressController: Error in splitting project Name %v", projectNameSet)
		}
		// check if the project in this iteration belongs to the same cluster in current context
		if split[0] == ic.clusterName {
			return true, nil
		}
	}

	return false, nil
}

func (ic *UserIngressController) findGlobalDNS(fqdnRequested string) (*v3.GlobalDNS, error) {
	allGlobalDNSs, err := ic.globalDNSLister.List(namespace.GlobalNamespace, labels.NewSelector())
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

func (ic *UserIngressController) checkForMultiClusterApp(obj *v1beta1.Ingress, globalDNS *v3.GlobalDNS) (bool, error) {
	ingressLabels := obj.Labels
	appID := ingressLabels[appSelectorLabel]

	if appID != "" {
		//find the app CR
		// go through all projects from multiclusterapp's targets
		mcappName, err := getMultiClusterAppName(globalDNS.Spec.MultiClusterAppName)
		if err != nil {
			return false, err
		}
		mcapp, err := ic.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
		if err != nil && k8serrors.IsNotFound(err) {
			logrus.Debugf("UserIngressController: multiclusterapp not found by name %v", mcappName)
			//pass the check and let the controller continue in this case, it will enqueue update to globaldns and the other controller will figure out the endpoint updates
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("UserIngressController: Error %v listing multiclusterapp by name %v", err, mcappName)
		}
		for _, t := range mcapp.Spec.Targets {
			split := strings.SplitN(t.ProjectName, ":", 2)
			if len(split) != 2 {
				return false, fmt.Errorf("error in splitting project ID %v", t.ProjectName)
			}
			if split[0] != ic.clusterName {
				continue
			}
			projectNS := split[1]
			userApp, err := ic.appLister.Get(projectNS, appID)
			if err != nil && k8serrors.IsNotFound(err) {
				logrus.Debugf("UserIngressController: The app %v for this ingress is not under the target namespace %v of multiclusterapp", projectNS, appID)
				return false, nil
			} else if err != nil {
				return false, fmt.Errorf("UserIngressController: Error finding the App with the Id %v under namespace %v", appID, projectNS)
			}

			if strings.EqualFold(userApp.Spec.MultiClusterAppName, mcappName) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (ic *UserIngressController) checkForProjects(obj *v1beta1.Ingress, globalDNS *v3.GlobalDNS) (bool, error) {
	ns, err := ic.namespaceLister.Get("", obj.Namespace)
	if err != nil {
		return false, fmt.Errorf("UserIngressController: Cannot find the App's namespace with the Id %v, error: %v", obj.Namespace, err)
	}
	nameSpaceProject := ns.ObjectMeta.Labels[projectSelectorLabel]

	logrus.Debugf("UserIngressController: check if Ingress's project %v belongs to GlobalDNS target projects %v", nameSpaceProject, globalDNS.Spec.ProjectNames)

	if ic.isProjectApproved(globalDNS.Spec.ProjectNames, nameSpaceProject) {
		return true, nil
	}

	return false, nil
}
