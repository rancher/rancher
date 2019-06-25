package cluster

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/user"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ActionHandler struct {
	NodepoolGetter     v3.NodePoolsGetter
	ClusterClient      v3.ClusterInterface
	NodeTemplateGetter v3.NodeTemplatesGetter
	UserMgr            user.Manager
	ClusterManager     *clustermanager.Manager
	BackupClient       v3.EtcdBackupInterface
	ClusterScanClient  v3.ClusterScanInterface
}

func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	canUpdateCluster := func() bool {
		cluster := map[string]interface{}{
			"id": apiContext.ID,
		}

		return apiContext.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, cluster, apiContext.Schema) == nil
	}

	switch actionName {
	case v3.ClusterActionGenerateKubeconfig:
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case v3.ClusterActionImportYaml:
		return a.ImportYamlHandler(actionName, action, apiContext)
	case v3.ClusterActionExportYaml:
		return a.ExportYamlHandler(actionName, action, apiContext)
	case v3.ClusterActionViewMonitoring:
		return a.viewMonitoring(actionName, action, apiContext)
	case v3.ClusterActionEditMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return a.editMonitoring(actionName, action, apiContext)
	case v3.ClusterActionEnableMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return a.enableMonitoring(actionName, action, apiContext)
	case v3.ClusterActionDisableMonitoring:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return a.disableMonitoring(actionName, action, apiContext)
	case v3.ClusterActionBackupEtcd:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not backup etcd")
		}
		return a.BackupEtcdHandler(actionName, action, apiContext)
	case v3.ClusterActionRestoreFromEtcdBackup:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not restore etcd backup")
		}
		return a.RestoreFromEtcdBackupHandler(actionName, action, apiContext)
	case v3.ClusterActionRotateCertificates:
		if !canUpdateCluster() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not rotate certificates")
		}
		return a.RotateCertificates(actionName, action, apiContext)
	case v3.ClusterActionRunCISScan:
		return a.runCISScan(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) getClusterToken(clusterID string, apiContext *types.APIContext) (string, error) {
	userName := a.UserMgr.GetUser(apiContext)
	return a.UserMgr.EnsureClusterToken(clusterID, fmt.Sprintf("kubeconfig-%s.%s", userName, clusterID), "Kubeconfig token", userName)
}

func (a ActionHandler) getToken(apiContext *types.APIContext) (string, error) {
	userName := a.UserMgr.GetUser(apiContext)
	return a.UserMgr.EnsureToken("kubeconfig-"+userName, "Kubeconfig token", userName)
}

func (a ActionHandler) getKubeConfig(apiContext *types.APIContext, cluster *mgmtclient.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.getToken(apiContext)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.ID, token), nil
}
