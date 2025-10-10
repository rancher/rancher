package alibaba

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
)

const (
	MockDefaultClusterFilename          = "test/onclusterchange_default.yaml"
	MockCreateClusterFilename           = "test/onclusterchange_create.yaml"
	MockActiveClusterFilename           = "test/onclusterchange_active.yaml"
	MockUpdateClusterFilename           = "test/onclusterchange_update.yaml"
	MockAliClusterConfigFilename        = "test/updatealicc.json"
	MockAliClusterConfigClusterFilename = "test/updatealicc.yaml"
	MockBuildAliCCCreateObjectFilename  = "test/buildalicccreateobject.json"
)

var mockOperatorController aliOperatorController // Operator controller with mock interfaces & sibling funcs

/*
* Test_onClusterChange
- cluster == nil. Return (nil nil)
- cluster.DeletionTimestamp or cluster.AliConfig == nil, return (nil nil)
- default phase
- create phase
- active phase
- update node pool phase
*/
func Test_onClusterChange_ClusterIsNil(t *testing.T) {
	cluster, _ := mockOperatorController.onClusterChange("", nil)
	if cluster != nil {
		t.Errorf("cluster should have returned nil")
	}
}

func Test_onClusterChange_AliConfigIsNil(t *testing.T) {
	mockCluster := &v3.Cluster{
		Spec: v3.ClusterSpec{
			AliConfig: nil,
		},
	}

	cluster, _ := mockOperatorController.onClusterChange("", mockCluster)
	if !reflect.DeepEqual(cluster, mockCluster) {
		t.Errorf("cluster should have returned with no update")
	}
}

func Test_onClusterChange_Default(t *testing.T) {

	// setup
	// create an instance of the operator controller with mock data to simulate the onChangeCluster function reacting
	// to a real cluster!
	mockOperatorController = getMockAliOperatorController(t, "default")

	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	// run test
	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// validate results
	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsUnknown(cluster) {
		t.Errorf("provisioned status should be Unknown and cluster returned successfully")
	}
}

func Test_onClusterChange_Create(t *testing.T) {
	mockOperatorController = getMockAliOperatorController(t, "create")
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsUnknown(cluster) {
		t.Errorf("provisioned status should be Unknown and cluster returned successfully")
	}
}

func Test_onClusterChange_Active(t *testing.T) {
	mockOperatorController = getMockAliOperatorController(t, "active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsTrue(cluster) || !capr.Updated.IsTrue(cluster) {
		t.Errorf("provisioned and updated status should be True and cluster returned successfully")
	}
}

func Test_onClusterChange_Update(t *testing.T) {
	mockOperatorController = getMockAliOperatorController(t, "update")
	mockCluster, err := getMockV3Cluster(MockUpdateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// check that cluster is updating
	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsTrue(cluster) || !capr.Updated.IsUnknown(cluster) {
		t.Errorf("provisioned status should be True, updated status should be Unknown and cluster returned successfully")
	}
}

/*
* Test_updateAliClusterConfig
  - success: Ali cluster tags are removed. Ali cluster is not immediately updated. Cluster sits in active for a few
    seconds, return (cluster nil)
*/
func Test_updateAliClusterConfig(t *testing.T) {
	mockOperatorController = getMockAliOperatorController(t, "alicc")
	mockCluster, err := getMockV3Cluster(MockAliClusterConfigClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	mockAliClusterConfig, err := getMockAliClusterConfig(MockAliClusterConfigFilename)
	if err != nil {
		t.Errorf("error getting mock ali cluster config: %s", err)
	}

	// test remove tags from the cluster
	_, err = mockOperatorController.updateAliClusterConfig(&mockCluster, mockAliClusterConfig, nil)

	if err != nil {
		t.Errorf("error running updateAliClusterConfig: %s", err)
	}
}

/*
* Test_buildAliCCCreateObject
- success: AliClusterConfig object created, return (AliClusterConfig nil)
*/
func Test_buildAliCCCreateObject(t *testing.T) {
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	expected, err := getMockAliClusterConfig(MockBuildAliCCCreateObjectFilename)
	if err != nil {
		t.Errorf("error getting mock ali cluster config: %s", err)
	}

	alicc, err := buildAliCCCreateObject(&mockCluster)

	if err != nil {
		t.Errorf("error running buildAliCCCreateObject: %s", err)
	}
	if !reflect.DeepEqual(alicc, expected) {
		t.Errorf("AliClusterConfig object was not built as expected")
	}
}

/*
* Test_recordAppliedSpec
- success: set current spec as applied spec. Return (updated cluster err)
- success: AliConfig and Applied Spec AliConfig are equal. Return (cluster nil)
*/
func Test_recordAppliedSpec_Updated(t *testing.T) {
	// We use a mock cluster that is still provisioning and in an Unknown state, because that is when the applied spec
	// needs to be updated.
	mockOperatorController = getMockAliOperatorController(t, "default")
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running recordAppliedSpec: %s", err)
	}
	if cluster.Status.AppliedSpec.AliConfig == nil {
		t.Errorf("cluster Status.AppliedSpec should have been updated with AliConfig")
	}
}

func Test_recordAppliedSpec_NoUpdate(t *testing.T) {
	// A mock active cluster already has the AliConfig set on the applied spec, so no update is required.
	mockOperatorController = getMockAliOperatorController(t, "active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running recordAppliedSpec: %s", err)
	}
	if !reflect.DeepEqual(cluster.Status.AppliedSpec, mockCluster.Status.AppliedSpec) {
		t.Errorf("cluster Status.AppliedSpec should have no update and cluster returned successfully")
	}
}
