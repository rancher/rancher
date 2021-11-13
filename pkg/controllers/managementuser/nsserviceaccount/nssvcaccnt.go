package nsserviceaccount

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/norman/types/slice"
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
	serviceAccounts rv1.ServiceAccountInterface
	nsLister        rv1.NamespaceLister
	projectLister   v3.ProjectLister
	clusterName     string
}

func Register(ctx context.Context, cluster *config.UserContext) {
	logrus.Debugf("Registering defaultSvcAccountHandler for checking default service account of system namespaces")
	nsh := &defaultSvcAccountHandler{
		serviceAccounts: cluster.Core.ServiceAccounts(""),
		nsLister:        cluster.Core.Namespaces("").Controller().Lister(),
		clusterName:     cluster.ClusterName,
		projectLister:   cluster.Management.Management.Projects("").Controller().Lister(),
	}
	cluster.Core.ServiceAccounts("").AddHandler(ctx, "defaultSvcAccountHandler", nsh.Sync)
}

func (nsh *defaultSvcAccountHandler) Sync(key string, sa *corev1.ServiceAccount) (runtime.Object, error) {
	if sa == nil || sa.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("defaultSvcAccountHandler: Sync service account: key=%v", key)
	//handle default svcAccount of system namespaces only
	if err := nsh.handleIfSystemNSDefaultSA(sa); err != nil {
		logrus.Errorf("defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=%v, err=%v", key, err)
	}
	return nil, nil
}

func (nsh *defaultSvcAccountHandler) handleIfSystemNSDefaultSA(defSvcAccnt *corev1.ServiceAccount) error {
	//check if sa is "default"
	if defSvcAccnt.Name != "default" {
		return nil
	}
	//check if ns is a system-ns
	namespace := defSvcAccnt.Namespace
	proj, err := project.GetSystemProject(nsh.clusterName, nsh.projectLister)
	if err != nil {
		return err
	}
	sysProjAnn := fmt.Sprintf("%v:%v", nsh.clusterName, proj.Name)
	if namespace == "kube-system" || namespace == "default" || (!nsh.isSystemNS(namespace) && !nsh.isSystemProjectNS(namespace, sysProjAnn)) {
		return nil
	}
	if defSvcAccnt.AutomountServiceAccountToken != nil && *defSvcAccnt.AutomountServiceAccountToken == false {
		return nil
	}
	automountServiceAccountToken := false
	defSvcAccnt.AutomountServiceAccountToken = &automountServiceAccountToken
	logrus.Debugf("defaultSvcAccountHandler: updating default service account key=%v", defSvcAccnt)
	_, err = nsh.serviceAccounts.Update(defSvcAccnt)
	if err != nil {
		logrus.Errorf("defaultSvcAccountHandler: error updating default service account flag for namespace: %v, err=%+v", namespace, err)
		return err
	}
	return nil
}

func (nsh *defaultSvcAccountHandler) isSystemNS(namespace string) bool {
	systemNamespacesStr := settings.SystemNamespaces.Get()
	if systemNamespacesStr == "" {
		return false
	}
	systemNamespaces := strings.Split(systemNamespacesStr, ",")
	return slice.ContainsString(systemNamespaces, namespace)
}

func (nsh *defaultSvcAccountHandler) isSystemProjectNS(namespace string, sysProjectAnnotation string) bool {
	nsObj, err := nsh.nsLister.Get("", namespace)
	if err != nil {
		return false
	}
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
