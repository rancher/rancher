package cluster

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/catalog/manager"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ActionHandler used for performing various cluster actions.
type ActionHandler struct {
	NodepoolGetter                v3.NodePoolsGetter
	NodeLister                    v3.NodeLister
	ClusterClient                 v3.ClusterInterface
	CatalogManager                manager.CatalogManager
	NodeTemplateGetter            v3.NodeTemplatesGetter
	UserMgr                       user.Manager
	ClusterManager                *clustermanager.Manager
	CatalogTemplateVersionLister  v3.CatalogTemplateVersionLister
	BackupClient                  v3.EtcdBackupInterface
	ClusterTemplateClient         v3.ClusterTemplateInterface
	ClusterTemplateRevisionClient v3.ClusterTemplateRevisionInterface
	SubjectAccessReviewClient     v1.SubjectAccessReviewInterface
	TokenClient                   v3.TokenInterface
	Auth                          requests.Authenticator
}

func canUpdateCluster(apiContext *types.APIContext) bool {
	if apiContext == nil {
		return false
	}
	cluster := map[string]interface{}{
		"id": apiContext.ID,
	}
	return canUpdateClusterWithValues(apiContext, cluster)
}

func canUpdateClusterWithValues(apiContext *types.APIContext, values map[string]interface{}) bool {
	return apiContext.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, values, apiContext.Schema) == nil
}

func canBackupEtcd(apiContext *types.APIContext, namespace string) bool {
	if apiContext == nil {
		return false
	}
	etcdBackupSchema := types.Schema{ID: mgmtclient.EtcdBackupType}
	backupMap := map[string]interface{}{
		"namespaceId": namespace,
	}
	return apiContext.AccessControl.CanDo(v3.EtcdBackupGroupVersionKind.Group, v3.EtcdBackupResource.Name, "create", apiContext, backupMap, &etcdBackupSchema) == nil
}

func canCreateClusterTemplate(sar v1.SubjectAccessReviewInterface, apiContext *types.APIContext) bool {
	if apiContext == nil {
		return false
	}
	callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
	canCreateTemplates, _ := CanCreateRKETemplate(callerID, sar)
	return canCreateTemplates
}

// ClusterActionHandler runs the handler for the provided cluster action in the given context.
func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case v32.ClusterActionGenerateKubeconfig:
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case v32.ClusterActionImportYaml:
		return a.ImportYamlHandler(actionName, action, apiContext)
	case v32.ClusterActionExportYaml:
		return a.ExportYamlHandler(actionName, action, apiContext)
	case v32.ClusterActionBackupEtcd:
		if !canBackupEtcd(apiContext, apiContext.ID) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not backup etcd")
		}
		return a.BackupEtcdHandler(actionName, action, apiContext)
	case v32.ClusterActionRestoreFromEtcdBackup:
		if !canUpdateCluster(apiContext) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not restore etcd backup")
		}
		return a.RestoreFromEtcdBackupHandler(actionName, action, apiContext)
	case v32.ClusterActionRotateCertificates:
		if !canUpdateCluster(apiContext) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not rotate certificates")
		}
		return a.RotateCertificates(actionName, action, apiContext)
	case v32.ClusterActionRotateEncryptionKey:
		if !canUpdateCluster(apiContext) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not rotate encryption key")
		}
		return a.RotateEncryptionKey(actionName, action, apiContext)
	case v32.ClusterActionSaveAsTemplate:
		if !canUpdateCluster(apiContext) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not save the cluster as an RKETemplate")
		}
		if !canCreateClusterTemplate(a.SubjectAccessReviewClient, apiContext) {
			return httperror.NewAPIError(httperror.PermissionDenied, "can not save the cluster as an RKETemplate")
		}
		return a.saveAsTemplate(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

// ensureClusterToken will create a new kubeconfig token for the user in the provided context with the default TTL.
func (a ActionHandler) ensureClusterToken(clusterID string, apiContext *types.APIContext) (string, error) {
	input, err := a.createTokenInput(apiContext)
	if err != nil {
		return "", err
	}

	return a.UserMgr.EnsureClusterToken(clusterID, input)
}

// ensureToken will create a new kubeconfig token for the user in the provided context with the default TTL.
func (a ActionHandler) ensureToken(apiContext *types.APIContext) (string, error) {
	input, err := a.createTokenInput(apiContext)
	if err != nil {
		return "", err
	}

	return a.UserMgr.EnsureToken(input)
}

// createTokenInput will create the input for a new kubeconfig token with the default TTL.
func (a ActionHandler) createTokenInput(apiContext *types.APIContext) (user.TokenInput, error) {
	userName := a.UserMgr.GetUser(apiContext)
	tokenNamePrefix := fmt.Sprintf("kubeconfig-%s", userName)

	authToken, err := a.Auth.TokenFromRequest(apiContext.Request)
	if err != nil {
		return user.TokenInput{}, err
	}

	defaultTokenTTL, err := tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds()
	if err != nil {
		return user.TokenInput{}, fmt.Errorf("failed to get default token TTL: %w", err)
	}

	return user.TokenInput{
		TokenName:     tokenNamePrefix,
		Description:   "Kubeconfig token",
		Kind:          "kubeconfig",
		UserName:      userName,
		AuthProvider:  authToken.AuthProvider,
		TTL:           defaultTokenTTL,
		Randomize:     true,
		UserPrincipal: authToken.UserPrincipal,
	}, nil
}

func (a ActionHandler) generateKubeConfig(apiContext *types.APIContext, cluster *mgmtclient.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.ensureToken(apiContext)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.ID, token), nil
}
