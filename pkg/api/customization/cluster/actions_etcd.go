package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/management/etcdbackup"
	"github.com/rancher/rancher/pkg/ref"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) BackupEtcdHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	response := map[string]interface{}{
		"message": "starting ETCD backup",
	}
	// checking access
	var mgmtCluster mgmtv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &mgmtCluster); err != nil {
		response["message"] = "none existent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		response["message"] = "none existent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}

	newBackup, err := etcdbackup.NewBackupObject(cluster, true)
	if err != nil {
		response["message"] = "failed to initialize etcdbackup object"
		apiContext.WriteResponse(http.StatusInternalServerError, response)
		return errors.Wrapf(err, "failed to initialize etcdbackup object")
	}

	backup, err := a.BackupClient.Create(newBackup)
	if err != nil {
		response["message"] = "failed to create etcdbackup object"
		apiContext.WriteResponse(http.StatusInternalServerError, response)
		return errors.Wrapf(err, "failed to cteate etcdbackup object")
	}
	backupJSON, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	apiContext.Response.Header().Set("Content-Type", "application/json")
	http.ServeContent(apiContext.Response, apiContext.Request, "backupEtcd", time.Now(), bytes.NewReader(backupJSON))
	return nil
}

func (a ActionHandler) RestoreFromEtcdBackupHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	response := map[string]interface{}{
		"message": "restoring etcdbackup for the cluster",
	}

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		response["message"] = "reading request body error"
		apiContext.WriteResponse(http.StatusInternalServerError, response)
		return errors.Wrap(err, "failed to read request body")
	}

	input := client.RestoreFromEtcdBackupInput{}
	if err = json.Unmarshal(data, &input); err != nil {
		response["message"] = "failed to parse request content"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrap(err, "unmarshaling input error")
	}
	// checking access
	var mgmtCluster client.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &mgmtCluster); err != nil {
		response["message"] = "nonexistent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		response["message"] = "nonexistent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, response)
		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}

	var backup *v3.EtcdBackup
	clusterBackupConfig := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig
	if clusterBackupConfig != nil && clusterBackupConfig.S3BackupConfig == nil {
		ns, name := ref.Parse(input.EtcdBackupID)
		if ns == "" || name == "" {
			return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid input id %s", input.EtcdBackupID))
		}
		backup, err = a.BackupClient.GetNamespaced(ns, name, v1.GetOptions{})
		if err != nil {
			response["message"] = "error getting backup config"
			apiContext.WriteResponse(http.StatusInternalServerError, response)
			return errors.Wrapf(err, "failed to get backup config by ID %s", input.EtcdBackupID)
		}
		if backup.Spec.BackupConfig.S3BackupConfig != nil {
			return httperror.NewAPIError(httperror.MethodNotAllowed,
				fmt.Sprintf("restoring S3 backups with no cluster level S3 configuration is not supported %s", input.EtcdBackupID))
		}
	}

	if input.RestoreRkeConfig != "" && backup.Status.ClusterObject == "" {
		// attempting to restore rke config and the backup does not contain data, probably pre 2.4 backup
		return httperror.NewAPIError(httperror.MethodNotAllowed,
			fmt.Sprintf("unable to restore RKE config, backup contains no cluster object: %s", input.EtcdBackupID))
	}

	// backup was taken in 2.4+ and has content
	switch strings.ToLower(input.RestoreRkeConfig) {
	case "kubernetesversion":
		// restore from copy stored inline to not have to decompress object
		cluster.Spec.RancherKubernetesEngineConfig.Version = backup.Status.KubernetesVersion
	case "all":
		clusterBackup, err := etcdbackup.DecompressCluster(backup.Status.ClusterObject)
		if err != nil {
			response["message"] = "error decompressing cluster object"
			apiContext.WriteResponse(http.StatusInternalServerError, response)
			return errors.Wrap(err,
				fmt.Sprintf("error decompressing cluster object for backupid %s: %s", input.EtcdBackupID, err))
		}
		cluster.Spec.RancherKubernetesEngineConfig = clusterBackup.Spec.RancherKubernetesEngineConfig
	}

	// flag cluster for restore
	cluster.Spec.RancherKubernetesEngineConfig.Restore.SnapshotName = input.EtcdBackupID
	cluster.Spec.RancherKubernetesEngineConfig.Restore.Restore = true

	if _, err = a.ClusterClient.Update(cluster); err != nil {
		response["message"] = "failed to update cluster object"
		apiContext.WriteResponse(http.StatusInternalServerError, response)
		return errors.Wrapf(err, "unable to update Cluster %s", cluster.Name)
	}
	apiContext.WriteResponse(http.StatusCreated, response)
	return nil
}
