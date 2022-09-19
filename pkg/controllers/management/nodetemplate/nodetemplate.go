package nodetemplate

import (
	"context"
	"fmt"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	OwnerBindingsAnno = "ownerBindingsCreated"
	vmwaredriver      = "vmwarevsphere"
)

type nodeTemplateController struct {
	ntClient        v3.NodeTemplateInterface
	ntLister        v3.NodeTemplateLister
	npLister        v3.NodePoolLister
	npClient        v3.NodePoolInterface
	nsLister        v1.NamespaceLister
	nsClient        v1.NamespaceInterface
	nodesLister     v3.NodeLister
	nodeClient      v3.NodeInterface
	ntDynamicClient dynamic.NamespaceableResourceInterface
	mgmtCtx         *config.ManagementContext
}

func Register(ctx context.Context, mgmt *config.ManagementContext) {
	// Parts of the node template in k8s are dynamic, such as an azureConfig. If the dynamic client is not use then the
	// dynamic fields, like azureConfig, are stripped when marshalling the data into the NodeTemplate struct. If changes
	// are made, and that struct is committed to k8s, then the dynamic field is effectively removed.
	restConfig := mgmt.RESTConfig
	dynamicClient, err := dynamic.NewForConfig(&restConfig)
	if err != nil {
		panic(fmt.Sprintf("[NodeTemplate Register] error creating dynamic client: %v", err))
	}

	s := schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "nodetemplates",
	}
	ntDynamicClient := dynamicClient.Resource(s)

	nt := nodeTemplateController{
		ntClient:        mgmt.Management.NodeTemplates(""),
		npLister:        mgmt.Management.NodePools("").Controller().Lister(),
		npClient:        mgmt.Management.NodePools(""),
		nsLister:        mgmt.Core.Namespaces("").Controller().Lister(),
		nsClient:        mgmt.Core.Namespaces(""),
		nodesLister:     mgmt.Management.Nodes("").Controller().Lister(),
		nodeClient:      mgmt.Management.Nodes(""),
		ntDynamicClient: ntDynamicClient,
		mgmtCtx:         mgmt,
	}

	mgmt.Management.NodeTemplates("").Controller().AddHandler(ctx, "nt-grb-handler", nt.sync)
}

