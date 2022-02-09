package cluster

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) RotateEncryptionKey(actionName string, action *types.Action, apiContext *types.APIContext) error {
	response := map[string]interface{}{
		"type": v3client.RotateEncryptionKeyOutputType,
		v3client.RotateEncryptionKeyOutputFieldMessage: "starting rotate encryption key",
	}

	var mgmtCluster mgmtv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &mgmtCluster); err != nil {
		response[v3client.RotateEncryptionKeyOutputFieldMessage] = "cluster does not exist"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get cluster by ID %s", apiContext.ID)
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		response[v3client.RotateEncryptionKeyOutputFieldMessage] = "cluster does not exist"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get cluster by ID %s", apiContext.ID)
	}

	if err := checkEncryptionConfig(cluster); err != nil {
		return httperror.NewAPIError(httperror.InvalidAction, err.Error())
	}

	if !v3.ClusterConditionUpdated.IsTrue(cluster) {
		return httperror.NewAPIError(httperror.InvalidAction, "cluster is in updating state")
	}

	cluster.Spec.RancherKubernetesEngineConfig.RotateEncryptionKey = true
	if _, err := a.ClusterClient.Update(cluster); err != nil {
		response[v3client.RotateEncryptionKeyOutputFieldMessage] = "failed to update cluster object"
		apiContext.WriteResponse(http.StatusInternalServerError, response)
		return errors.Wrapf(err, "unable to update cluster %s", cluster.Name)
	}

	res, err := json.Marshal(response)
	if err != nil {
		return err
	}

	apiContext.Response.Header().Set("Content-Type", "application/json")
	http.ServeContent(apiContext.Response, apiContext.Request, v3.ClusterActionRotateEncryptionKey, time.Now(), bytes.NewReader(res))
	return nil
}

// checkEncryptionConfig validates that the secrets encryption is both enabled and not custom.
func checkEncryptionConfig(c *v3.Cluster) error {
	if c.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig == nil {
		return errors.New("secrets encryption configuration is not defined")
	}
	if !c.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.Enabled {
		return errors.New("secrets encryption is disabled")
	}
	if c.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig != nil {
		return errors.New("custom encryption configuration is not supported for key rotation action")
	}
	return nil
}
