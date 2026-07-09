package systemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	k8sprovider "github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// copied from `pkg/controllers/capr/managesystemagent/managesystemagent.go`

const (
	Day2OpsEnabledAnnotation                 = "operations.cattle.io/ops-enabled"
	UpgradeDigestAnnotation                  = "upgrade.cattle.io/digest"
	AppliedSystemAgentUpgraderHashAnnotation = "management.cattle.io/applied-system-agent-upgrader-hash"

	SystemAgentUpgraderPlanName               = "system-agent-upgrader"
	SystemAgentUpgraderServiceAccountName     = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleName        = "system-agent-upgrader"
	SystemAgentUpgraderClusterRoleBindingName = "system-agent-upgrader"
)

// Enabled checks if version management is enabled for a given cluster
func OperationsEnabledForCluster(cluster *apimgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}

	value := ""
	if cluster.Annotations != nil {
		value = cluster.Annotations[Day2OpsEnabledAnnotation]
	}

	switch value {
	case "true":
		return true
	case "false":
		return false
	case "system-default":
		fallthrough
	default:
		return settings.ImportedClusterDay2OpsEnabledDefault.Get() == "true"
	}
}

var (
	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32
)

type handler struct {
	ctx         context.Context
	manager     *clustermanager.Manager
	dynamic     *dynamic.Controller
	clusters    mgmtcontrollers.ClusterController
	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache
}

func Register(ctx context.Context, w *wrangler.Context, manager *clustermanager.Manager) {
	h := &handler{
		ctx:         ctx,
		manager:     manager,
		dynamic:     w.Dynamic,
		clusters:    w.Mgmt.Cluster(),
		beacons:     w.Plan.Beacon(),
		beaconCache: w.Plan.Beacon().Cache(),
	}

	w.Mgmt.Cluster().OnChange(ctx, "imported-system-agent-setup", h.OnChange)
}

// shouldInstall determines if the system agent should be installed based on the cluster's properties and annotations.
// v2prov Clusters are handled by the managesystemagent handler, whereas both imported RKE2/K3s and imported CAPRKE2
// should be handled by this controller. Ideally, all will be unified in the future, however, this prevents unnecessary
// regression risks.
func shouldInstall(cluster *apimgmtv3.Cluster) bool {
	if cluster == nil {
		return false
	}

	if cluster.Name == "local" {
		return false
	}

	if cluster.Annotations != nil && cluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		return false
	}

	// imported k3s
	if cluster.Status.Driver == apimgmtv3.ClusterDriverK3s {
		return true
	}

	// imported rke2
	if cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 {
		return true
	}

	// imported CAPRKE2
	if cluster.Labels != nil && cluster.Labels[k8sprovider.ProviderKey] == "rke2" {
		return true
	}

	if cluster.Labels != nil && cluster.Labels[k8sprovider.ProviderKey] == "k3s" {
		return true
	}

	return false
}

func (h *handler) OnChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if cluster.Name == "local" {
		return cluster, nil
	}

	if !shouldInstall(cluster) {
		return cluster, nil
	}

	if features.ImportedDay2Ops.Enabled() {
		if OperationsEnabledForCluster(cluster) {
			return h.InstallSystemAgent(cluster)
		}
	}

	cluster, err := h.UninstallSystemAgent(cluster)
	if err != nil {
		return cluster, err
	}

	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	delete(cluster.Annotations, Day2OpsEnabledAnnotation)

	return h.clusters.Update(cluster)
}

func (h *handler) UninstallSystemAgent(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	ref, err := h.clusterOwner(cluster)
	if err != nil {
		return nil, err
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = ref.Name
	}

	beacon, err := h.beaconCache.Get(namespace, ref.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return cluster, err
	} else if err == nil {
		err = h.beacons.Delete(beacon.Namespace, beacon.Name, &metav1.DeleteOptions{})
		if err != nil {
			return cluster, err
		}
	}

	clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
	if err != nil {
		return cluster, err
	}

	if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
		// skip further work if the cluster's API is not reachable,
		// this usually happen during cattle-cluster-agent being redeployed
		logrus.Debugf("[importedsystemagent] [%s] cluster API is not reachable, will try again", cluster.Name)
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}

	// Limit the number of cluster to be processed simultaneously
	if installCounter.Load() >= int32(settings.SystemAgentUpgraderInstallConcurrency.GetInt()) {
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}
	installCounter.Add(1)
	defer installCounter.Add(-1)

	apply, err := apply.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return cluster, err
	}
	err = apply.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects()
	if err != nil {
		return cluster, err
	}

	// Update the annotation with the latest hash value
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	delete(cluster.Annotations, AppliedSystemAgentUpgraderHashAnnotation)

	return h.clusters.Update(cluster)
}

