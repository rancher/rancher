package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/apis/cluster.cattle.io/v3/schema"
	corev1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	clusterclient "github.com/rancher/rancher/pkg/types/client/cluster/v3"
	mgmtclient "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/compose"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) ImportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	data, err := ioutil.ReadAll(apiContext.Request.Body)
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

	cfg, err := a.getKubeConfig(apiContext, &cluster)
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

func (a ActionHandler) ExportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return err
	}

	if cluster.Status.Driver != v3.ClusterDriverRKE {
		return fmt.Errorf("cluster %v does not support being exported", cluster.Name)
	}

	topkey := compose.Config{}
	topkey.Version = "v3"
	c := mgmtclient.Cluster{}
	if err := convert.ToObj(cluster.Spec, &c); err != nil {
		return err
	}
	topkey.Clusters = map[string]mgmtclient.Cluster{}
	topkey.Clusters[cluster.Spec.DisplayName] = c

	// if driver is rancherKubernetesEngine, add any nodePool if found
	if cluster.Status.Driver == v3.ClusterDriverRKE {
		nodepools, err := a.NodepoolGetter.NodePools(cluster.Name).List(v1.ListOptions{})
		if err != nil {
			return err
		}
		topkey.NodePools = map[string]mgmtclient.NodePool{}
		for _, nodepool := range nodepools.Items {
			n := mgmtclient.NodePool{}
			if err := convert.ToObj(nodepool.Spec, &n); err != nil {
				return err
			}
			n.ClusterID = cluster.Spec.DisplayName
			namespace, id := ref.Parse(nodepool.Spec.NodeTemplateName)
			nodeTemplate, err := a.NodeTemplateGetter.NodeTemplates(namespace).Get(id, v1.GetOptions{})
			if err != nil {
				return err
			}
			n.NodeTemplateID = nodeTemplate.Spec.DisplayName
			topkey.NodePools[nodepool.Name] = n
		}
	}

	m, err := convert.EncodeToMap(topkey)
	if err != nil {
		return err
	}
	delete(m["clusters"].(map[string]interface{})[cluster.Spec.DisplayName].(map[string]interface{}), "actions")
	delete(m["clusters"].(map[string]interface{})[cluster.Spec.DisplayName].(map[string]interface{}), "links")
	for name := range topkey.NodePools {
		delete(m["nodePools"].(map[string]interface{})[name].(map[string]interface{}), "actions")
		delete(m["nodePools"].(map[string]interface{})[name].(map[string]interface{}), "links")
	}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	buf, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	if apiContext.ResponseFormat == "yaml" {
		reader := bytes.NewReader(buf)
		apiContext.Response.Header().Set("Content-Type", "application/yaml")
		http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
		return nil
	}
	r := v3.ExportOutput{
		YAMLOutput: string(buf),
	}
	jsonOutput, err := json.Marshal(r)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(jsonOutput)
	apiContext.Response.Header().Set("Content-Type", "application/json")
	http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
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

func (a ActionHandler) findOrCreateProjectNamespaces(apiContext *types.APIContext, namespaces []string, clusterName, projectName string) (corev1.NamespaceInterface, error) {
	userCtx, err := a.ClusterManager.UserContext(clusterName)
	if err != nil {
		return nil, err
	}

	nsClient := userCtx.Core.Namespaces("")

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
		} else if nsObj.Annotations[nslabels.ProjectIDFieldLabel] == projectName {
			// nothing
		} else {
			return nil, fmt.Errorf("Namespace [%s] already exists in project [%s]", ns, nsObj.Annotations[nslabels.ProjectIDFieldLabel])
		}
	}

	return nsClient, nil
}
