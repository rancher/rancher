package managesystemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	generationSecretName    = "system-agent-upgrade-generation"
	upgradeAPIVersion       = "upgrade.cattle.io/v1"
	upgradeDigestAnnotation = "upgrade.cattle.io/digest"

	SystemAgentUpgrader = "system-agent-upgrader"

	AppliedSystemAgentUpgraderHashAnnotation = "rke.cattle.io/applied-system-agent-upgrader-hash"
)

var (
	Kubernetes125 = semver.MustParse("v1.25.0")

	// GH5551FixedVersions is a slice of rke2 versions
	// which have resolved GH-5551 for Windows nodes.
	// ref:  https://github.com/rancher/rke2/issues/5551
	// The SUC should not deploy plans to Windows nodes
	// running a version less than the below for each minor.
	// This check can be removed when 1.31.x is the lowest supported
	// rke2 version.
	GH5551FixedVersions = map[int]*semver.Version{
		30: semver.MustParse("v1.30.4"),
		29: semver.MustParse("v1.29.8"),
		28: semver.MustParse("v1.28.13"),
		27: semver.MustParse("v1.27.16"),
	}

	// uninstallCounter keeps track of the number of clusters for which the handler is concurrently uninstalling the Fleet-based app.
	// An atomic integer is used for efficiency, as it is lighter than a traditional lock.
	uninstallCounter atomic.Int32

	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32
)

type handler struct {
	clusterRegistrationTokens v3.ClusterRegistrationTokenCache
	bundles                   fleetcontrollers.BundleController
	cluster                   provisioningcontrollers.ClusterController
	rkeControlPlanes          v1.RKEControlPlaneController
	managedCharts             v3.ManagedChartController
	secrets                   corev1controllers.SecretController
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		clusterRegistrationTokens: clients.Mgmt.ClusterRegistrationToken().Cache(),
		bundles:                   clients.Fleet.Bundle(),
		cluster:                   clients.Provisioning.Cluster(),
		rkeControlPlanes:          clients.RKE.RKEControlPlane(),
		managedCharts:             clients.Mgmt.ManagedChart(),
		secrets:                   clients.Core.Secret(),
	}

	clients.Provisioning.Cluster().OnChange(ctx, "uninstall-fleet-managed-suc-and-system-agent", h.UninstallFleetBasedApps)
	clients.Provisioning.Cluster().OnChange(ctx, "install-system-agent-upgrader", h.InstallSystemAgentUpgrader)

}

