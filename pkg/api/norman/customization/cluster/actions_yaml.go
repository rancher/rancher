package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	clusterclient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/kubectl"
	schema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) ImportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	data, err := io.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}

	input := mgmtclient.ImportClusterYamlInput{}
	if err = json.Unmarshal(data, &input); err != nil {
		return errors.Wrap(err, "unmarshaling input error")
	}

	var cluster mgmtclient.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}

	cfg, err := a.generateKubeConfig(apiContext, &cluster)
	if err != nil {
		return err
	}
	for _, context := range cfg.Contexts {
		if input.DefaultNamespace == "" {
			context.Namespace = input.Namespace
		} else {
			context.Namespace = input.DefaultNamespace
		}
	}

	var msg []byte
	if input.ProjectID != "" {
		err = a.processYAML(apiContext, cluster.ID, input.ProjectID, input.YAML)
		if err == nil {
			msg, err = kubectl.Apply([]byte(input.YAML), cfg)
		}
	} else if input.Namespace == "" {
		msg, err = kubectl.Apply([]byte(input.YAML), cfg)
	} else {
		msg, err = kubectl.ApplyWithNamespace([]byte(input.YAML), input.Namespace, cfg)
	}
	rtn := map[string]interface{}{
		"message": string(msg),
		"type":    "importYamlOutput",
	}
	if err == nil {
		apiContext.WriteResponse(http.StatusOK, rtn)
	} else {
		if rtn["message"] == "" {
			rtn["message"] = err.Error()
		}
		apiContext.WriteResponse(http.StatusBadRequest, rtn)
	}

	return nil
}

func (a ActionHandler) processYAML(apiContext *types.APIContext, clusterName, projectName, inputYAML string) error {
	namespaces, err := findNamespaceCreates(inputYAML)
	if err != nil {
		return err
	}

	nsClient, err := a.findOrCreateProjectNamespaces(apiContext, namespaces, clusterName, projectName)
	if err != nil {
		return err
	}

	waitForNS(nsClient, namespaces)
	return nil
}

func (a ActionHandler) findOrCreateProjectNamespaces(apiContext *types.APIContext, namespaces []string, clusterName, projectName string) (wcorev1.NamespaceClient, error) {
	userCtx, err := a.ClusterManager.UserContextNoControllers(clusterName)
	if err != nil {
		return nil, err
	}

	nsClient := userCtx.Corew.Namespace()

	for _, ns := range namespaces {
		nsObj, err := nsClient.Get(ns, v1.GetOptions{})
		if kerrors.IsNotFound(err) {
			apiContext.SubContext = map[string]string{
				"/v3/schemas/cluster": clusterName,
			}
			err := access.Create(apiContext, &schema.Version, clusterclient.NamespaceType, map[string]interface{}{
				clusterclient.NamespaceFieldName:      ns,
				clusterclient.NamespaceFieldProjectID: projectName,
			}, nil)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else if nsObj.Annotations[nslabels.ProjectIDFieldLabel] != projectName {
			return nil, fmt.Errorf("Namespace [%s] already exists in project [%s]", ns, nsObj.Annotations[nslabels.ProjectIDFieldLabel])
		}
	}

	return nsClient, nil
}
