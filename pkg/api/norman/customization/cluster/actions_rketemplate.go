package cluster

import (
	"fmt"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	numberOfRetries   = 3
	retryIntervalInMs = 5
)

func (a ActionHandler) saveAsTemplate(actionName string, action *types.Action, apiContext *types.APIContext) error {

	var err error
	var clusterTemplate *v3.ClusterTemplate
	var clusterTemplateRevision *v3.ClusterTemplateRevision

	defer a.cleanup(err, clusterTemplate)

	clusterTempName, clusterTempRevName, err := a.validateTemplateInput(apiContext)
	if err != nil {
		return err
	}

	cluster, err := a.validateClusterState(apiContext)
	if err != nil {
		return err
	}

	//Copy the cluster rke spec to a template and revision object
	clusterTemplate, err = a.createNewClusterTemplate(apiContext, clusterTempName, cluster)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to create RKE Template object"))
	}

	clusterTemplateRevision, err = a.createNewClusterTemplateRevision(apiContext, clusterTempRevName, clusterTemplate, cluster)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to create RKE Template Revision object"))
	}

	err = a.updateCluster(apiContext, clusterTemplate, clusterTemplateRevision)
	if err != nil {
		return err
	}

	response := map[string]interface{}{
		"clusterTemplateName":         ref.Ref(clusterTemplate),
		"clusterTemplateRevisionName": ref.Ref(clusterTemplateRevision),
		"type":                        "saveAsTemplateOutput",
	}

	apiContext.WriteResponse(http.StatusOK, response)
	return nil
}

func (a ActionHandler) validateTemplateInput(apiContext *types.APIContext) (string, string, error) {
	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return "", "", err
	}
	clusterTempName, ok := actionInput["clusterTemplateName"].(string)
	if !ok || len(clusterTempName) == 0 {
		return "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "must specify a name for the RKE Template")
	}
	clusterTempRevName, ok := actionInput["clusterTemplateRevisionName"].(string)
	if !ok || len(clusterTempRevName) == 0 {
		return "", "", httperror.NewAPIError(httperror.InvalidBodyContent, "must specify a name for the RKE Template Revision")
	}
	return clusterTempName, clusterTempRevName, nil
}

func (a ActionHandler) validateClusterState(apiContext *types.APIContext) (*v3.Cluster, error) {
	var clusterForAccessCheck mgmtclient.Cluster
	var err error

	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &clusterForAccessCheck); err != nil {
		return nil, httperror.NewAPIError(httperror.NotFound,
			fmt.Sprintf("failed to get cluster by id %v", apiContext.ID))
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return nil, httperror.WrapAPIError(err, httperror.NotFound,
			fmt.Sprintf("cluster with id %v doesn't exist", apiContext.ID))
	}

	if cluster.DeletionTimestamp != nil {
		return nil, httperror.NewAPIError(httperror.InvalidType,
			fmt.Sprintf("cluster with id %v is being deleted", apiContext.ID))
	}
	/*if !v3.ClusterConditionReady.IsTrue(cluster) {
		return nil, httperror.WrapAPIError(err, httperror.ClusterUnavailable,
			fmt.Sprintf("cluster with id %v is not ready", apiContext.ID))
	}*/
	return cluster, nil
}

func (a ActionHandler) createNewClusterTemplate(apiContext *types.APIContext, clusterTempName string, cluster *v3.Cluster) (*v3.ClusterTemplate, error) {
	creatorID := apiContext.Request.Header.Get("Impersonate-User")
	newTemplateObj := a.newClusterTemplate(clusterTempName, cluster, creatorID)
	return a.ClusterTemplateClient.Create(newTemplateObj)
}

