package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	rketypes "github.com/rancher/rke/types"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) RotateCertificates(actionName string, action *types.Action, apiContext *types.APIContext) error {
	rtn := map[string]interface{}{
		"type":    "rotateCertificateOutput",
		"message": "rotating certificates for all components",
	}
	var mgmtCluster mgmtv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &mgmtCluster); err != nil {
		rtn["message"] = "none existent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, rtn)

		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		rtn["message"] = "none existent Cluster"
		apiContext.WriteResponse(http.StatusBadRequest, rtn)

		return errors.Wrapf(err, "failed to get Cluster by ID %s", apiContext.ID)
	}
	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		rtn["message"] = "reading request body error"
		apiContext.WriteResponse(http.StatusBadRequest, rtn)

		return errors.Wrapf(err, "failed to read request body")
	}

	input := client.RotateCertificateInput{}
	if err = json.Unmarshal(data, &input); err != nil {
		rtn["message"] = "failed to parse request content"
		apiContext.WriteResponse(http.StatusBadRequest, rtn)

		return errors.Wrap(err, "unmarshaling input error")
	}

	rotateCerts := &rketypes.RotateCertificates{
		CACertificates: input.CACertificates,
		Services:       []string{input.Services},
	}
	cluster.Spec.RancherKubernetesEngineConfig.RotateCertificates = rotateCerts
	if _, err := a.ClusterClient.Update(cluster); err != nil {
		rtn["message"] = "failed to update cluster object"
		apiContext.WriteResponse(http.StatusInternalServerError, rtn)

		return errors.Wrapf(err, "unable to update Cluster %s", cluster.Name)
	}
	if input.CACertificates {
		rtn["message"] = "rotating CA certificates and all components"
	} else if len(input.Services) > 0 {
		rtn["message"] = fmt.Sprintf("rotating %s certificates", input.Services)
	} else {
		rtn["message"] = "rotating certificates for all components"
	}

	apiContext.WriteResponse(http.StatusOK, rtn)
	return nil
}