// InstallSystemAgentUpgrader ensures that the resources required to upgrade the system-agent in the target cluster
// are deployed and kept up to date. It uses Wrangler's Apply mechanism to manage the resources and leverages
// the hash of the rendered templates to avoid redundant calls to the downstream cluster's API server.
func (h *handler) InstallSystemAgentUpgrader(_ string, cluster *rancherv1.Cluster) (*rancherv1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}
	// Skip if it is not a node-driver or custom RKE2/k3s cluster
	if cluster.Spec.RKEConfig == nil {
		return cluster, nil
	}
	if settings.SystemAgentUpgradeImage.Get() == "" {
		logrus.Debugf("[managesystemagent] cluster %s/%s: the SystemAgentUpgradeImage setting is not set, skip installing system-agent-upgrader", cluster.Namespace, cluster.Name)
		return cluster, fmt.Errorf("[managesystemagent] cluster %s/%s: the SystemAgentUpgradeImage setting is not set", cluster.Namespace, cluster.Name)
	}
	if settings.SystemUpgradeControllerChartVersion.Get() == "" {
		logrus.Debugf("[managesystemagent] cluster %s/%s: the SystemUpgradeControllerChartVersion setting is not set, skip installing system-agent-upgrader", cluster.Namespace, cluster.Name)
		return cluster, fmt.Errorf("[managesystemagent] cluster %s/%s: the SystemUpgradeControllerChartVersion setting is not set", cluster.Namespace, cluster.Name)
	}
	// Skip if Rancher does not have a connection to the cluster
	if !clusterconnected.Connected.IsTrue(cluster) {
		return cluster, nil
	}
	// Skip if the cluster's kubeconfig is not populated
	if cluster.Status.ClientSecretName == "" {
		return cluster, nil
	}

	cp, err := h.rkeControlPlanes.Cache().Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return cluster, err
	}
	if capr.SystemUpgradeControllerReady.GetStatus(cp) == "" {
		logrus.Debugf("[managesystemagent] cluster %s/%s: SystemUpgradeControllerReady condition is not found, skip installing system-agent-upgrader", cluster.Namespace, cluster.Name)
		return cluster, nil
	}
	// Skip if the system-upgrade-controller app is not ready or the target version has not been installed,
	// because new Plans may depend on functionality of a new version of the system-upgrade-controller app
	if !capr.SystemUpgradeControllerReady.IsTrue(cp) {
		logrus.Debugf("[managesystemagent] cluster %s/%s: waiting for system-upgrade-controller to be ready (reason: %s)",
			cluster.Namespace, cluster.Name, capr.SystemUpgradeControllerReady.GetReason(cp))
		return cluster, nil
	}

	targetVersion := settings.SystemUpgradeControllerChartVersion.Get()
	if targetVersion != capr.SystemUpgradeControllerReady.GetMessage(cp) {
		logrus.Debugf("[managesystemagent] cluster %s/%s: waiting for system-upgrade-controller to be upgraded to %s",
			cluster.Namespace, cluster.Name, targetVersion)
		return cluster, nil
	}

	var (
		secretName = "stv-aggregation"
		result     []runtime.Object
	)

	if cluster.Status.ClusterName == "local" && cluster.Namespace == fleetconst.ClustersLocalNamespace {
		secretName += "-local-"

		token, err := h.clusterRegistrationTokens.Get(cluster.Status.ClusterName, "default-token")
		if err != nil {
			return cluster, err
		}
		if token.Status.Token == "" {
			return cluster, fmt.Errorf("token not yet generated for %s/%s", token.Namespace, token.Name)
		}

		digest := sha256.New()
		digest.Write([]byte(settings.InternalServerURL.Get()))
		digest.Write([]byte(token.Status.Token))
		digest.Write([]byte(systemtemplate.InternalCAChecksum()))
		d := digest.Sum(nil)
		secretName += hex.EncodeToString(d[:])[:12]

		result = append(result, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespaces.System,
			},
			Data: map[string][]byte{
				"CATTLE_SERVER":      []byte(settings.InternalServerURL.Get()),
				"CATTLE_TOKEN":       []byte(token.Status.Token),
				"CATTLE_CA_CHECKSUM": []byte(systemtemplate.InternalCAChecksum()),
			},
		})
	}

	result = append(result, installer(cluster, secretName)...)

	// Calculate a hash value of the templates
	data, err := json.Marshal(result)
	if err != nil {
		return cluster, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	val, ok := cp.Annotations[AppliedSystemAgentUpgraderHashAnnotation]
	if ok && hash == val {
		logrus.Debugf("[managesystemagent] cluster %s/%s: applied templates for system-agent-upgrader is up to date. "+
			"To trigger a force redeployment, remove the %s annotation from the conresponding rkeControlPlane object",
			cluster.Namespace, cluster.Name, AppliedSystemAgentUpgraderHashAnnotation)
		return cluster, nil
	}

	// Limit the number of cluster to be processed simultaneously
	if installCounter.Load() >= int32(settings.SystemAgentUpgraderInstallConcurrency.GetInt()) {
		h.cluster.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
		return cluster, nil
	}
	installCounter.Add(1)
	defer installCounter.Add(-1)

	logrus.Infof("[managesystemagent] cluster %s/%s: applying system-agent-upgrader templates", cluster.Namespace, cluster.Name)
	// Construct a Wrangler's Apply object
	kcSecret, err := h.secrets.Cache().Get(cluster.Namespace, cluster.Status.ClientSecretName)
	if err != nil {
		return cluster, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kcSecret.Data["value"])
	if err != nil {
		return cluster, fmt.Errorf("failed to get rest config: %w", err)
	}
	apply, err := apply.NewForConfig(restConfig)
	if err != nil {
		return cluster, fmt.Errorf("failed to create Apply: %w", err)
	}
	err = apply.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects(result...)
	if err != nil {
		return cluster, fmt.Errorf("failed to apply objects: %w", err)
	}

	// Update the annotation with the latest hash value
	cp = cp.DeepCopy()
	if cp.Annotations == nil {
		cp.Annotations = map[string]string{}
	}
	cp.Annotations[AppliedSystemAgentUpgraderHashAnnotation] = hash
	if _, err := h.rkeControlPlanes.Update(cp); err != nil {
		return cluster, fmt.Errorf("failed to update annotation: %w", err)
	}

	return cluster, nil
}

