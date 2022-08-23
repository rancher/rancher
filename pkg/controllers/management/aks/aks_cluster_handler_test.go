package aks

import (
	"reflect"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const (
	MockDefaultClusterFilename = "./test/onclusterchange_default.yaml"
	MockCreateClusterFilename = "./test/onclusterchange_create.yaml"
	MockActiveClusterFilename = "./test/onclusterchange_active.yaml"
	MockUpdateClusterFilename = "./test/onclusterchange_update.yaml"
	MockAKSClusterConfigFilename = "./test/updateaksclusterconfig.json"
	MockAKSClusterConfigClusterFilename = "./test/updateaksclusterconfig.yaml"
	MockBuildAKSCCCreateObjectFilename = "./test/buildakscccreateobject.json"
)


var mockOperatorController mockAksOperatorController	// Operator controller with mock interfaces & sibling funcs


/** Test_onClusterChange
	- cluster == nil. Return (nil nil)
	- cluster.DeletionTimestamp or cluster.AKSConfig == nil, return (nil nil)
	- default phase
	- create phase
	- active phase
	- update node pool phase
 */
func Test_onClusterChange_ClusterIsNil(t *testing.T) {
	cluster, _ := mockOperatorController.onClusterChange("", nil)
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

	cluster, _ := mockOperatorController.onClusterChange("", mockCluster)
	if cluster != mockCluster {
		t.Error("cluster should have returned with no update")
	}
}

func Test_onClusterChange_Default(t *testing.T) {

	// setup
	// create an instance of the operator controller with mock data to simulate the onChangeCluster function reacting
	// to a real cluster!
	mockOperatorController = getMockAksOperatorController("default")

	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	// run test
	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// validate results
	if cluster.Status.Conditions[1].Status != "Unknown" || err != nil {
		t.Error("status should be Unknown and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_Create(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("create")
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if cluster.Status.Conditions[1].Status != "Unknown" || err != nil {
		t.Error("provisioned status should be Unknown and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_Active(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if cluster.Status.Conditions[1].Status != "True" ||
		cluster.Status.Conditions[14].Status != "True" || err != nil {
		t.Error("provisioned and updated status should be True and cluster returned successfully: " + err.Error())
	}
}

func Test_onClusterChange_UpdateNodePool(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("update")
	mockCluster, err := getMockV3Cluster(MockUpdateClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// check that cluster is updating node pool
	if cluster.Status.Conditions[1].Status != "True" ||
		cluster.Status.Conditions[14].Status != "Unknown" || err != nil {
		t.Error("provisioned status should be True, updated status should be Unknown and cluster returned successfully: " + err.Error())
	}
}


/** Test_setInitialUpstreamSpec
	- success: buildUpstreamClusterState returns a valid upstream spec
 */
func Test_setInitialUpstreamSpec(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("create")
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.setInitialUpstreamSpec(&mockCluster)
	if cluster.Status.AKSStatus.UpstreamSpec == nil || err != nil {
		t.Error("upstreamSpec should have been set and cluster returned successfully")
	}
}


/** Test_updateAKSClusterConfig
	- success: AKS cluster tags are removed. AKS cluster is not immediately updated. Cluster sits in active for a few
      seconds, return (cluster nil)
 */
func Test_updateAKSClusterConfig(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("akscc")
	mockCluster, err := getMockV3Cluster(MockAKSClusterConfigClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}
	mockAksClusterConfig, err := getMockAksClusterConfig(MockAKSClusterConfigFilename)

	// test remove tags from the cluster
	_, err = mockOperatorController.updateAKSClusterConfig(&mockCluster, mockAksClusterConfig, nil)

	if err != nil {
		t.Error("AKSClusterConfig should have been updated and cluster returned successfully: " + err.Error())
	}
}


/** Test_generateAndSetServiceAccount
	- success: service account token generated, cluster updated! Return updated cluster.Status
	- error generating service account token. Return (cluster, err)
 */
func Test_generateAndSetServiceAccount(t *testing.T) {
	mockOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.generateAndSetServiceAccount(&mockCluster)
	if cluster.Status.ServiceAccountTokenSecret == "" && err != nil {
		t.Error("service account token and secret should have been set on Status and cluster returned successfully")
	}
}


/** Test_buildAKSCCCreateObject
	- success: AKSClusterConfig object created, return (AKSClusterConfig nil)
 */
func Test_buildAKSCCCreateObject(t *testing.T) {
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}
	expected, err := getMockAksClusterConfig(MockBuildAKSCCCreateObjectFilename)

	akscc, err := buildAKSCCCreateObject(&mockCluster)

	if !reflect.DeepEqual(akscc, expected) {
		t.Error("AKS cluster config object was not built as expected")
	}
	if err != nil {
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
	mockOperatorController = getMockAksOperatorController("default")
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)
	if cluster.Status.AppliedSpec.AKSConfig == nil || err != nil {
		t.Error("cluster Status.AppliedSpec should have been updated with AKSConfig: " + err.Error())
	}
}

func Test_recordAppliedSpec_NoUpdate(t *testing.T) {
	// A mock active cluster already has the AKSConfig set on the applied spec, so no update is required.
	mockOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)
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
	mockOperatorController = getMockAksOperatorController("active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Error("error getting mock v3 cluster: ", err.Error())
	}
	input := mockCluster.DeepCopy()
	isPrivate := true
	input.Status.AKSStatus.UpstreamSpec.PrivateCluster = &isPrivate

	token, requiresTunnel, err := mockOperatorController.generateSATokenWithPublicAPI(input)
	if token == "" || to.Bool(requiresTunnel) != false || err != nil {
		t.Error("values (token, false, nil) should have been returned successfully")
	}
}


/** Test_getRestConfig
 */
func Test_getRestConfig(t *testing.T) {
	t.Skip("not implemented: requires AKS controller")
}
