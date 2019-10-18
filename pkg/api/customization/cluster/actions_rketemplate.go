package cluster

import (
	"fmt"
	"net/http"

	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) saveAsTemplate(actionName string, action *types.Action, apiContext *types.APIContext) error {

	var clusterForAccessCheck mgmtclient.Cluster
	var err error
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &clusterForAccessCheck); err != nil {
		return httperror.NewAPIError(httperror.NotFound,
			fmt.Sprintf("failed to get cluster by id %v", apiContext.ID))
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound,
			fmt.Sprintf("cluster with id %v doesn't exist", apiContext.ID))
	}

	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType,
			fmt.Sprintf("cluster with id %v is being deleted", apiContext.ID))
	}
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return httperror.WrapAPIError(err, httperror.ClusterUnavailable,
			fmt.Sprintf("cluster not ready"))
	}

	//Copy the cluster rke spec to a template and revision object

	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}
	clusterTempName, ok := actionInput["clusterTemplateName"].(string)
	if !ok || len(clusterTempName) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must specify a name for the RKE Template")
	}
	clusterTempRevName, ok := actionInput["clusterTemplateRevisionName"].(string)
	if !ok || len(clusterTempRevName) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must specify a name for the RKE Template Revision")
	}

	creatorID := apiContext.Request.Header.Get("Impersonate-User")

	newTemplateObj := newClusterTemplate(clusterTempName, cluster, creatorID)
	clusterTemplate, err := a.ClusterTemplateClient.Create(newTemplateObj)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to create RKE Template object"))
	}

	newTemplateRevObj := newClusterTemplateRevision(clusterTempRevName, cluster, clusterTemplate, creatorID)
	clusterTemplateRevision, err := a.ClusterTemplateRevisionClient.Create(newTemplateRevObj)
	if err != nil {
		//delete the clusterTemplate created above
		deleteErr := a.ClusterTemplateClient.Delete(clusterTemplate.Name, &metav1.DeleteOptions{})
		if deleteErr != nil {
			logrus.Errorf("Failed to delete the RKE Template %v", clusterTemplate.Name)
		}

		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to create RKE Template Revision object"))
	}

	updatedCluster := cluster.DeepCopy()
	updatedCluster.Spec.ClusterTemplateRevisionName = ref.Ref(clusterTemplateRevision)
	updatedCluster.Spec.ClusterTemplateName = ref.Ref(clusterTemplate)

	// Can't add either too many retries or longer interval as this an API handler
	for i := 0; i < NumberOfRetriesForClusterUpdate; i++ {
		_, err = a.ClusterClient.Update(updatedCluster)
		if err == nil {
			break
		}
		time.Sleep(RetryIntervalInMilliseconds * time.Millisecond)
		cluster, err = a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
		if err != nil {
			logrus.Errorf("error fetching cluster with id %v: %v", apiContext.ID, err)
			continue
		}
		updatedCluster = cluster.DeepCopy()
		updatedCluster.Spec.ClusterTemplateRevisionName = ref.Ref(clusterTemplateRevision)
		updatedCluster.Spec.ClusterTemplateName = ref.Ref(clusterTemplate)
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func newClusterTemplate(clusterTempName string, cluster *v3.Cluster, creatorID string) *v3.ClusterTemplate {
	return &v3.ClusterTemplate{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "ct-",
			Namespace:    namespace.GlobalNamespace,
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
			Annotations: map[string]string{
				globalnamespacerbac.CreatorIDAnn: creatorID,
			},
		},
		Spec: v3.ClusterTemplateSpec{
			DisplayName: clusterTempName,
			Description: fmt.Sprintf("RKETemplate generated from cluster Id %v", cluster.Name),
		},
	}
}

func newClusterTemplateRevision(clusterTempRevisionName string, cluster *v3.Cluster, clusterTemplate *v3.ClusterTemplate, creatorID string) *v3.ClusterTemplateRevision {
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
				globalnamespacerbac.CreatorIDAnn: creatorID,
			},
		},
		Spec: v3.ClusterTemplateRevisionSpec{
			DisplayName:         clusterTempRevisionName,
			ClusterTemplateName: ref.Ref(clusterTemplate),
			ClusterConfig:       clusterConfig,
		},
	}
}
