package namespace

import (
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	client "github.com/rancher/types/client/cluster/v3"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/cache"
)

var (
	projectIDFieldLabel = "field.cattle.io/projectId"
	namespaceOwnerMap   = cache.NewLRUExpireCache(1000)
)

func updateNamespaceOwnerMap(apiContext *types.APIContext) error {
	var namespaces []client.Namespace
	if err := access.List(apiContext, &schema.Version, client.NamespaceType, &types.QueryOptions{}, &namespaces); err != nil {
		return err
	}

	for _, namespace := range namespaces {
		namespaceOwnerMap.Add(namespace.Name, namespace.ProjectID, time.Hour)
	}

	return nil
}

func ProjectMap(apiContext *types.APIContext, refresh bool) (map[string]string, error) {
	if refresh {
		err := updateNamespaceOwnerMap(apiContext)
		if err != nil {
			return nil, err
		}
	}

	data := map[string]string{}
	for _, key := range namespaceOwnerMap.Keys() {
		if val, ok := namespaceOwnerMap.Get(key); ok {
			data[key.(string)] = val.(string)
		}
	}

	return data, nil
}

type ActionWrapper struct {
	ClusterManager *clustermanager.Manager
}

func (w ActionWrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}

	if !canUpdateNS(apiContext) {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	switch actionName {
	case "move":
		clusterID := w.ClusterManager.ClusterName(apiContext)
		_, projectID := ref.Parse(convert.ToString(actionInput["projectId"]))
		userContext, err := w.ClusterManager.UserContext(clusterID)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}
			return httperror.NewAPIError(httperror.NotFound, err.Error())
		}
		if projectID != "" {
			project, err := userContext.Management.Management.Projects(clusterID).Get(projectID, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if project.Spec.ResourceQuota != nil {
				return errors.Errorf("can't move namespace. Project %s has resource quota set", project.Spec.DisplayName)
			}
		}
		nsClient := userContext.Core.Namespaces("")
		ns, err := nsClient.Get(apiContext.ID, metav1.GetOptions{})
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}
			return httperror.NewAPIError(httperror.NotFound, err.Error())
		}
		if ns.Annotations[helm.AppIDsLabel] != "" {
			return errors.New("namespace is currently being used")
		}
		if projectID == "" {
			delete(ns.Annotations, projectIDFieldLabel)
		} else {
			ns.Annotations[projectIDFieldLabel] = convert.ToString(actionInput["projectId"])
		}
		if _, err := nsClient.Update(ns); err != nil {
			return err
		}
	default:
		return errors.New("invalid action")
	}
	return nil
}

func NewFormatter(next types.Formatter) types.Formatter {
	return func(request *types.APIContext, resource *types.RawResource) {
		if next != nil {
			next(request, resource)
		}
		annotations := convert.ToMapInterface(resource.Values["annotations"])

		if canUpdate := canUpdateNS(request); canUpdate && convert.ToString(annotations[helm.AppIDsLabel]) == "" {
			resource.AddAction(request, "move")
		}
	}
}

func canUpdateNS(apiContext *types.APIContext) bool {
	nsObj := map[string]interface{}{
		"id": apiContext.ID,
	}
	// note that the user must have * permissions on namespace, the create-ns role alone won't return true here
	return apiContext.AccessControl.CanDo("", "namespaces", "update", apiContext, nsObj, apiContext.Schema) == nil
}
