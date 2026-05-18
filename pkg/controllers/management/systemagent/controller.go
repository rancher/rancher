package systemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	planv1alpha1 "github.com/rancher/rancher/pkg/apis/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	plancontrollers "github.com/rancher/rancher/pkg/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/image"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
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

var (
	// installCounter keeps track of the number of clusters for which the handler is concurrently installing or upgrading
	// the resources needed for upgrading system-agent.
	installCounter atomic.Int32
)

type handler struct {
	ctx         context.Context
	manager     *clustermanager.Manager
	clusters    mgmtcontrollers.ClusterController
	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache
}

func Register(ctx context.Context, w *wrangler.Context, manager *clustermanager.Manager) {
	h := &handler{
		ctx:         ctx,
		manager:     manager,
		clusters:    w.Mgmt.Cluster(),
		beacons:     w.Plan.Beacon(),
		beaconCache: w.Plan.Beacon().Cache(),
	}
	if features.ImportedDay2Ops.Enabled() {
		w.Mgmt.Cluster().OnChange(ctx, "imported-system-agent-setup", h.onChange)
	}
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if cluster.Name == "local" {
		return cluster, nil
	}

	// only applies to imported RKE2/K3s cluster
	if cluster.Status.Driver != apimgmtv3.ClusterDriverK3s && cluster.Status.Driver != apimgmtv3.ClusterDriverRke2 {
		return cluster, nil
	}

	if cluster.Annotations[Day2OpsEnabledAnnotation] == "" {
		if settings.ImportedClusterDay2OpsEnabledDefault.Get() == "true" {
			cluster := cluster.DeepCopy()
			if cluster.Annotations == nil {
				cluster.Annotations = map[string]string{}
			}
			cluster.Annotations[Day2OpsEnabledAnnotation] = "true"
			return h.clusters.Update(cluster)
		}
		return cluster, nil
	} else if cluster.Annotations[Day2OpsEnabledAnnotation] != "true" {
		return cluster, nil
	}

	_, err := h.beaconCache.Get(cluster.Name, cluster.Name)
	if apierrors.IsNotFound(err) {
		_, err = h.beacons.Create(&planv1alpha1.Beacon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name,
				Namespace: cluster.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Name:       cluster.Name,
						UID:        cluster.UID,
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
	if _, err := h.clusters.Update(cluster); err != nil {
		return cluster, fmt.Errorf("failed to update annotation: %w", err)
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