// installer generates the Plans and corresponding Kubernetes resources required for
// deploying and running the system-agent-upgrader
func installer(cluster *rancherv1.Cluster, secretName string) []runtime.Object {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := SystemAgentUpgraderVersion()

	var env []corev1.EnvVar
	for _, e := range cluster.Spec.AgentEnvVars {
		env = append(env, corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		})
	}

	// Merge the env vars with the AgentTLSModeStrict
	found := false
	for _, ev := range env {
		if ev.Name == "STRICT_VERIFY" {
			found = true // The user has specified `STRICT_VERIFY`, we should not attempt to overwrite it.
		}
	}
	if !found {
		if settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict {
			env = append(env, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "true",
			})
		} else {
			env = append(env, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "false",
			})
		}
	}

	if len(cluster.Spec.RKEConfig.MachineSelectorConfig) == 0 {
		env = append(env, corev1.EnvVar{
			Name:  "CATTLE_ROLE_WORKER",
			Value: "true",
		})
	}

	if cluster.Spec.RKEConfig.DataDirectories.SystemAgent != "" {
		env = append(env, corev1.EnvVar{
			Name:  capr.SystemAgentDataDirEnvVar,
			Value: capr.GetSystemAgentDataDir(&cluster.Spec.RKEConfig.ClusterConfiguration),
		})
	}
	var plans []runtime.Object

	plan := &upgradev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: upgradeAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      SystemAgentUpgrader,
			Namespace: namespaces.System,
			Annotations: map[string]string{
				upgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
			},
		},
		Spec: upgradev1.PlanSpec{
			Concurrency: 10,
			Version:     version,
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      corev1.LabelOSStable,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"linux",
						},
					},
				},
			},
			ServiceAccountName: SystemAgentUpgrader,
			// envFrom is still the source of CATTLE_ vars in plan, however secrets will trigger an update when changed.
			Secrets: []upgradev1.SecretSpec{
				{
					Name: "stv-aggregation",
				},
			},
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(upgradeImage[0], cluster),
				Env:   env,
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				}},
			},
		},
	}
	plans = append(plans, plan)

	if CurrentVersionResolvesGH5551(cluster.Spec.KubernetesVersion) {
		windowsPlan := winsUpgradePlan(cluster, env, secretName)
		if cluster.Spec.RedeploySystemAgentGeneration != 0 {
			windowsPlan.Spec.Secrets = append(windowsPlan.Spec.Secrets, upgradev1.SecretSpec{
				Name: generationSecretName,
			})
		}
		plans = append(plans, windowsPlan)
	}

	objs := []runtime.Object{
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SystemAgentUpgrader,
				Namespace: namespaces.System,
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgrader,
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgrader,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      SystemAgentUpgrader,
				Namespace: namespaces.System,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     SystemAgentUpgrader,
			},
		},
	}

	// The stv-aggregation secret is managed separately, and SUC will trigger a plan upgrade automatically when the
	// secret is updated. This prevents us from having to manually update the plan every time the secret changes
	// (which is not often, and usually never).
	if cluster.Spec.RedeploySystemAgentGeneration != 0 {
		plan.Spec.Secrets = append(plan.Spec.Secrets, upgradev1.SecretSpec{
			Name: generationSecretName,
		})

		objs = append(objs, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generationSecretName,
				Namespace: namespaces.System,
			},
			StringData: map[string]string{
				"cluster-uid": string(cluster.UID),
				"generation":  strconv.Itoa(int(cluster.Spec.RedeploySystemAgentGeneration)),
			},
		})
	}

	return append(plans, objs...)
}

