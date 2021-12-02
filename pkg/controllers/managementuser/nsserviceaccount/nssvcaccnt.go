package nsserviceaccount

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nsserviceaccount"
	rv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectIDAnnotation    = "field.cattle.io/projectId"
	sysNamespaceAnnotation = "management.cattle.io/system-namespace"
)

type defaultSvcAccountHandler struct {
	namespaces    rv1.NamespaceInterface
	projectLister v3.ProjectLister
	clusterName   string
}

func Register(ctx context.Context, cluster *config.UserContext) {
	logrus.Debugf("Registering defaultSvcAccountHandler for checking default service account of system namespaces")
	nsh := &defaultSvcAccountHandler{
		namespaces:    cluster.Core.Namespaces(""),
		clusterName:   cluster.ClusterName,
		projectLister: cluster.Management.Management.Projects("").Controller().Lister(),
	}
	cluster.Core.Namespaces("").AddHandler(ctx, "defaultSvcAccountHandler", nsh.Sync)
}

func (nsh *defaultSvcAccountHandler) Sync(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("defaultSvcAccountHandler: Sync service account: key=%v", key)
	//handle default svcAccount of system namespaces only
	ret, err := nsh.handleIfSystemNSDefaultSA(ns)
	if err != nil {
		logrus.Errorf("defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=%v, err=%v", key, err)
	}
	return ret, err
}

func (nsh *defaultSvcAccountHandler) handleIfSystemNSDefaultSA(ns *corev1.Namespace) (runtime.Object, error) {
	namespace := ns.Name
	proj, err := project.GetSystemProject(nsh.clusterName, nsh.projectLister)
	if err != nil {
		return nil, err
	}
	sysProjAnn := fmt.Sprintf("%v:%v", nsh.clusterName, proj.Name)
	if namespace == "kube-system" || namespace == "default" || (!nsh.isSystemNS(namespace) && !nsh.isSystemProjectNS(ns, sysProjAnn)) {
		return nil, nil
	}
	if ns.Annotations[nsserviceaccount.NoDefaultSATokenAnnotation] != "true" {
		ns = ns.DeepCopy()
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[nsserviceaccount.NoDefaultSATokenAnnotation] = "true"
		return nsh.namespaces.Update(ns)
	}
	return ns, nil
}

func (nsh *defaultSvcAccountHandler) isSystemNS(namespace string) bool {
	systemNamespacesStr := settings.SystemNamespaces.Get()
	if systemNamespacesStr == "" {
		return false
	}
	systemNamespaces := strings.Split(systemNamespacesStr, ",")
	return slice.ContainsString(systemNamespaces, namespace)
}

func (nsh *defaultSvcAccountHandler) isSystemProjectNS(nsObj *corev1.Namespace, sysProjectAnnotation string) bool {
	if nsObj.Annotations == nil {
		return false
	}

	if val, ok := nsObj.Annotations[sysNamespaceAnnotation]; ok && val == "true" {
		return true
	}

	if prjAnnVal, ok := nsObj.Annotations[projectIDAnnotation]; ok && prjAnnVal == sysProjectAnnotation {
		return true
	}

	return false
}
