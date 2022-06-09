package globaldns

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/ingresswrapper"
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
	globalDNSs            v3.GlobalDnsInterface
	globalDNSLister       v3.GlobalDnsLister
	multiclusterappLister v3.MultiClusterAppLister
	clusterName           string
}

func newUserIngressController(ctx context.Context, clusterContext *config.UserContext) *UserIngressController {
	n := &UserIngressController{
		globalDNSs:            clusterContext.Management.Management.GlobalDnses(""),
		globalDNSLister:       clusterContext.Management.Management.GlobalDnses("").Controller().Lister(),
		multiclusterappLister: clusterContext.Management.Management.MultiClusterApps("").Controller().Lister(),
		clusterName:           clusterContext.ClusterName,
	}
	return n
}

func Register(ctx context.Context, clusterContext *config.UserContext) {
	n := newUserIngressController(ctx, clusterContext)
	starter := clusterContext.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, clusterContext)
		return nil
	})

	globalDNS := clusterContext.Management.Management.GlobalDnses("")
	globalDNS.AddHandler(ctx, "globaldns-deferred", func(key string, obj *v3.GlobalDns) (runtime.Object, error) {
		if obj == nil {
			return obj, nil
		} else if ok, err := n.doesGlobalDNSTargetCurrentCluster(obj); err != nil {
			return obj, err
		} else if ok {
			return obj, starter()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, clusterContext *config.UserContext) {
	n := newUserIngressController(ctx, clusterContext)
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		clusterContext.Networking.Ingresses("").AddHandler(ctx, UserIngressControllerName, ingresswrapper.CompatSyncV1(n.sync))
	} else {
		clusterContext.Extensions.Ingresses("").AddHandler(ctx, UserIngressControllerName, ingresswrapper.CompatSyncV1Beta1(n.sync))
	}
	g := newUserGlobalDNSController(clusterContext)
	clusterContext.Management.Management.GlobalDnses("").AddHandler(ctx, UserGlobalDNSControllerName, g.sync)
}

func (ic *UserIngressController) sync(key string, obj ingresswrapper.Ingress) (runtime.Object, error) {
	return ic.reconcileAllGlobalDNSs()
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

func (ic *UserIngressController) doesGlobalDNSTargetCurrentCluster(globalDNS *v3.GlobalDns) (bool, error) {
	var targetProjectNames []string

	if globalDNS.Spec.MultiClusterAppName != "" {
		mcappName, err := getMultiClusterAppName(globalDNS.Spec.MultiClusterAppName)
		if err != nil {
			return false, err
		}
		mcapp, err := ic.multiclusterappLister.Get(namespace.GlobalNamespace, mcappName)
		if err != nil && k8serrors.IsNotFound(err) {
			logrus.Debugf("UserIngressController: reconcileallgdns, multiclusterapp not found by name %v, might be marked for deletion", mcappName)
			//pass the check and let the controller continue in this case, it will enqueue update to globaldns and the other controller will figure out the endpoint updates
			return true, nil
		}
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