func (a ActionHandler) newClusterTemplate(clusterTempName string, cluster *v3.Cluster, creatorID string) *v3.ClusterTemplate {
	return &v3.ClusterTemplate{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "ct-",
			Namespace:    namespace.GlobalNamespace,
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
			Annotations: map[string]string{
				rbac.CreatorIDAnn: creatorID,
			},
		},
		Spec: v32.ClusterTemplateSpec{
			DisplayName: clusterTempName,
			Description: fmt.Sprintf("RKETemplate generated from cluster Id %v", cluster.Name),
		},
	}
}

func (a ActionHandler) createNewClusterTemplateRevision(apiContext *types.APIContext, clusterTempRevName string, clusterTemplate *v3.ClusterTemplate, cluster *v3.Cluster) (*v3.ClusterTemplateRevision, error) {
	creatorID := apiContext.Request.Header.Get("Impersonate-User")
	newTemplateRevObj := a.newClusterTemplateRevision(clusterTempRevName, cluster, clusterTemplate, creatorID)
	return a.ClusterTemplateRevisionClient.Create(newTemplateRevObj)
}

func (a ActionHandler) newClusterTemplateRevision(clusterTempRevisionName string, cluster *v3.Cluster, clusterTemplate *v3.ClusterTemplate, creatorID string) *v3.ClusterTemplateRevision {
	controller := true
	clusterConfig := cluster.Status.AppliedSpec.ClusterSpecBase.DeepCopy()

	return &v3.ClusterTemplateRevision{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "ctr-",
			Namespace:    namespace.GlobalNamespace,
			OwnerReferences: []v1.OwnerReference{
				{
					Name:       clusterTemplate.Name,
					UID:        clusterTemplate.UID,
					APIVersion: "management.cattle.io/v3",
					Kind:       "ClusterTemplate",
					Controller: &controller,
				},
			},
			Labels: map[string]string{
				"cattle.io/creator":                 "norman",
				"io.cattle.field/clusterTemplateId": clusterTemplate.Name,
			},
			Annotations: map[string]string{
				rbac.CreatorIDAnn: creatorID,
			},
		},
		Spec: v32.ClusterTemplateRevisionSpec{
			DisplayName:         clusterTempRevisionName,
			ClusterTemplateName: ref.Ref(clusterTemplate),
			ClusterConfig:       clusterConfig,
		},
	}
}

func (a ActionHandler) updateCluster(apiContext *types.APIContext, clusterTemplate *v3.ClusterTemplate, clusterTemplateRevision *v3.ClusterTemplateRevision) error {
	// Can't add either too many retries or longer interval as this an API handler
	for i := 0; i < numberOfRetries; i++ {
		cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
		if err != nil {
			logrus.Errorf("error fetching cluster with id %v: %v", apiContext.ID, err)
			continue
		}
		updatedCluster := cluster.DeepCopy()
		updatedCluster.Spec.ClusterTemplateRevisionName = ref.Ref(clusterTemplateRevision)
		updatedCluster.Spec.ClusterTemplateName = ref.Ref(clusterTemplate)

		_, err = a.ClusterClient.Update(updatedCluster)
		if err == nil {
			return nil
		}
		time.Sleep(retryIntervalInMs * time.Millisecond)
	}
	return httperror.NewAPIError(httperror.Conflict,
		fmt.Sprintf("Error while updating cluster %v with RKE Template Id %v and RKE TemplateRevision Id %v", apiContext.ID, clusterTemplate.Name, clusterTemplateRevision.Name))
}

func (a ActionHandler) cleanup(err error, clusterTemplate *v3.ClusterTemplate) {
	if err == nil {
		return
	}
	if clusterTemplate != nil {
		for i := 0; i < numberOfRetries; i++ {
			//delete the clusterTemplate created, any revision will be deleted due to owner-ref
			deleteErr := a.ClusterTemplateClient.Delete(clusterTemplate.Name, &metav1.DeleteOptions{})
			if deleteErr != nil {
				logrus.Errorf("Failed to delete the RKE Template %v", clusterTemplate.Name)
				continue
			}
			time.Sleep(retryIntervalInMs * time.Millisecond)
		}
	}
}
