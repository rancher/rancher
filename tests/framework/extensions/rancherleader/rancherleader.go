package rancherleader

import (
	"encoding/json"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/configmaps"
	"github.com/rancher/rancher/tests/framework/extensions/constants"
)

// GetRancherLeaderPod is a helper function to retrieve the name of the rancher leader pod
func GetRancherLeaderPod(client *rancher.Client) (string, error) {
	configMapList, err := client.Steve.SteveType(configmaps.ConfigMapSteveType).NamespacedSteveClient(constants.KubeSystemNamespace).List(nil)
	if err != nil {
		return "", err
	}

	var leaderAnnotation string
	for _, cm := range configMapList.Data {
		if cm.Name == constants.RancherConfigMap {
			leaderAnnotation = cm.Annotations[constants.RancherLeaderAnnotation]
			break
		}
	}

	var leaderRecord map[string]interface{}
	err = json.Unmarshal([]byte(leaderAnnotation), &leaderRecord)
	if err != nil {
		return "", err
	}

	leaderPodName := leaderRecord["holderIdentity"].(string)

	return leaderPodName, nil
}