func (h *handler) clusterOwner(cluster *apimgmtv3.Cluster) (*corev1.ObjectReference, error) {
	if cluster.Labels != nil &&
		cluster.Labels["cluster-api.cattle.io/capi-cluster-owner"] != "" &&
		cluster.Labels["cluster-api.cattle.io/capi-cluster-owner-ns"] != "" {
		capiCluster, err := h.dynamic.Get(capiv1beta2.GroupVersion.WithKind("Cluster"), cluster.Labels["cluster-api.cattle.io/capi-cluster-owner-ns"], cluster.Labels["cluster-api.cattle.io/capi-cluster-owner"])
		if err != nil {
			return nil, err
		}
		m, err := meta.Accessor(capiCluster)
		if err != nil {
			return nil, err
		}
		return &corev1.ObjectReference{
			APIVersion: capiv1beta2.GroupVersion.String(),
			Kind:       "Cluster",
			Namespace:  m.GetNamespace(),
			Name:       m.GetName(),
			UID:        m.GetUID(),
		}, nil
	}

	return &corev1.ObjectReference{
		APIVersion: cluster.APIVersion,
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}, nil
}

func (h *handler) InstallSystemAgent(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	ref, err := h.clusterOwner(cluster)
	if err != nil {
		return nil, err
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = ref.Name
	}

	_, err = h.beaconCache.Get(namespace, ref.Name)
	if apierrors.IsNotFound(err) {
		_, err = h.beacons.Create(&planv1alpha1.Beacon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: ref.APIVersion,
						Kind:       ref.Kind,
						Name:       ref.Name,
						UID:        ref.UID,
					},
				},
			}})
		if err != nil {
			return cluster, err
		}
	} else if err != nil {
		return cluster, err
	}

	clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
	if err != nil {
		return cluster, err
	}

	if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
		// skip further work if the cluster's API is not reachable,
		// this usually happen during cattle-cluster-agent being redeployed
		logrus.Debugf("[importedsystemagent] [%s] cluster API is not reachable, will try again", cluster.Name)
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}

	var (
		secretName = "stv-aggregation"
		result     []runtime.Object
	)

	result = append(result, installer(cluster, secretName)...)

	// Calculate a hash value of the templates
	data, err := json.Marshal(result)
	if err != nil {
		return cluster, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	val, ok := cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation]
	if ok && hash == val {
		logrus.Debugf("[importedsystemagent] cluster %s/%s: applied templates for system-agent-upgrader is up to date. "+
			"To trigger a force redeployment, remove the %s annotation from the corresponding management cluster object",
			cluster.Namespace, cluster.Name, AppliedSystemAgentUpgraderHashAnnotation)
		return cluster, nil
	}

	// Limit the number of cluster to be processed simultaneously
	if installCounter.Load() >= int32(settings.SystemAgentUpgraderInstallConcurrency.GetInt()) {
		h.clusters.EnqueueAfter(cluster.Name, 5*time.Second)
		return cluster, nil
	}
	installCounter.Add(1)
	defer installCounter.Add(-1)

	// ensure SUC plan is installed
	apply, err := apply.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return cluster, err
	}
	err = apply.
		WithSetID("managed-system-agent").
		WithDynamicLookup().
		WithDefaultNamespace(namespaces.System).
		ApplyObjects(result...)
	if err != nil {
		return cluster, err
	}

	// Update the annotation with the latest hash value
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[AppliedSystemAgentUpgraderHashAnnotation] = hash

	return h.clusters.Update(cluster)
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

func installer(cluster *apimgmtv3.Cluster, secretName string) []runtime.Object {
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

	env = append(env, corev1.EnvVar{
		Name:  "CATTLE_ROLE_NONE",
		Value: "true",
	})

	// todo: data directory detection
	var plans []runtime.Object

	plan := upgradev1.NewPlan(namespaces.System, SystemAgentUpgraderPlanName, upgradev1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				UpgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
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
			ServiceAccountName: SystemAgentUpgraderServiceAccountName,
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
	})
	plans = append(plans, plan)

	windowsPlan := winsUpgradePlan(cluster, env, secretName)

	// todo: redeploy support
	plans = append(plans, windowsPlan)

	objs := []runtime.Object{
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SystemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgraderClusterRoleName,
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: SystemAgentUpgraderClusterRoleBindingName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      SystemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     SystemAgentUpgraderClusterRoleName,
			},
		},
	}

	return append(plans, objs...)
}

func winsUpgradePlan(cluster *apimgmtv3.Cluster, env []corev1.EnvVar, secretName string) *upgradev1.Plan {
	winsUpgradeImage := strings.SplitN(settings.WinsAgentUpgradeImage.Get(), ":", 2)
	winsVersion := "latest"
	if len(winsUpgradeImage) == 2 {
		winsVersion = winsUpgradeImage[1]
	}

	return upgradev1.NewPlan(namespaces.System, "system-agent-upgrader-windows", upgradev1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-agent-upgrader-windows",
			Namespace: namespaces.System,
			Annotations: map[string]string{
				UpgradeDigestAnnotation: "spec.upgrade.envs,spec.upgrade.envFrom",
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
			ServiceAccountName: SystemAgentUpgraderServiceAccountName,
			Upgrade: &upgradev1.ContainerSpec{
				Image: image.ResolveWithCluster(winsUpgradeImage[0], cluster),
				Env:   env,
				SecurityContext: &corev1.SecurityContext{
					WindowsOptions: &corev1.WindowsSecurityContextOptions{
						HostProcess:   ptr.To(true),
						RunAsUserName: ptr.To("NT AUTHORITY\\SYSTEM"),
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
	})
}