func winsUpgradePlan(cluster *rancherv1.Cluster, env []corev1.EnvVar, secretName string) *upgradev1.Plan {
	winsUpgradeImage := strings.SplitN(settings.WinsAgentUpgradeImage.Get(), ":", 2)
	winsVersion := "latest"
	if len(winsUpgradeImage) == 2 {
		winsVersion = winsUpgradeImage[1]
	}

	return &upgradev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: upgradeAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-agent-upgrader-windows",
			Namespace: namespaces.System,
			Annotations: map[string]string{
				upgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
			},
		},
		Spec: upgradev1.PlanSpec{
			Concurrency: 10,
			Version:     winsVersion,
			Tolerations: []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      corev1.LabelOSStable,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"windows",
						},
					},
				},
			},
			ServiceAccountName: SystemAgentUpgrader,
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(winsUpgradeImage[0], cluster),
				Env:   env,
				SecurityContext: &corev1.SecurityContext{
					WindowsOptions: &corev1.WindowsSecurityContextOptions{
						HostProcess:   toBoolPointer(true),
						RunAsUserName: toStringPointer("NT AUTHORITY\\SYSTEM"),
					},
				},
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				}},
			},
		},
	}
}

func toBoolPointer(x bool) *bool {
	return &x
}

func toStringPointer(x string) *string {
	return &x
}

// CurrentVersionResolvesGH5551 determines if the given rke2 version
// has fixed the RKE2 bug outlined in GH-5551. Windows SUC plans cannot be delivered
// to clusters running versions containing this bug. This function can be removed
// when v1.31.x is the lowest supported version offered by Rancher.
func CurrentVersionResolvesGH5551(version string) bool {

	// remove leading v and trailing distro identifier
	v := strings.TrimPrefix(version, "v")
	verSplit := strings.Split(v, "+")
	if len(verSplit) != 2 {
		return false
	}

	curSemVer, err := semver.NewVersion(verSplit[0])
	if err != nil {
		return false
	}

	minor := curSemVer.Minor()
	if minor >= 31 {
		return true
	}
	if minor <= 26 {
		return false
	}

	return curSemVer.GreaterThanEqual(GH5551FixedVersions[int(minor)])
}

