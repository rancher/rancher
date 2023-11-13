package globalroles

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiVersion = "management.cattle.io/v3"
	kind       = "GlobalRole"
)

func kubectlApplyOrDelete(t *testing.T, client *rancher.Client, clusterV1 *v1.Cluster, roleStruct interface{}, clusterID, operation, expectedLog string) {
	roleJSON, err := json.Marshal(roleStruct)
	require.NoError(t, err)

	yamlInput := &management.ImportClusterYamlInput{
		YAML: string(roleJSON),
	}

	command := []string{"kubectl", "", "-f", "/root/.kube/my-pod.yaml"}
	command[1] = operation
	podLogs, err := kubectl.Command(client, clusterV1, yamlInput, clusterID, command)
	require.NoError(t, err)

	assert.Contains(t, podLogs, expectedLog)
}

func deleteGlobalRole(t *testing.T, client *rancher.Client, clusterV1 *v1.Cluster, clusterID, roleName, expectedLog string) {

	command := []string{"kubectl", "delete", "globalrole", roleName}
	podLogs, err := kubectl.Command(client, clusterV1, nil, clusterID, command)
	require.NoError(t, err)

	assert.Contains(t, podLogs, expectedLog)
}

func readGlobalRole(t *testing.T, client *rancher.Client, clusterV1 *v1.Cluster, clusterID, roleName, expectedLog string) {

	command := []string{"kubectl", "get", "globalrole", roleName}
	podLogs, err := kubectl.Command(client, clusterV1, nil, clusterID, command)
	require.NoError(t, err)

	assert.Contains(t, podLogs, expectedLog)
}

func setupGlobalRole(name string, builtIn bool, inheritedRoles []string) *v3.GlobalRole {
	globalRole := v3.GlobalRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		DisplayName:           name,
		Rules:                 nil,
		NewUserDefault:        false,
		Builtin:               builtIn,
		InheritedClusterRoles: inheritedRoles,
	}
	return &globalRole
}
