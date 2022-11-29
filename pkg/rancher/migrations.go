package rancher

import (
	"github.com/rancher/norman/condition"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rancherversion "github.com/rancher/rancher/pkg/version"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	cattleNamespace                           = "cattle-system"
	forceLocalSystemAndDefaultProjectCreation = "forcelocalprojectcreation"
	forceSystemNamespacesAssignment           = "forcesystemnamespaceassignment"
	projectsCreatedKey                        = "projectsCreated"
	namespacesAssignedKey                     = "namespacesAssigned"
)

func getConfigMap(configMapController controllerv1.ConfigMapController, configMapName string) (*v1.ConfigMap, error) {
	cm, err := configMapController.Cache().Get(cattleNamespace, configMapName)
	if err != nil && !k8serror.IsNotFound(err) {
		return nil, err
	}

	// if this is the first ever migration initialize the configmap
	if cm == nil {
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: cattleNamespace,
			},
			Data: make(map[string]string, 1),
		}
	}

	// we do not migrate in development environments
	if rancherversion.Version == "dev" {
		return nil, nil
	}

	return cm, nil
}

func createOrUpdateConfigMap(configMapClient controllerv1.ConfigMapClient, cm *v1.ConfigMap) error {
	var err error
	if cm.ObjectMeta.GetResourceVersion() != "" {
		_, err = configMapClient.Update(cm)
	} else {
		_, err = configMapClient.Create(cm)
	}

	return err
}

// forceSystemAndDefaultProjectCreation will set the corresponding conditions on the local cluster object,
// if it exists, to Unknown. This will force the corresponding controller to check that the projects exist
// and create them, if necessary.
func forceSystemAndDefaultProjectCreation(configMapController controllerv1.ConfigMapController, clusterClient v3.ClusterClient) error {
	cm, err := getConfigMap(configMapController, forceLocalSystemAndDefaultProjectCreation)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[projectsCreatedKey] == "true" {
		return nil
	}

	localCluster, err := clusterClient.Get("local", metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			return nil
		}
		return err
	}

	v32.ClusterConditionDefaultProjectCreated.Unknown(localCluster)
	v32.ClusterConditionSystemProjectCreated.Unknown(localCluster)

	_, err = clusterClient.Update(localCluster)
	if err != nil {
		return err
	}

	cm.Data[projectsCreatedKey] = "true"
	return createOrUpdateConfigMap(configMapController, cm)
}

func forceSystemNamespaceAssignment(configMapController controllerv1.ConfigMapController, projectClient v3.ProjectClient) error {
	cm, err := getConfigMap(configMapController, forceSystemNamespacesAssignment)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[namespacesAssignedKey] == rancherversion.Version {
		return nil
	}

	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/system-project=true", v32.ProjectConditionSystemNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}
	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/default-project=true", v32.ProjectConditionDefaultNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}

	cm.Data[namespacesAssignedKey] = rancherversion.Version
	return createOrUpdateConfigMap(configMapController, cm)
}

func applyProjectConditionForNamespaceAssignment(label string, condition condition.Cond, projectClient v3.ProjectClient) error {
	projects, err := projectClient.List("", metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return err
	}

	for i := range projects.Items {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			p := &projects.Items[i]
			p, err = projectClient.Get(p.Namespace, p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			condition.Unknown(p)
			_, err = projectClient.Update(p)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