// UninstallFleetBasedApps handles the removal of Fleet Bundles for both the managed-system-agent and
// managed-system-upgrade-controller, and also takes care of removing the AppliedSystemAgentUpgraderHashAnnotation
// annotation in a specific scenario.
func (h *handler) UninstallFleetBasedApps(_ string, cluster *rancherv1.Cluster) (*rancherv1.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}
	if cluster.Spec.RKEConfig == nil {
		return cluster, nil
	}
	// The absence of the FleetWorkspaceName indicates that Fleet is not ready on the cluster, so do Fleet bundles
	if cluster.Status.FleetWorkspaceName == "" {
		return cluster, nil
	}

	// Skip if Rancher does not have a connection to the cluster
	if !clusterconnected.Connected.IsTrue(cluster) {
		return cluster, nil
	}

	// Skip if the cluster's kubeconfig is not populated
	if cluster.Status.ClientSecretName == "" {
		return cluster, nil
	}

	if settings.SystemAgentUpgradeImage.Get() == "" {
		logrus.Debugf("[managesystemagent] cluster %s/%s: the SystemAgentUpgradeImage setting is not set, skip uninstalling Fleet-based apps", cluster.Namespace, cluster.Name)
		return cluster, fmt.Errorf("[managesystemagent] cluster %s/%s: the SystemAgentUpgradeImage setting is not set", cluster.Namespace, cluster.Name)
	}
	if settings.SystemUpgradeControllerChartVersion.Get() == "" {
		logrus.Debugf("[managesystemagent] cluster %s/%s: the SystemUpgradeControllerChartVersion setting is not set, skip uninstalling Fleet-based apps", cluster.Namespace, cluster.Name)
		return cluster, fmt.Errorf("[managesystemagent] cluster %s/%s: the SystemUpgradeControllerChartVersion setting is not set", cluster.Namespace, cluster.Name)
	}

	dropAnnotation := false
	// Limit the number of cluster to be processed simultaneously
	if uninstallCounter.Load() >= int32(settings.K3sBasedUpgraderUninstallConcurrency.GetInt()) {
		h.cluster.EnqueueAfter(cluster.Namespace, cluster.Name, 5*time.Second)
		return cluster, nil
	}
	uninstallCounter.Add(1)
	defer uninstallCounter.Add(-1)

	// Step 1: uninstall the system-agent bundle
	name := capr.SafeConcatName(capr.MaxHelmReleaseNameLength, cluster.Name, "managed", "system-agent")
	bundle, err := h.bundles.Cache().Get(cluster.Status.FleetWorkspaceName, name)
	if err != nil && !errors.IsNotFound(err) {
		return cluster, err
	}
	if bundle != nil && bundle.DeletionTimestamp == nil {
		logrus.Infof("[managesystemagent] cluster %s/%s: uninstalling the bundle %s", cluster.Namespace, cluster.Name, bundle.Name)
		err := h.bundles.Delete(bundle.Namespace, bundle.Name, &metav1.DeleteOptions{})
		if err == nil {
			dropAnnotation = true
		} else if !errors.IsNotFound(err) {
			return nil, err
		}
	}

	// step 2: uninstall the system-upgrade-controller managedChart( which is translated into a Fleet Bundle by another handler)
	sucName := capr.SafeConcatName(48, cluster.Name, "managed", "system-upgrade-controller")
	managedChart, err := h.managedCharts.Cache().Get(cluster.Status.FleetWorkspaceName, sucName)
	if err != nil && !errors.IsNotFound(err) {
		return cluster, err
	}
	if managedChart != nil && managedChart.DeletionTimestamp == nil {
		logrus.Infof("[managesystemagent] cluster %s/%s: uninstalling the managedChart %s", cluster.Namespace, cluster.Name, sucName)
		err := h.managedCharts.Delete(managedChart.Namespace, managedChart.Name, &metav1.DeleteOptions{})
		if err == nil {
			dropAnnotation = true
		} else if !errors.IsNotFound(err) {
			return nil, err
		}
	}

	// The AppliedSystemAgentUpgraderHashAnnotation annotation should not exist when Fleet bundles are present.
	// This may exist if Rancher is upgraded to 2.12.x, then rolled back to 2.11.x, and later re-upgraded to 2.12.x without restoring the local cluster.
	// In such cases, remove the annotation if it exists to ensure the system-agent-upgrader will be
	// applied by the InstallSystemAgentUpgrader function.
	if !dropAnnotation {
		return cluster, nil
	}
	cp, err := h.rkeControlPlanes.Cache().Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return cluster, err
	}
	if cp.Annotations == nil {
		return cluster, nil
	}
	if _, ok := cp.Annotations[AppliedSystemAgentUpgraderHashAnnotation]; !ok {
		return cluster, nil
	}
	logrus.Debugf("[managesystemagent] cluster %s/%s: removing AppliedSystemAgentUpgraderHashAnnotation", cluster.Namespace, cluster.Name)
	cp = cp.DeepCopy()
	delete(cp.Annotations, AppliedSystemAgentUpgraderHashAnnotation)
	if _, err = h.rkeControlPlanes.Update(cp); err != nil {
		return cluster, err
	}

	return cluster, nil
}

// SystemAgentUpgraderVersion returns the version of the system-agent-upgrader,
// which is determined by the image tag or defaults to "latest" if unspecified.
func SystemAgentUpgraderVersion() string {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := "latest"
	if len(upgradeImage) == 2 {
		version = upgradeImage[1]
	}
	return version
}
