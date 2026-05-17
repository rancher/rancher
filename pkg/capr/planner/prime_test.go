package planner

import (
"testing"

rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
"github.com/rancher/rancher/pkg/capr"
"github.com/rancher/rancher/pkg/settings"
"github.com/stretchr/testify/assert"
metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func newTestControlPlane(k8sVersion string, annotations map[string]string) *rkev1.RKEControlPlane {
return &rkev1.RKEControlPlane{
ObjectMeta: metav1.ObjectMeta{
Name:        "test-cluster",
Namespace:   "fleet-default",
Annotations: annotations,
},
Spec: rkev1.RKEControlPlaneSpec{
KubernetesVersion: k8sVersion,
},
}
}

func newServerEntry() *planEntry {
return &planEntry{
Machine: &capi.Machine{
ObjectMeta: metav1.ObjectMeta{
Labels: map[string]string{
capr.ControlPlaneRoleLabel: "true",
capr.EtcdRoleLabel:        "true",
},
},
},
Metadata: &plan.Metadata{
Labels: map[string]string{
capr.ControlPlaneRoleLabel: "true",
capr.EtcdRoleLabel:        "true",
},
},
}
}

func newWorkerEntry() *planEntry {
return &planEntry{
Machine: &capi.Machine{
ObjectMeta: metav1.ObjectMeta{
Labels: map[string]string{
capr.WorkerRoleLabel: "true",
},
},
},
Metadata: &plan.Metadata{
Labels: map[string]string{
capr.WorkerRoleLabel: "true",
},
},
}
}

// TestAddRKE2Prime tests the addRKE2Prime function.
// Note: The KDM release data check (capr.GetKDMReleaseData) requires the channelserver
// to be initialized, which is not available in unit tests. Therefore, tests that would
// result in prime=true will instead verify that the function returns early due to nil
// KDM release data, which is the correct safety behavior.
func TestAddRKE2Prime(t *testing.T) {
origDefault := settings.Rke2ProvisioningPrimeDefault.Get()
defer func() { _ = settings.Rke2ProvisioningPrimeDefault.Set(origDefault) }()

t.Run("k3s runtime is skipped", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("true")
cp := newTestControlPlane("v1.35.0+k3s1", nil)
config := map[string]interface{}{}
addRKE2Prime(config, cp, newServerEntry())
assert.NotContains(t, config, "prime")
})

t.Run("worker node is skipped", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("true")
cp := newTestControlPlane("v1.35.0+rke2r1", nil)
config := map[string]interface{}{}
addRKE2Prime(config, cp, newWorkerEntry())
assert.NotContains(t, config, "prime")
})

t.Run("existing prime key is not overridden", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("true")
cp := newTestControlPlane("v1.35.0+rke2r1", nil)
config := map[string]interface{}{"prime": false}
addRKE2Prime(config, cp, newServerEntry())
assert.Equal(t, false, config["prime"])
})

t.Run("setting false means prime not enabled", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("false")
cp := newTestControlPlane("v1.35.0+rke2r1", nil)
config := map[string]interface{}{}
addRKE2Prime(config, cp, newServerEntry())
assert.NotContains(t, config, "prime")
})

t.Run("annotation false overrides true setting", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("true")
cp := newTestControlPlane("v1.35.0+rke2r1", map[string]string{
capr.RKE2PrimeEnabledAnnotation: "false",
})
config := map[string]interface{}{}
addRKE2Prime(config, cp, newServerEntry())
assert.NotContains(t, config, "prime")
})

t.Run("annotation true overrides false setting but KDM not available", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("false")
cp := newTestControlPlane("v1.35.0+rke2r1", map[string]string{
capr.RKE2PrimeEnabledAnnotation: "true",
})
config := map[string]interface{}{}
addRKE2Prime(config, cp, newServerEntry())
// Without KDM release data, prime won't be added (safety check)
assert.NotContains(t, config, "prime")
})

t.Run("setting true but KDM not available", func(t *testing.T) {
_ = settings.Rke2ProvisioningPrimeDefault.Set("true")
cp := newTestControlPlane("v1.35.0+rke2r1", nil)
config := map[string]interface{}{}
addRKE2Prime(config, cp, newServerEntry())
// Without KDM release data, prime won't be added (safety check)
assert.NotContains(t, config, "prime")
})
}
