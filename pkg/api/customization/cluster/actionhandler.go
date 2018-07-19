package clusteregistrationtokens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	yaml2 "github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/rancher/pkg/kubectl"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/cluster/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	"github.com/rancher/types/user"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ActionHandler struct {
	NodepoolGetter     v3.NodePoolsGetter
	ClusterClient      v3.ClusterInterface
	NodeTemplateGetter v3.NodeTemplatesGetter
	UserMgr            user.Manager
	ClusterManager     *clustermanager.Manager
}

func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "generateKubeconfig":
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case "importYaml":
		return a.ImportYamlHandler(actionName, action, apiContext)
	case "exportYaml":
		return a.ExportYamlHandler(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) getToken(apiContext *types.APIContext) (string, error) {
	userName := a.UserMgr.GetUser(apiContext)
	return a.UserMgr.EnsureToken("kubeconfig-"+userName, "Kubeconfig token", userName)
}

func (a ActionHandler) GenerateKubeconfigActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var cluster managementv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}

	userName := a.UserMgr.GetUser(apiContext)
	token, err := a.getToken(apiContext)
	if err != nil {
		return err
	}
	cfg, err := kubeconfig.ForTokenBased(cluster.Name, apiContext.ID, apiContext.Request.Host, userName, token)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"config": cfg,
		"type":   "generateKubeconfigOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (a ActionHandler) ImportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}

	input := managementv3.ImportClusterYamlInput{}
	if err = json.Unmarshal(data, &input); err != nil {
		return errors.Wrap(err, "unmarshaling input error")
	}

	var cluster managementv3.Cluster
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
	cluster, err := a.ClusterClient.Get(apiContext.ID, metav1.GetOptions{})
	if err != nil {
		return err
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
	if cluster.Status.Driver == "rancherKubernetesEngine" {
		nodepools, err := a.NodepoolGetter.NodePools(cluster.Name).List(metav1.ListOptions{})
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
			nodeTemplate, err := a.NodeTemplateGetter.NodeTemplates(namespace).Get(id, metav1.GetOptions{})
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
	buf, err := yaml2.JSONToYAML(data)
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

type noopCloser struct {
	io.Reader
}

func (noopCloser) Close() error {
	return nil
}

func findNamespaceCreates(inputYAML string) ([]string, error) {
	var namespaces []string

	reader := yaml.NewDocumentDecoder(noopCloser{Reader: bytes.NewBufferString(inputYAML)})
	for {
		next, readErr := ioutil.ReadAll(reader)
		if readErr != nil && readErr != io.ErrShortBuffer {
			return nil, readErr
		}

		obj := &unstructured.Unstructured{}
		next, err := yaml2.YAMLToJSON(next)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(next, &obj.Object)
		if err != nil {
			return nil, err
		}

		if obj.IsList() {
			obj.EachListItem(func(obj runtime.Object) error {
				metadata, err := meta.Accessor(obj)
				if err != nil {
					return err
				}
				if obj.GetObjectKind().GroupVersionKind().Kind == "Namespace" && obj.GetObjectKind().GroupVersionKind().Version == "v1" {
					namespaces = append(namespaces, metadata.GetName())
				}

				if metadata.GetNamespace() != "" {
					namespaces = append(namespaces, metadata.GetNamespace())
				}
				return nil
			})
		} else if obj.GetKind() == "Namespace" && obj.GetAPIVersion() == "v1" {
			namespaces = append(namespaces, obj.GetName())
			if obj.GetNamespace() != "" {
				namespaces = append(namespaces, obj.GetNamespace())
			}
		}

		if readErr == nil {
			break
		}
	}

	uniq := map[string]bool{}
	var newNamespaces []string
	for _, ns := range namespaces {
		if !uniq[ns] {
			uniq[ns] = true
			newNamespaces = append(newNamespaces, ns)
		}
	}

	return newNamespaces, nil
}

func (a ActionHandler) findOrCreateProjectNamespaces(apiContext *types.APIContext, namespaces []string, clusterName, projectName string) (corev1.NamespaceInterface, error) {
	userCtx, err := a.ClusterManager.UserContext(clusterName)
	if err != nil {
		return nil, err
	}

	nsClient := userCtx.Core.Namespaces("")

	for _, ns := range namespaces {
		nsObj, err := nsClient.Get(ns, metav1.GetOptions{})
		if errors2.IsNotFound(err) {
			apiContext.SubContext = map[string]string{
				"/v3/schemas/cluster": clusterName,
			}
			err := access.Create(apiContext, &schema.Version, client.NamespaceType, map[string]interface{}{
				client.NamespaceFieldName:      ns,
				client.NamespaceFieldProjectID: projectName,
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

func waitForNS(nsClient corev1.NamespaceInterface, namespaces []string) {
	for i := 0; i < 3; i++ {
		allGood := true
		for _, ns := range namespaces {
			ns, err := nsClient.Get(ns, metav1.GetOptions{})
			if err != nil {
				allGood = false
				break
			}
			status := ns.Annotations["cattle.io/status"]
			if status == "" {
				allGood = false
				break
			}
			nsMap := map[string]interface{}{}
			err = json.Unmarshal([]byte(status), &nsMap)
			if err != nil {
				allGood = false
				break
			}

			foundCond := false
			conds := convert.ToMapSlice(values.GetValueN(nsMap, "Conditions"))
			for _, cond := range conds {
				if cond["Type"] == "InitialRolesPopulated" && cond["Status"] == "True" {
					foundCond = true
				}
			}

			if !foundCond {
				allGood = false
			}
		}

		if allGood {
			break
		} else {
			time.Sleep(2 * time.Second)
		}
	}
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

func (a ActionHandler) getKubeConfig(apiContext *types.APIContext, cluster *managementv3.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.getToken(apiContext)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.ID, token), nil
}
