package cluster

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/tokens"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (a ActionHandler) GenerateKubeconfigActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var err error
	var cluster mgmtclient.Cluster
	var nodes []*mgmtv3.Node
	if err = access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}
	if apiContext.Type == "cluster" {
		nodes, err = a.NodeLister.List(cluster.ID, labels.Everything())
		if err != nil {
			return err
		}
	}

	var (
		cfg      string
		tokenKey string
	)

	endpointEnabled := cluster.LocalClusterAuthEndpoint != nil && cluster.LocalClusterAuthEndpoint.Enabled

	generateToken := strings.EqualFold(settings.KubeconfigGenerateToken.Get(), "true")
	if generateToken {
		// generate token and place it in kubeconfig, token doesn't expire
		if endpointEnabled {
			tokenKey, err = a.ensureClusterToken(cluster.ID, apiContext)
		} else {
			tokenKey, err = a.ensureToken(apiContext)
		}
		if err != nil {
			return err
		}
	}

	host := settings.ServerURL.Get()
	if host == "" {
		host = apiContext.Request.Host
	} else {
		u, err := url.Parse(host)
		if err == nil {
			host = u.Host
		} else {
			host = apiContext.Request.Host
		}
	}

	if endpointEnabled {
		if err = a.createClusterAuthTokenDownstream(apiContext.ID, tokenKey); err != nil {
			return err
		}

		cfg, err = kubeconfig.ForClusterTokenBased(&cluster, nodes, apiContext.ID, host, tokenKey)
		if err != nil {
			return err
		}
	} else {
		cfg, err = kubeconfig.ForTokenBased(cluster.Name, apiContext.ID, host, tokenKey)
		if err != nil {
			return err
		}
	}

	data := map[string]interface{}{
		"config": cfg,
		"type":   "generateKubeconfigOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

// createClusterAuthTokenDownstream will create a ClusterAuthToken in the downstream cluster if the token hashing feature flag is enabled.
// This is required because, if the token hashing is enabled, then the controller will not be able to create the ClusterAuthToken
// in the downstream cluster because the token is stored hashed in the local cluster.
// If the token hashing feature flag is disabled, then this function is a no-op and the corresponding controller will create it.
// The reason the controller will create it in the second case is that a user may want to create a kubeconfig for the cluster
// before the cluster is active. Since the tokens are not hashed, then the controller can create the ClusterAuthToken in the downstream
// cluster when the cluster is ready and the action handler can return successfully.
func (a ActionHandler) createClusterAuthTokenDownstream(clusterName, tokenKey string) error {
	if features.TokenHashing.Enabled() {
		clusterClient, err := a.ClusterManager.UserContextNoControllers(clusterName)
		if err != nil {
			return err
		}

		if tokenKey != "" {
			tokenName, tokenValue := tokens.SplitTokenParts(tokenKey)
			// A lister is not used here because the token was recently created, therefore the lister would likely miss
			token, err := a.TokenClient.Get(tokenName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			clusterAuthToken, err := common.NewClusterAuthToken(token, tokenValue)
			if err != nil {
				return err
			}

			if _, err = clusterClient.Cluster.ClusterAuthTokens(namespace.System).Create(clusterAuthToken); err != nil {
				return err
			}
		}
	}

	return nil
}
