package aks

import (
	"os"
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const (
	MockDefaultClusterFilename = "./test/onclusterchange_default.yaml"
	MockCreateClusterFilename = "./test/onclusterchange_create.yaml"
	MockActiveClusterFilename = "./test/onclusterchange_active.yaml"
	MockUpdateClusterFilename = "./test/onclusterchange_update.yaml"

	MockAKSClusterConfigFilename = "./test/updateaksclusterconfig.json"
	MockAKSClusterConfigClusterFilename = "./test/updateaksclusterconfig_cluster.yaml"

	MockBuildAKSCCCreateObjectFilename = "./test/buildakscccreateobject.json"
)


var mockAksOperatorController aksOperatorController

// test setup

func TestMain(m *testing.M) {
	// Code to run before the tests

	// Run tests
	exitVal := m.Run()

	// Code to run after the tests

	// Exit with exit value from tests
	os.Exit(exitVal)
}

/** Test_onClusterChange
	- cluster == nil. Return (nil nil)
	- cluster.DeletionTimestamp or cluster.AKSConfig == nil, return (nil nil)
	- default phase
	- create phase
	- active phase
	- update node pool phase
 */
func Test_onClusterChange_ClusterIsNil(t *testing.T) {
	cluster, _ := mockAksOperatorController.onClusterChange("", nil)
	if cluster != nil {
		t.Error("cluster should have returned nil")
	}
}

func Test_onClusterChange_AKSConfigIsNil(t *testing.T) {
	mockCluster := &v3.Cluster{
		Spec: v3.ClusterSpec{
			AKSConfig: nil,
		},
	}

	cluster, _ := mockAksOperatorController.onClusterChange("", mockCluster)
	if cluster != mockCluster {
		t.Error("cluster should have returned with no update")
	}
}

func Test_onClusterChange_Default(t *testing.T) {

	// setup
	// create an instance of the operator controller with mock data to simulate the onChangeCluster function reacting
	// to a real cluster!
	mockAksOperatorController = getMockAksOperatorController("default")

	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	// run test
	cluster, err := mockAksOperatorController.onClusterChange("", &mockCluster)

	// validate results
	if cluster.Status.Conditions[1].Status != "Unknown" || err != nil {
		t.Error("status should be Unknown and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_Create(t *testing.T) {
	mockAksOperatorController = getMockAksOperatorController("create")
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockAksOperatorController.onClusterChange("", &mockCluster)

	if cluster.Status.Conditions[1].Status != "Unknown" || err != nil {
		t.Error("provisioned status should be Unknown and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_Active(t *testing.T) {
	mockAksOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockAksOperatorController.onClusterChange("", &mockCluster)

	if cluster.Status.Conditions[1].Status != "True" ||
		cluster.Status.Conditions[14].Status != "True" || err != nil {
		t.Error("provisioned and updated status should be True and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_UpdateNodePool(t *testing.T) {
	mockAksOperatorController = getMockAksOperatorController("update")
	mockCluster, err := getMockV3Cluster(MockUpdateClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockAksOperatorController.onClusterChange("", &mockCluster)

	// check that cluster is updating node pool
	if cluster.Status.Conditions[1].Status != "True" ||
		cluster.Status.Conditions[14].Status != "Unknown" || err != nil {
		t.Error("provisioned status should be True, updated status should be Unknown and cluster returned successfully: " + err.Error())
	}
}


/** Test_setInitialUpstreamSpec
	- success
	- buildUpstreamClusterState returns an error
 */
func Test_setInitialUpstreamSpec(t *testing.T) {
	t.Error("not implemented")
}


/** Test_updateAKSClusterConfig
	- success: AKS cluster tags are removed. AKS cluster is not immediately updated. Cluster sits in active for a few
      seconds, return (cluster nil)
 */
func Test_updateAKSClusterConfig(t *testing.T) {
	mockAksOperatorController = getMockAksOperatorController("akscc")
	mockCluster, err := getMockV3Cluster(MockAKSClusterConfigClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}
	mockAksClusterConfig, err := getMockAKSClusterConfig(MockAKSClusterConfigFilename)

	// test remove tags from the cluster
	_, err = mockAksOperatorController.updateAKSClusterConfig(&mockCluster, mockAksClusterConfig, nil)

	if err != nil {
		t.Error("AKSClusterConfig should have been updated and cluster returned successfully: " + err.Error())
	}
}


/** Test_generateAndSetServiceAccount
	- error generating service account token. Return (cluster, err)
	- success: service account token generated, cluster updated! Return updated cluster.Status
 */
func Test_generateAndSetServiceAccount(t *testing.T) {
	//mockAksOperatorController = getMockAksOperatorController("active")
	//mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	//if err != nil {
	//	t.Error("error getting mock v3 cluster: ", err.Error())
	//}
	//
	//// set service account secret and token on cluster.Status
	//expected := mockCluster.DeepCopy()
	//expected.Status.ServiceAccountTokenSecret = ""
	//expected.Status.ServiceAccountToken = ""
	//
	//cluster, err := mockAksOperatorController.generateAndSetServiceAccount(&mockCluster)
	//if cluster != expected {
	//	t.Error("service account token and secret should have been set on Status and cluster returned successfully")
	//}

	t.Error("To do: mock sibling functions")
}


/** Test_buildAKSCCCreateObject
	- success: AKSClusterConfig object created, return (AKSClusterConfig nil)
 */
func Test_buildAKSCCCreateObject(t *testing.T) {
	mockAksOperatorController = getMockAksOperatorController("default")
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}
	expected, err := getMockAKSClusterConfig(MockBuildAKSCCCreateObjectFilename)

	akscc, err := buildAKSCCCreateObject(&mockCluster)

	if !reflect.DeepEqual(akscc, expected) || err != nil {
		t.Error("AKS cluster config object should have been returned successfully: " + err.Error())
	}
}


/** Test_recordAppliedSpec
	- success: set current spec as applied spec. Return (updated cluster err)
	- success: AKSConfig and Applied Spec AKSConfig are equal. Return (cluster nil)
 */
func Test_recordAppliedSpec_Updated(t *testing.T) {
	// We use a mock cluster that is still provisioning and in an Unknown state, because that is when the applied spec
	// needs to be updated.
	mockAksOperatorController = getMockAksOperatorController("default")
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockAksOperatorController.recordAppliedSpec(&mockCluster)
	if cluster.Status.AppliedSpec.AKSConfig == nil || err != nil {
		t.Error("cluster Status.AppliedSpec should have been updated with AKSConfig: " + err.Error())
	}
}

func Test_recordAppliedSpec_NoUpdate(t *testing.T) {
	// A mock active cluster already has the AKSConfig set on the applied spec, so no update is required.
	mockAksOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockAksOperatorController.recordAppliedSpec(&mockCluster)
	if cluster != &mockCluster || err != nil {
		t.Error("cluster should have successfully returned with no applied spec update " + err.Error())
	}
}


/** Test_generateSATokenWithPublicAPI
	  PRIVATE CLUSTER ONLY
	- success in getting a service account token from the public API endpoint. Return (token mustTunnel=false nil)
	- failure to get service account token. Return ("" mustTunnel=true err)
	- unknown error. Return ("" mustTunnel=nil err)
 */
func Test_generateSATokenWithPublicAPI(t *testing.T) {
	t.Error("not implemented")
}


/** Test_getRestConfig
	- success: return (restConfig nil)
	- error getting kube config. Return (nil err)
	- error getting CAcert from the cluster. Return (nil err)
 */
func Test_getRestConfig(t *testing.T) {
	t.Error("not implemented")
}