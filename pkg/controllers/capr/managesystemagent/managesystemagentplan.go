package managesystemagent

import (
	"fmt"
	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) OnChangeInstallSUC(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if cluster.Spec.RKEConfig == nil {
		return nil, status, nil
	}

	if cluster.Status.FleetWorkspaceName == "" {
		return nil, status, nil
	}

	// we must limit the output of name.SafeConcatName to at most 48 characters because
	// a) the chart release name cannot exceed 53 characters, and
	// b) upon creation of this resource the prefix 'mcc-' will be added to the release name, hence the limiting to 48 characters
	mcc := &v3.ManagedChart{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Status.FleetWorkspaceName,
			Name:      capr.SafeConcatName(48, cluster.Name, "managed", "system-upgrade-controller"),
		},
		Spec: v3.ManagedChartSpec{
			DefaultNamespace: namespaces.System,
			RepoName:         "rancher-charts",
			Chart:            "system-upgrade-controller",
			Version:          settings.SystemUpgradeControllerChartVersion.Get(),
			Values: &v1alpha1.GenericMap{
				Data: map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": image.GetPrivateRepoURLFromCluster(cluster),
						},
					},
				},
			},
			Targets: []v1alpha1.BundleTarget{
				{
					ClusterName: cluster.Name,
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "provisioning.cattle.io/unmanaged-system-agent",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}

	return []runtime.Object{
		mcc,
	}, status, nil
}

// syncSystemUpgradeControllerStatus queries the managed system-upgrade-controller chart and determines if it is properly configured for a given
// version of Kubernetes. It applies a condition onto the control-plane object to be used by the planner when handling Kubernetes upgrades.
func (h *handler) syncSystemUpgradeControllerStatus(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	// perform the same name limiting as in the OnChangeInstallSUC and the managedchart controller
	cluster, err := h.provClusters.Get(obj.Namespace, obj.Name)
	if err != nil {
		return status, err
	}
	if cluster.Status.FleetWorkspaceName == "" {
		return status, fmt.Errorf("unable to sync system upgrade controller status for [%s] [%s/%s], status.FleetWorkspaceName was blank", cluster.TypeMeta.String(), cluster.Namespace, cluster.Name)
	}

	bundleName := capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc", capr.SafeConcatName(48, obj.Name, "managed", "system-upgrade-controller"))
	sucBundle, err := h.bundles.Get(cluster.Status.FleetWorkspaceName, bundleName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// if we couldn't find the bundle then we know it's not ready
			capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("unable to find bundle %s: %v", bundleName, err))
			capr.SystemUpgradeControllerReady.Message(&status, "")
			capr.SystemUpgradeControllerReady.False(&status)
			// don't return the error, otherwise the status won't be set to 'false'
			return status, nil
		}
		logrus.Errorf("[managesystemagentplan] rkecluster %s/%s: error encountered while retrieving bundle %s: %v", obj.Namespace, obj.Name, bundleName, err)
		return status, err
	}

	if sucBundle.Spec.Helm.Version != settings.SystemUpgradeControllerChartVersion.Get() && settings.SystemUpgradeControllerChartVersion.Get() != "" {
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("waiting for system-upgrade-controller bundle to update to the latest version %s", settings.SystemUpgradeControllerChartVersion.Get()))
		capr.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	// determine if the SUC deployment has been rolled out fully, and if there were any errors encountered
	if sucBundle.Status.Summary.Ready != sucBundle.Status.Summary.DesiredReady || sucBundle.Status.Summary.DesiredReady == 0 || sucBundle.Status.Summary.Pending != 0 {
		if sucBundle.Status.Summary.ErrApplied != 0 && len(sucBundle.Status.Summary.NonReadyResources) > 0 {
			nonReady := sucBundle.Status.Summary.NonReadyResources
			capr.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("error encountered waiting for system-upgrade-controller bundle roll out: %s", nonReady[0].Message))
			capr.SystemUpgradeControllerReady.Message(&status, "")
			capr.SystemUpgradeControllerReady.Unknown(&status)
			return status, nil
		}
		capr.SystemUpgradeControllerReady.Reason(&status, "waiting for system-upgrade-controller bundle roll out")
		capr.SystemUpgradeControllerReady.Message(&status, "")
		capr.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	capr.SystemUpgradeControllerReady.Message(&status, "")
	capr.SystemUpgradeControllerReady.Reason(&status, "")
	capr.SystemUpgradeControllerReady.True(&status)
	return status, nil
}
