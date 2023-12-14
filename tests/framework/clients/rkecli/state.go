package rkecli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v3 "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/configmaps"
	"github.com/rancher/rancher/tests/framework/pkg/file"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/rancher/rke/cluster"
	rketypes "github.com/rancher/rke/types"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

const filePrefix = "cluster"
const dirName = "rke-cattle-test-dir"

// NewRKEConfigs creates a new dir.
// In that dir, it generates state and cluster files from the state configmap.
// Returns generated state and cluster files' paths.
func NewRKEConfigs(client *rancher.Client) (stateFilePath, clusterFilePath string, err error) {
	err = file.NewDir(dirName)
	if err != nil {
		return
	}

	state, err := GetFullState(client)
	if err != nil {
		return
	}

	clusterFilePath, err = NewClusterFile(state, dirName)
	if err != nil {
		return
	}

	stateFilePath, err = NewStateFile(state, dirName)
	if err != nil {
		return
	}

	return
}

// ReadClusterFromStateFile is a function that reads the RKE config from the given state file path.
// Returns RKE config.
func ReadClusterFromStateFile(stateFilePath string) (*v3.RancherKubernetesEngineConfig, error) {
	byteState, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	//get bytes state
	state := make(map[string]interface{})
	err = json.Unmarshal(byteState, &state)
	if err != nil {
		return nil, err
	}

	//reach rke config in the current state
	byteRkeConfig := state["currentState"].(map[string]interface{})["rkeConfig"]
	byteTest, err := json.Marshal(byteRkeConfig)
	if err != nil {
		return nil, err
	}

	//final unmarshal to get the struct
	rkeConfig := new(v3.RancherKubernetesEngineConfig)
	err = json.Unmarshal(byteTest, rkeConfig)
	if err != nil {
		return nil, err
	}

	return rkeConfig, nil
}

// UpdateKubernetesVersion is a function that updates kubernetes version value in cluster.yml file.
func UpdateKubernetesVersion(kubernetesVersion, clusterFilePath string) error {
	byteRkeConfig, err := os.ReadFile(clusterFilePath)
	if err != nil {
		return err
	}

	rkeConfig := new(rketypes.RancherKubernetesEngineConfig)
	err = yaml.Unmarshal(byteRkeConfig, rkeConfig)
	if err != nil {
		return err
	}

	rkeConfig.Version = kubernetesVersion

	byteConfig, err := yaml.Marshal(rkeConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(clusterFilePath, byteConfig, 0644)
}

// NewClusterFile is a function that generates new cluster.yml file from the full state.
// Returns the generated file's path.
func NewClusterFile(state *cluster.FullState, dirName string) (clusterFilePath string, err error) {
	extension := "yml"
	rkeConfigFileName := fmt.Sprintf("%v/%v.%v", dirName, filePrefix, extension)

	rkeConfig := rketypes.RancherKubernetesEngineConfig{}
	currentRkeConfig := state.CurrentState.RancherKubernetesEngineConfig.DeepCopy()

	rkeConfig.Version = currentRkeConfig.Version
	rkeConfig.Nodes = currentRkeConfig.Nodes

	rkeConfig.SSHKeyPath = appendSSHPath(currentRkeConfig.SSHKeyPath)
	for i := range rkeConfig.Nodes {
		rkeConfig.Nodes[i].SSHKeyPath = appendSSHPath(rkeConfig.Nodes[i].SSHKeyPath)
	}

	marshaled, err := yaml.Marshal(rkeConfig)
	if err != nil {
		return
	}

	fileName := file.Name(rkeConfigFileName)

	clusterFilePath, err = fileName.NewFile(marshaled)
	if err != nil {
		return
	}

	return
}

// NewStateFile is a function that generates new cluster.rkestate file from the full state.
// Returns the generated file's path.
func NewStateFile(state *cluster.FullState, dirName string) (stateFilePath string, err error) {
	extension := "rkestate"
	rkeStateFileName := fmt.Sprintf("%v/%v.%v", dirName, filePrefix, extension)

	marshaled, err := json.Marshal(state)
	if err != nil {
		return
	}

	stateFilePath, err = file.Name(rkeStateFileName).NewFile(marshaled)
	if err != nil {
		return
	}

	return
}

// GetFullState is a function that gets RKE full state from "full-cluster-state" configmap.
// And returns the cluster full state.
func GetFullState(client *rancher.Client) (state *cluster.FullState, err error) {
	namespacedConfigmapClient := client.Steve.SteveType(configmaps.ConfigMapSteveType).NamespacedSteveClient(cluster.SystemNamespace)
	if err != nil {
		return
	}

	configmapResp, err := namespacedConfigmapClient.ByID(cluster.FullStateConfigMapName)
	if err != nil {
		return
	}

	configmap := &corev1.ConfigMap{}
	err = v1.ConvertToK8sType(configmapResp.JSONResp, configmap)
	if err != nil {
		return
	}

	rawState, ok := configmap.Data[cluster.FullStateConfigMapName]
	if !ok {
		err = errors.Wrapf(err, "couldn't retrieve full state data in the configmap")
		return
	}

	rkeFullState := &cluster.FullState{}
	err = json.Unmarshal([]byte(rawState), rkeFullState)
	if err != nil {
		return
	}

	return rkeFullState, nil
}

// appendSSHPath reads sshPath input from the cattle config file.
// If the config input has a different prefix, adds the prefix.
func appendSSHPath(sshPath string) string {
	sshPathPrefix := nodes.GetSSHPath().SSHPath

	if strings.HasPrefix(sshPath, sshPathPrefix) {
		return sshPath
	}

	ssh := ".ssh/"
	sshPath = strings.TrimPrefix(sshPath, ssh)

	return fmt.Sprintf(sshPathPrefix + "/" + sshPath)
}
