package cluster

import (
	"fmt"

	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

// Create is a no-op required by the ClusterLifecycle interface
func (c *controller) Create(obj *v3.Cluster) (runtime.Object, error) {
	return obj, nil
}

// Updated is a no-op required by the ClusterLifecycle interface
func (c *controller) Updated(obj *v3.Cluster) (runtime.Object, error) {
	return obj, nil
}

// Remove handles the cleanup of Harvester credentials when the cluster is deleted
func (c *controller) Remove(obj *v3.Cluster) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	// Only attempt cleanup if this is a Harvester-related cluster
	if obj.Status.Driver != "harvester" {
		return obj, nil
	}

	if err := c.cleanupHarvesterCloudCredentials(obj); err != nil {
		// Log error but don't block deletion indefinitely if credential cleanup fails
		logrus.Warnf("Failed to cleanup Harvester credentials for cluster %s: %v", obj.Name, err)
		return obj, nil
	}
	return obj, nil
}

func (c *controller) cleanupHarvesterCloudCredentials(cluster *v3.Cluster) error {
	creds, err := c.cloudCredLister.List("", labels.Everything())
	if err != nil {
		return err
	}

	for _, cred := range creds {
		if !isHarvesterCredentialForCluster(cred, cluster.Name) {
			continue
		}

		logrus.Infof(
			"Deleting Harvester cloud credential %s associated with removed cluster %s",
			cred.Name,
			cluster.Name,
		)

		if err := c.cloudCredClient.Delete(cred.Name, &metav1.DeleteOptions{}); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}
	}

	return nil
}

func isHarvesterCredentialForCluster(cred *v3.CloudCredential, clusterID string) bool {
	credMap, err := convert.EncodeToMap(cred)
	if err != nil {
		return false
	}

	// Helper to extract clusterId string from map
	getClusterID := func(data map[string]interface{}) string {
		if val, ok := data["clusterId"]; ok {
			return fmt.Sprintf("%v", val)
		}
		return ""
	}

	// Look for 'harvestercredentialConfig' key
	if configVal, ok := credMap["harvestercredentialConfig"]; ok {
		if configMap, ok := configVal.(map[string]interface{}); ok {
			if foundID := getClusterID(configMap); foundID == clusterID {
				return true
			}
		}
	}

	return false
}