func (nt *nodeTemplateController) sync(key string, nodeTemplate *v3.NodeTemplate) (runtime.Object, error) {
	if nodeTemplate == nil || nodeTemplate.DeletionTimestamp != nil {
		return nil, nil
	}

	// if owner bindings annotation is present, the node template is in the proper namespace and has had
	// its creator rolebindings created
	if nodeTemplate.Annotations != nil && nodeTemplate.Annotations[OwnerBindingsAnno] == "true" {
		return nodeTemplate, nil
	}

	creatorID, ok := nodeTemplate.Annotations[rbac.CreatorIDAnn]
	if !ok {
		return nodeTemplate, fmt.Errorf("nodeTemplate [%v] has no creatorId annotation", nodeTemplate.Name)
	}

	var migratedTemplate bool

	if nodeTemplate.Namespace != namespace.NodeTemplateGlobalNamespace {
		// node template must be migrated to global namespace
		var err error
		nodeTemplate, err = nt.migrateNodeTemplate(nt.ntDynamicClient, nodeTemplate)
		if err != nil {
			return nil, err
		}

		migratedTemplate = true
	}

	// Create Role and RBs if they do not exist
	if err := rbac.CreateRoleAndRoleBinding(rbac.NodeTemplateResource, v3.NodeTemplateGroupVersionKind.Kind, nodeTemplate.Name, namespace.NodeTemplateGlobalNamespace,
		rbac.RancherManagementAPIVersion, creatorID, []string{rbac.RancherManagementAPIGroup},
		nodeTemplate.UID,
		[]v32.Member{}, nt.mgmtCtx); err != nil {
		return nil, err
	}

	dynamicNodeTemplate, err := nt.ntDynamicClient.Namespace(namespace.NodeTemplateGlobalNamespace).Get(context.TODO(), nodeTemplate.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	annotations := dynamicNodeTemplate.GetAnnotations()
	// owner bindings annotation is meant to prevent bindings from being created again if they have been removed from creator
	annotations[OwnerBindingsAnno] = "true"
	dynamicNodeTemplate.SetAnnotations(annotations)

	if _, err = nt.ntDynamicClient.Namespace(namespace.NodeTemplateGlobalNamespace).Update(context.TODO(), dynamicNodeTemplate, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}

	// if a migrations has been performed, then the original node template object has been deleted
	if migratedTemplate {
		return nil, nil
	}

	return nt.ntClient.GetNamespaced(nodeTemplate.Namespace, nodeTemplate.Name, metav1.GetOptions{})
}

// migrateNodeTemplate creates duplicate of node template in the global node template namespace, creates new role bindings
// for duplicate, then deletes old node template
func (nt *nodeTemplateController) migrateNodeTemplate(ntDynamicClient dynamic.NamespaceableResourceInterface, nodeTemplate *v3.NodeTemplate) (*v3.NodeTemplate, error) {
	logrus.Infof("migrating node template [%s]", nodeTemplate.Spec.DisplayName)

	migratedNTName := fmt.Sprintf("nt-%s-%s", nodeTemplate.Namespace, nodeTemplate.Name)
	fullLegacyNTName := fmt.Sprintf("%s:%s", nodeTemplate.Namespace, nodeTemplate.Name)
	fullMigratedNTName := fmt.Sprintf("%s:%s", namespace.NodeTemplateGlobalNamespace, migratedNTName)

	dynamicNodeTemplate, err := ntDynamicClient.Namespace(nodeTemplate.Namespace).Get(context.TODO(), nodeTemplate.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := nt.createGlobalNodeTemplateClone(nodeTemplate.Name, migratedNTName, dynamicNodeTemplate, ntDynamicClient); err != nil {
		return nil, err
	}

	// update any node pools referencing old node template
	if err := nt.reviseNodePoolNodeTemplate(fullMigratedNTName, fullLegacyNTName); err != nil {
		return nil, err
	}

	// update any nodes referencing old node template
	if err := nt.reviseNodes(fullMigratedNTName, fullLegacyNTName); err != nil {
		return nil, err
	}

	// delete old node template
	if err := nt.ntClient.DeleteNamespaced(nodeTemplate.Namespace, nodeTemplate.Name, &metav1.DeleteOptions{}); err != nil {
		return nil, err
	}

	logrus.Infof("successfully migrated node template [%s]", nodeTemplate.Spec.DisplayName)
	return nt.ntClient.GetNamespaced(namespace.NodeTemplateGlobalNamespace, migratedNTName, metav1.GetOptions{})
}

// reviseNodes searches for nodePools that reference the old node template and replaces it with the new, global template
func (nt *nodeTemplateController) reviseNodePoolNodeTemplate(fullMigratedNTName, fullLegacyNTName string) error {
	npList, err := nt.npLister.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, np := range npList {
		if np.Spec.NodeTemplateName == fullLegacyNTName {
			npCopy := np.DeepCopy()
			npCopy.Spec.NodeTemplateName = fullMigratedNTName

			if _, err := nt.npClient.Update(npCopy); err != nil {
				return err
			}
		}
	}
	return nil
}

// reviseNodes searches for nodes that reference the old node template and replaces it with the new, global template
func (nt *nodeTemplateController) reviseNodes(fullMigratedNTName, fullLegacyNTName string) error {
	nodeList, err := nt.nodesLister.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, node := range nodeList {
		if node.Spec.NodeTemplateName == fullLegacyNTName {
			nodeCopy := node.DeepCopy()
			nodeCopy.Spec.NodeTemplateName = fullMigratedNTName

			if _, err := nt.nodeClient.Update(nodeCopy); err != nil {
				return err
			}
		}
	}
	return nil
}

// createGlobalNodeTemplateClone creates a global clone of the given legacy node template if it does not exist
func (nt *nodeTemplateController) createGlobalNodeTemplateClone(legacyName, cloneName string, dynamicNodeTemplate *unstructured.Unstructured,
	client dynamic.NamespaceableResourceInterface) error {
	if _, err := nt.ntClient.GetNamespaced(namespace.NodeTemplateGlobalNamespace, cloneName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		globalNodeTemplate := dynamicNodeTemplate

		annotations := dynamicNodeTemplate.GetAnnotations()
		labels := dynamicNodeTemplate.GetLabels()
		globalNodeTemplate.Object["metadata"] = map[string]interface{}{
			"name":        cloneName,
			"namespace":   namespace.NodeTemplateGlobalNamespace,
			"annotations": annotations,
			"labels":      labels,
		}

		vsphereLegacyNormalizer(globalNodeTemplate)

		if _, err = client.Namespace(namespace.NodeTemplateGlobalNamespace).Create(context.TODO(), globalNodeTemplate, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

/*
legacy vmwarevsphere nodetemplates were free form text fields
and this is an attempt to normalize the data to a valid vsphere path
e.g. "My Network" becomes "/DC_NAME/networks/My Network"
*/
func vsphereLegacyNormalizer(nt *unstructured.Unstructured) {
	spec, ok := nt.Object["spec"].(map[string]interface{})
	if !ok {
		return
	}

	driver, _ := spec["driver"]
	if driver != vmwaredriver {
		return
	}

	k := vmwaredriver + "Config"
	c, ok := nt.Object[k].(map[string]interface{})
	if !ok {
		return
	}

	dc, _ := c["datacenter"].(string)
	if dc != "" && !strings.HasPrefix(dc, "/") {
		dc = "/" + dc
		c["datacenter"] = dc
	}

	ds, _ := c["datastore"].(string)
	if ds != "" && !strings.HasPrefix(ds, "/") {
		c["datastore"] = fmt.Sprintf("%s/datastore/%s", dc, ds)
	}

	nets, _ := c["network"].([]interface{})
	for i, net := range nets {
		n := net.(string)
		if n != "" && !strings.HasPrefix(n, "/") {
			n = fmt.Sprintf("%s/network/%s", dc, n)
			nets[i] = n
		}
	}
}
