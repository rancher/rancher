package cluster

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/user"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ActionHandler struct {
	NodepoolGetter                v3.NodePoolsGetter
	ClusterClient                 v3.ClusterInterface
	NodeTemplateGetter            v3.NodeTemplatesGetter
	UserMgr                       user.Manager
	ClusterManager                *clustermanager.Manager
	BackupClient                  v3.EtcdBackupInterface
	ClusterScanClient             v3.ClusterScanInterface
	ClusterTemplateClient         v3.ClusterTemplateInterface
	ClusterTemplateRevisionClient v3.ClusterTemplateRevisionInterface
	SubjectAccessReviewClient     v1.SubjectAccessReviewInterface
	CisBenchmarkVersionClient     v3.CisBenchmarkVersionInterface
	CisBenchmarkVersionLister     v3.CisBenchmarkVersionLister
	CisConfigClient               v3.CisConfigInterface
	CisConfigLister               v3.CisConfigLister
}

func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	canUpdateCluster := func() bool {
		cluster := map[string]interface{}{
			"id": apiContext.ID,
		}

		return apiContext.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, cluster, apiContext.Schema) == nil
	}

	canBackupEtcd := func() bool {
		etcdBackupSchema := types.Schema{ID: mgmtclient.EtcdBackupType}
		// pkg/rbac/access_control.go:55 canAccess checks for the object's ID or namespace. The ns for etcdbackup will be the clusterID
		backupMap := map[string]interface{}{
			"namespaceId": apiContext.ID,
		}
		return apiContext.AccessControl.CanDo(v3.EtcdBackupGroupVersionKind.Group, v3.EtcdBackupResource.Name, "create", apiContext, backupMap, &etcdBackupSchema) == nil
	}

	canCreateClusterTemplate := func() bool {

		callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
		canCreateTemplates, _ := CanCreateRKETemplate(callerID, a.SubjectAccessReviewClient)
		return canCreateTemplates
	}

	switch actionName {
	case v32.ClusterActionGenerateKubeconfig:
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case v32.ClusterActionImportYaml:
		return a.ImportYamlHandler(actionName, action, apiContext)
	case v32.ClusterActionExportYaml:
		return a.ExportYamlHandler(actionName, action, apiContext)
	case v32.ClusterActionViewMonitoring:
		return a.viewMonitoring(actionName, action, apiContext)
	case v32.ClusterActionEditMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not access")
		}
		return a.editMonitoring(actionName, action, apiContext)
	case v32.ClusterActionEnableMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not access")
		}
		return a.enableMonitoring(actionName, action, apiContext)
	case v32.ClusterActionDisableMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not access")
		}
		return a.disableMonitoring(actionName, action, apiContext)
	case v32.ClusterActionBackupEtcd:
		if !canBackupEtcd() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not backup etcd")
		}
		return a.BackupEtcdHandler(actionName, action, apiContext)
	case v32.ClusterActionRestoreFromEtcdBackup:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not restore etcd backup")
		}
		return a.RestoreFromEtcdBackupHandler(actionName, action, apiContext)
	case v32.ClusterActionRotateCertificates:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not rotate certificates")
		}
		return a.RotateCertificates(actionName, action, apiContext)
	case v32.ClusterActionRunSecurityScan:
		return a.runCisScan(actionName, action, apiContext)
	case v32.ClusterActionSaveAsTemplate:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not save the cluster as an RKETemplate")
		}
		if !canCreateClusterTemplate() {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not save the cluster as an RKETemplate")
		}
		return a.saveAsTemplate(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) getClusterToken(clusterID string, apiContext *types.APIContext) (string, error) {
	userName := a.UserMgr.GetUser(apiContext)
	return a.UserMgr.EnsureClusterToken(clusterID, fmt.Sprintf("kubeconfig-%s.%s", userName, clusterID), "Kubeconfig token", "kubeconfig", userName)
}

func (a ActionHandler) getToken(apiContext *types.APIContext) (string, error) {
	userName := a.UserMgr.GetUser(apiContext)
	return a.UserMgr.EnsureToken("kubeconfig-"+userName, "Kubeconfig token", "kubeconfig", userName)
}

func (a ActionHandler) getKubeConfig(apiContext *types.APIContext, cluster *mgmtclient.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.getToken(apiContext)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.ID, token), nil
}
