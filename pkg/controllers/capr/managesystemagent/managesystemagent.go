package managesystemagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/gvk"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	generationSecretName                  = "system-agent-upgrade-generation"
	upgradeAPIVersion                     = "upgrade.cattle.io/v1"
	upgradeDigestAnnotation               = "upgrade.cattle.io/digest"
	systemAgentUpgraderServiceAccountName = "system-agent-upgrader"
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
)

type handler struct {
	clusterRegistrationTokens v3.ClusterRegistrationTokenCache
	bundles                   fleetcontrollers.BundleClient
	provClusters              rocontrollers.ClusterCache
	controlPlanes             v1.RKEControlPlaneCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterRegistrationTokens: clients.Mgmt.ClusterRegistrationToken().Cache(),
		bundles:                   clients.Fleet.Bundle(),
		provClusters:              clients.Provisioning.Cluster().Cache(),
		controlPlanes:             clients.RKE.RKEControlPlane().Cache(),
	}

	v1.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "monitor-system-upgrade-controller-readiness", h.syncSystemUpgradeControllerStatus)

	rocontrollers.RegisterClusterGeneratingHandler(ctx, clients.Provisioning.Cluster(),
		clients.Apply.
			WithSetOwnerReference(false, false).
			WithCacheTypes(clients.Fleet.Bundle(),
				clients.Provisioning.Cluster(),
				clients.Core.Secret(),
				clients.RBAC.RoleBinding(),
				clients.RBAC.Role(),
				clients.RKE.RKEControlPlane()),
		"", "manage-system-agent", h.OnChange, &generic.GeneratingHandlerOptions{
			AllowCrossNamespace: true,
		})

	rocontrollers.RegisterClusterGeneratingHandler(ctx, clients.Provisioning.Cluster(),
		clients.Apply.
			WithSetOwnerReference(false, false).
			WithCacheTypes(clients.Mgmt.ManagedChart(),
				clients.Provisioning.Cluster()),
		"", "manage-system-upgrade-controller", h.OnChangeInstallSUC, nil)
}

func (h *handler) OnChange(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if cluster.Spec.RKEConfig == nil || settings.SystemAgentUpgradeImage.Get() == "" {
		return nil, status, nil
	}

	// Intentionally return an ErrSkip to prevent unnecessarily thrashing the corresponding bundle
	// if the status field for fleetworkspacename has not yet been updated
	if cluster.Status.FleetWorkspaceName == "" {
		return nil, status, generic.ErrSkip
	}

	var (
		secretName = "stv-aggregation"
		result     []runtime.Object
	)

	if cluster.Status.ClusterName == "local" && cluster.Namespace == fleetconst.ClustersLocalNamespace {
		secretName += "-local-"

		token, err := h.clusterRegistrationTokens.Get(cluster.Status.ClusterName, "default-token")
		if err != nil {
			return nil, status, err
		}
		if token.Status.Token == "" {
			return nil, status, fmt.Errorf("token not yet generated for %s/%s", token.Namespace, token.Name)
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

	cp, err := h.controlPlanes.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		logrus.Errorf("Error encountered getting RKE control plane while determining SUC readiness: %v", err)
		return nil, status, err
	}

	if !capr.SystemUpgradeControllerReady.IsTrue(cp) {
		// If the SUC is not ready do not create any plans, as those
		// plans may depend on functionality only a newer version of the SUC contains
		logrus.Debugf("[managesystemagent] the SUC is not yet ready, waiting to create system agent upgrade plans (SUC status: %s)", capr.SystemUpgradeControllerReady.GetStatus(cp))
		return nil, status, generic.ErrSkip
	}

	resources, err := ToResources(installer(cluster, secretName))
	if err != nil {
		return nil, status, err
	}

	result = append(result, &v1alpha1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Status.FleetWorkspaceName,
			Name:      capr.SafeConcatName(capr.MaxHelmReleaseNameLength, cluster.Name, "managed", "system", "agent"),
		},
		Spec: v1alpha1.BundleSpec{
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{
				DefaultNamespace: namespaces.System,
				// In the event that a controller updates the SUC Plan at the same time as
				// fleet is attempting to update the plan via the bundle, we may end up with drift.
				CorrectDrift: &v1alpha1.CorrectDrift{
					Enabled: true,
				},
			},
			Resources: resources,
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
	})

	return result, status, nil
}

func installer(cluster *rancherv1.Cluster, secretName string) []runtime.Object {
	upgradeImage := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := "latest"
	if len(upgradeImage) == 2 {
		version = upgradeImage[1]
	}

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
			Value: capr.GetSystemAgentDataDir(&cluster.Spec.RKEConfig.RKEClusterSpecCommon),
		})
	}

	var plans []runtime.Object

	plan := &upgradev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: upgradeAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      systemAgentUpgraderServiceAccountName,
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
			ServiceAccountName: systemAgentUpgraderServiceAccountName,
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

	if currentVersionResolvesGH5551(cluster.Spec.KubernetesVersion) {
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
				Name:      systemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: systemAgentUpgraderServiceAccountName,
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: systemAgentUpgraderServiceAccountName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      systemAgentUpgraderServiceAccountName,
				Namespace: namespaces.System,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     systemAgentUpgraderServiceAccountName,
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
			ServiceAccountName: systemAgentUpgraderServiceAccountName,
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

// currentVersionResolvesGH5551 determines if the given rke2 version
// has fixed the RKE2 bug outlined in GH-5551. Windows SUC plans cannot be delivered
// to clusters running versions containing this bug. This function can be removed
// when v1.31.x is the lowest supported version offered by Rancher.
func currentVersionResolvesGH5551(version string) bool {

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

func ToResources(objs []runtime.Object) (result []v1alpha1.BundleResource, err error) {
	for _, obj := range objs {
		obj = obj.DeepCopyObject()
		if err := gvk.Set(obj); err != nil {
			return nil, fmt.Errorf("failed to set gvk: %w", err)
		}

		typeMeta, err := meta.TypeAccessor(obj)
		if err != nil {
			return nil, err
		}

		meta, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}

		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}

		digest := sha256.Sum256(data)
		filename := name.SafeConcatName(typeMeta.GetKind(), meta.GetNamespace(), meta.GetName(), hex.EncodeToString(digest[:])[:12]) + ".yaml"
		result = append(result, v1alpha1.BundleResource{
			Name:    filename,
			Content: string(data),
		})
	}
	return
}
