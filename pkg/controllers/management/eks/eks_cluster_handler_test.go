package eks

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/capr"

	"github.com/Azure/go-autorest/autorest/to"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

const (
	MockDefaultClusterFilename          = "test/onclusterchange_default.yaml"
	MockCreateClusterFilename           = "test/onclusterchange_create.yaml"
	MockActiveClusterFilename           = "test/onclusterchange_active.yaml"
	MockUpdateClusterFilename           = "test/onclusterchange_update.yaml"
	MockEksClusterConfigFilename        = "test/updateeksclusterconfig.json"
	MockEksClusterConfigClusterFilename = "test/updateeksclusterconfig.yaml"
	MockBuildEksCCCreateObjectFilename  = "test/buildekscccreateobject.json"
)

var mockOperatorController mockEksOperatorController // Operator controller with mock interfaces & sibling funcs

/*
* Test_onClusterChange
- cluster == nil. Return (nil nil)
- cluster.DeletionTimestamp or cluster.EksConfig == nil, return (nil nil)
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

func Test_onClusterChange_EKSConfigIsNil(t *testing.T) {
	mockCluster := &v3.Cluster{
		Spec: v3.ClusterSpec{
			EKSConfig: nil,
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
	mockOperatorController = getMockEksOperatorController(t, "default")

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
	mockOperatorController = getMockEksOperatorController(t, "create")
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
	mockOperatorController = getMockEksOperatorController(t, "active")
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

func Test_onClusterChange_UpdateNodePool(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "update")
	mockCluster, err := getMockV3Cluster(MockUpdateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.onClusterChange("", &mockCluster)

	// check that cluster is updating node pool
	if err != nil {
		t.Errorf("error running onClusterChange: %s", err)
	}
	if !capr.Provisioned.IsTrue(cluster) || !capr.Updated.IsUnknown(cluster) {
		t.Errorf("provisioned status should be True, updated status should be Unknown and cluster returned successfully")
	}
}

func Test_onClusterChange_Active_nilUpstreamSpec(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	mockCluster.Name = "test"
	mockCluster.Status.EKSStatus.UpstreamSpec = nil

	_, err = mockOperatorController.onClusterChange("", &mockCluster)

	exp := fmt.Errorf("initial upstreamSpec on cluster [test] has not been set, unable to continue")
	if err.Error() != exp.Error() {
		t.Errorf("expected error %q but got %q", exp, err)
	}
}

/*
* Test_setInitialUpstreamSpec
- success: buildUpstreamClusterState returns a valid upstream spec
*/
func Test_setInitialUpstreamSpec(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "create")
	mockCluster, err := getMockV3Cluster(MockCreateClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.setInitialUpstreamSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running setInitialUpstreamSpec: %s", err)
	}
	if cluster.Status.EKSStatus.UpstreamSpec == nil {
		t.Errorf("upstreamSpec should have been set and cluster returned successfully")
	}
}

/*
* Test_updateEKSClusterConfig
  - success: EKS cluster tags are removed. EKS cluster is not immediately updated. Cluster sits in active for a few
    seconds, return (cluster nil)
*/
func Test_updateEKSClusterConfig(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "Ekscc")
	mockCluster, err := getMockV3Cluster(MockEksClusterConfigClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	mockEksClusterConfig, err := getMockEksClusterConfig(MockEksClusterConfigFilename)

	// test remove tags from the cluster
	_, err = mockOperatorController.updateEKSClusterConfig(&mockCluster, mockEksClusterConfig, nil)

	if err != nil {
		t.Errorf("error running updateEKSClusterConfig: %s", err)
	}
}

/*
* Test_generateAndSetServiceAccount
- success: service account token generated, cluster updated! Return updated cluster.Status
- error generating service account token. Return (cluster, err)
*/
func Test_generateAndSetServiceAccount(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.generateAndSetServiceAccount(&mockCluster)

	// check that serviceAccountToken name and token are set
	if err != nil {
		t.Errorf("error running generateAndSetServiceAccount: %s", err)
	}
	if cluster.Status.ServiceAccountTokenSecret == "" {
		t.Errorf("service account token secret should have been set on Status and cluster returned successfully")
	}
}

/*
* Test_buildEKSCCCreateObject
- success: EKSClusterConfig object created, return (EKSClusterConfig nil)
*/
func Test_buildEKSCCCreateObject(t *testing.T) {
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	expected, err := getMockEksClusterConfig(MockBuildEksCCCreateObjectFilename)

	ekscc, err := buildEKSCCCreateObject(&mockCluster)

	if err != nil {
		t.Errorf("error running buildEKSCCCreateObject: %s", err)
	}
	if !reflect.DeepEqual(ekscc, expected) {
		t.Errorf("EKS cluster config object was not built as expected")
	}
}

/*
* Test_recordAppliedSpec
- success: set current spec as applied spec. Return (updated cluster err)
- success: EksConfig and Applied Spec EksConfig are equal. Return (cluster nil)
*/
func Test_recordAppliedSpec_Updated(t *testing.T) {
	// We use a mock cluster that is still provisioning and in an Unknown state, because that is when the applied spec
	// needs to be updated.
	mockOperatorController = getMockEksOperatorController(t, "default")
	mockCluster, err := getMockV3Cluster(MockDefaultClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}

	cluster, err := mockOperatorController.recordAppliedSpec(&mockCluster)

	if err != nil {
		t.Errorf("error running recordAppliedSpec: %s", err)
	}
	if cluster.Status.AppliedSpec.EKSConfig == nil {
		t.Errorf("cluster Status.AppliedSpec should have been updated with EKSConfig")
	}
}

func Test_recordAppliedSpec_NoUpdate(t *testing.T) {
	// A mock active cluster already has the EKSConfig set on the applied spec, so no update is required.
	mockOperatorController = getMockEksOperatorController(t, "active")
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

func Test_getAccessToken(t *testing.T) {
	t.Skip("not implemented: requires EKS controller")
}

/*
  - Test_generateSATokenWithPublicAPI
    PRIVATE CLUSTER ONLY
  - success in getting a service account token from the public API endpoint. Return (token mustTunnel=false nil)
  - failure to get service account token. Return ("" mustTunnel=true err)
  - unknown error. Return ("" mustTunnel=nil err)
*/
func Test_generateSATokenWithPublicAPI(t *testing.T) {
	mockOperatorController = getMockEksOperatorController(t, "active")
	mockCluster, err := getMockV3Cluster(MockActiveClusterFilename)
	if err != nil {
		t.Errorf("error getting mock v3 cluster: %s", err)
	}
	input := mockCluster.DeepCopy()
	hasPublicAccess := false
	input.Status.EKSStatus.UpstreamSpec.PublicAccess = &hasPublicAccess

	token, requiresTunnel, err := mockOperatorController.generateSATokenWithPublicAPI(input)

	if err != nil {
		t.Errorf("error running generateSATokenWithPublicAPI: %s", err)
	}
	if token == "" || to.Bool(requiresTunnel) != false {
		t.Errorf("values (token, requiresTunnel=false, nil) should have been returned successfully")
	}
}

/** Test_getRestConfig
 */
func Test_getRestConfig(t *testing.T) {
	t.Skip("not implemented: requires EKS controller")
}
