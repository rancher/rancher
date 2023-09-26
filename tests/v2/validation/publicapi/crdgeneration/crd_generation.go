package crdgeneration

import (
	"os"
	"strings"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/customresourcedefinitions"
	"github.com/rancher/rancher/tests/framework/extensions/kubectl"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	groupManagement      = "management.cattle.io"
	groupCatalog         = "catalog.cattle.io"
	groupUI              = "ui.cattle.io"
	groupCluster         = "cluster.cattle.io"
	groupProject         = "project.cattle.io"
	groupRKE             = "rke.cattle.io"
	groupProvisioning    = "provisioning.cattle.io"
	groupFleet           = "fleet.cattle.io"
	roleTemplate         = "roletemplates.management.cattle.io" //Remove in 2.8.0
	fleetlocal           = "fleet-local"
	fleetdefault         = "fleet-default"
	crdJSONFilePath      = "../resources/crds.json"
	roleTemplateJSONPath = "../resources/roleTemplate.json"
)

func mapCRD(crdList []string) map[string][]string {
	groups := []string{groupManagement, groupCatalog, groupUI, groupCluster, groupProject, groupRKE, groupProvisioning, groupFleet}
	crdMap := make(map[string][]string)

	for _, group := range groups {
		crdMap[group] = []string{}
	}

	crdMap["additional"] = []string{}

	for _, crd := range crdList {
		found := false
		for _, group := range groups {
			if strings.Contains(crd, group) {
				crdMap[group] = append(crdMap[group], crd)
				found = true
				break
			}
		}
		if !found {
			crdMap["additional"] = append(crdMap["additional"], crd)
		}
	}

	return crdMap
}

func listCRDS(client *rancher.Client, clusterID string) ([]string, error) {
	crdCollection, err := customresourcedefinitions.ListCustomResourceDefinitions(client, clusterID, "")
	if err != nil {
		return nil, err
	}

	crds := make([]string, len(crdCollection.Items))
	for idx, crd := range crdCollection.Items {
		crds[idx] = crd.GetName()
	}
	return crds, nil
}

func validateCRDList(t *testing.T, crdsList []string, crdMapPreUpgrade map[string][]string, clusterName string) {
	crdMap := mapCRD(crdsList)

	if clusterName == "local" {
		for group, value := range crdMap {
			if group == "additional" || group == groupCluster {
				continue
			}
			assert.Equal(t, crdMapPreUpgrade[group], value)
			assert.Equal(t, len(crdMapPreUpgrade[group]), len(value))
		}
	}
}

func validateCRDDescription(t *testing.T, client *rancher.Client, clusterV1 *v1.Cluster, clusterID string) {

	logs, err := kubectl.Explain(client, clusterV1, roleTemplate, clusterID)
	require.NoError(t, err)
	assert.NotEmpty(t, logs)
	desIndexStart := strings.Index(logs, "DESCRIPTION:")
	desIndexEnd := strings.Index(logs, "FIELDS")
	if desIndexStart == -1 {
		t.Log("Pod logs do not contain description field")
		t.FailNow()
	}
	if desIndexEnd == -1 {
		desIndexEnd = len(logs) - 1
	}

	description := logs[desIndexStart+len("DESCRIPTION:") : desIndexEnd]
	assert.NotContains(t, description, "<empty>")
	log.Info(description)
}

func validateRoleCreation(t *testing.T, client *rancher.Client, clusterV1 *v1.Cluster, clusterID string) {
	role, err := os.ReadFile(roleTemplateJSONPath)
	require.NoError(t, err)

	yamlInput := &management.ImportClusterYamlInput{
		YAML: string(role),
	}

	podLogs, err := kubectl.Apply(client, clusterV1, yamlInput, clusterID)
	require.NoError(t, err)
	errorLogs := "Unsupported value: \"invalid\": supported values: \"project\", \"cluster\""

	assert.Contains(t, podLogs, errorLogs)
}
