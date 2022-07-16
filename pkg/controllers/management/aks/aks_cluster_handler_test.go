package aks

import (
	"testing"

	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
)

func Test_onClusterChange(t *testing.T) {

	mockOperatorController := getMockOperatorController()
	mockOperatorController.onClusterChange(string key, v3mgmt.cluster object)

	t.Error()
}

func Test_setInitialUpstreamSpec(t *testing.T) {
	t.Error()
}

func Test_updateAKSClusterConfig(t *testing.T) {
	t.Error()
}

func Test_generateAndSetServiceAccount(t *testing.T) {
	t.Error()
}

func Test_buildAKSCCCreateObject(t *testing.T) {
	t.Error()
}

func Test_recordAppliedSpec(t *testing.T) {
	t.Error()
}

func Test_generateSATokenWithPublicAPI(t *testing.T) {
	t.Error()
}

func Test_getRestConfig(t *testing.T) {
	t.Error()
}

// utility

func getMockOperatorController() aksOperatorController {

	mockAksOperatorController := &aksOperatorController{
		OperatorController: clusteroperator.OperatorController{
			ClusterEnqueueAfter:  nil,
			SecretsCache:         nil,
			Secrets:              nil,
			TemplateCache:        nil,
			ProjectCache:         nil,
			AppLister:            nil,
			AppClient:            nil,
			NsClient:             nil,
			ClusterClient:        nil,
			CatalogManager:       nil,
			SystemAccountManager: nil,
			DynamicClient:        nil,
			ClientDialer:         nil,
			Discovery:            nil,
		},
		secretClient: nil,
	}

	return *mockAksOperatorController
}




