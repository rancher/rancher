package systemtemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/features"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

var (
	templateFuncMap = sprig.TxtFuncMap()
	t               = template.Must(template.New("import").Funcs(templateFuncMap).Parse(templateSource))
	pct             = template.Must(template.New("priorityClass").Funcs(templateFuncMap).Parse(cattleClusterAgentPriorityClassTemplate))
	pdbt            = template.Must(template.New("podDisruptionBudget").Funcs(templateFuncMap).Parse(cattleClusterPodDisruptionBudgetTemplate))
)

type clusterAgentContext struct {
	Features              string
	CAChecksum            string
	AgentImage            string
	AgentEnvVars          string
	AuthImage             string
	TokenKey              string
	Token                 string
	URL                   string
	Namespace             string
	URLPlain              string
	IsPreBootstrap        bool
	PrivateRegistryConfig string
	Tolerations           string
	AppendTolerations     string
	Affinity              string
	ResourceRequirements  string
	ClusterRegistry       string
	EnablePriorityClass   bool
	PodDisruptionBudget   string
	SUCAppNameOverride    string
}

type priorityClassContext struct {
	PriorityClassValue int
	PreemptionPolicy   string
	Description        string
}

type podDisruptionBudgetContext struct {
	MinAvailable   string
	MaxUnavailable string
}

func toFeatureString(features map[string]bool) string {
	buf := &strings.Builder{}
	var keys []string
	for k := range features {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := features[k]
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k)
		if v {
			buf.WriteString("=true")
		} else {
			buf.WriteString("=false")
		}
	}
	return buf.String()
}

func PriorityClassTemplate(cluster *apimgmtv3.Cluster) ([]byte, error) {
	value, preemption := util.GetDesiredPriorityClassValueAndPreemption(cluster)

	pctx := priorityClassContext{
		PriorityClassValue: value,
		PreemptionPolicy:   preemption,
		Description:        util.PriorityClassDescription,
	}

	buf := &bytes.Buffer{}
	err := pct.Execute(buf, pctx)
	if err != nil {
		return nil, err
	}

	if buf.Len() == 0 {
		return nil, nil
	}

	return buf.Bytes(), nil
}

func PodDisruptionBudgetTemplate(cluster *apimgmtv3.Cluster) ([]byte, error) {
	minAvailable, maxUnavailable := util.GetDesiredPodDisruptionBudgetValues(cluster)

	pdbctx := podDisruptionBudgetContext{
		MinAvailable:   minAvailable,
		MaxUnavailable: maxUnavailable,
	}

	buf := &bytes.Buffer{}
	err := pdbt.Execute(buf, pdbctx)
	if err != nil {
		return nil, err
	}

	if buf.Len() == 0 {
		return nil, nil
	}

	return buf.Bytes(), nil
}

func SystemTemplate(resp io.Writer, agentImage, authImage, namespace, token, url string, isPreBootstrap bool,
	cluster *apimgmtv3.Cluster, agentFeatures map[string]bool, taints []corev1.Taint,
	secretLister v1.SecretLister, pcExists bool) error {
	var tolerations, agentEnvVars, agentAppendTolerations, agentAffinity, agentResourceRequirements string
	d := sha256.Sum256([]byte(fmt.Sprintf("%s.%s.%s", url, token, namespace)))
	tokenKey := hex.EncodeToString(d[:])[:10]

	if authImage == "fixed" {
		authImage = settings.AuthImage.Get()
	}

	registryURL, registryConfig, err := util.GeneratePrivateRegistryEncodedDockerConfig(cluster, secretLister)
	if err != nil {
		return err
	}

	if taints != nil {
		tolerationList := make([]corev1.Toleration, 0, len(taints))
		for _, taint := range taints {
			toleration := corev1.Toleration{
				Key:    taint.Key,
				Effect: taint.Effect,
			}

			if taint.Value == "" {
				toleration.Operator = corev1.TolerationOpExists
			} else {
				toleration.Operator = corev1.TolerationOpEqual
				toleration.Value = taint.Value
			}

			tolerationList = append(tolerationList, toleration)
		}
		tolerations = toYAML(tolerationList)
	}

	envVars := settings.DefaultAgentSettingsAsEnvVars()
	if cluster != nil {
		envVars = append(envVars, cluster.Spec.AgentEnvVars...)
	}

	// Merge the env vars with the AgentTLSModeStrict
	found := false
	for _, ev := range envVars {
		if ev.Name == "STRICT_VERIFY" {
			found = true // The user has specified `STRICT_VERIFY`, we should not attempt to overwrite it.
		}
	}
	if !found {
		if settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "true",
			})
		} else {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "false",
			})
		}
	}

	agentEnvVars = toYAML(envVars)

	if appendTolerations := util.GetClusterAgentTolerations(cluster); appendTolerations != nil {
		agentAppendTolerations = toYAML(appendTolerations)
		if agentAppendTolerations == "" {
			return fmt.Errorf("error converting agent append tolerations to YAML")
		}
	}

	affinity, err := util.GetClusterAgentAffinity(cluster)
	if err != nil {
		return err
	}
	agentAffinity = toYAML(affinity)
	if agentAffinity == "" {
		return fmt.Errorf("error converting agent affinity to YAML")
	}

	if resourceRequirements := util.GetClusterAgentResourceRequirements(cluster); resourceRequirements != nil {
		agentResourceRequirements = toYAML(resourceRequirements)
		if agentResourceRequirements == "" {
			return fmt.Errorf("error converting agent resource requirements to YAML")
		}
	}

	pcEnabled, pdbEnabled := util.AgentSchedulingCustomizationEnabled(cluster)

	var pdb string
	if pdbEnabled {
		pdbYaml, err := PodDisruptionBudgetTemplate(cluster)
		if err != nil {
			return err
		}
		pdb = string(pdbYaml)
	}

	context := &clusterAgentContext{
		Features:              toFeatureString(agentFeatures),
		CAChecksum:            CAChecksum(),
		AgentImage:            agentImage,
		AgentEnvVars:          agentEnvVars,
		AuthImage:             authImage,
		TokenKey:              tokenKey,
		Token:                 base64.StdEncoding.EncodeToString([]byte(token)),
		URL:                   base64.StdEncoding.EncodeToString([]byte(url)),
		Namespace:             base64.StdEncoding.EncodeToString([]byte(namespace)),
		URLPlain:              url,
		IsPreBootstrap:        isPreBootstrap,
		PrivateRegistryConfig: registryConfig,
		Tolerations:           tolerations,
		AppendTolerations:     agentAppendTolerations,
		Affinity:              agentAffinity,
		ResourceRequirements:  agentResourceRequirements,
		ClusterRegistry:       registryURL,
		PodDisruptionBudget:   pdb,
		EnablePriorityClass:   pcExists && pcEnabled,
		SUCAppNameOverride: func() string {
			// Set the field to ensure backward compatibility in the case of node-driver RKE2/K3s cluster
			if cluster.Status.Driver == apimgmtv3.ClusterDriverImported &&
				(cluster.Status.Provider == apimgmtv3.ClusterDriverRke2 || cluster.Status.Provider == apimgmtv3.ClusterDriverK3s) {
				if cluster.Spec.DisplayName != "" {
					return capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc",
						capr.SafeConcatName(48, cluster.Spec.DisplayName, "managed", "system-upgrade-controller"))
				}
			}
			return ""
		}(),
	}

	return t.Execute(resp, context)
}

func GetDesiredFeatures(cluster *apimgmtv3.Cluster) map[string]bool {
	enableMSUC := false
	if cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 || cluster.Status.Driver == apimgmtv3.ClusterDriverK3s {
		// the case of imported RKE2/K3s cluster
		enableMSUC = importedclusterversionmanagement.Enabled(cluster) && features.ManagedSystemUpgradeController.Enabled()
	}
	if cluster.Status.Driver == apimgmtv3.ClusterDriverImported &&
		(cluster.Status.Provider == apimgmtv3.ClusterDriverRke2 || cluster.Status.Provider == apimgmtv3.ClusterDriverK3s) {
		// the case of node-driver/custom RKE2/K3s cluster
		// The SUC app must be installed in order for Rancher to upgrade the cluster’s Kubernetes version.
		enableMSUC = true
	}
	return map[string]bool{
		features.MCM.Name():                            false,
		features.MCMAgent.Name():                       true,
		features.Fleet.Name():                          false,
		features.RKE2.Name():                           false,
		features.ProvisioningV2.Name():                 false,
		features.EmbeddedClusterAPI.Name():             false,
		features.Turtles.Name():                        false,
		features.UISQLCache.Name():                     features.UISQLCache.Enabled(),
		features.ProvisioningPreBootstrap.Name():       capr.PreBootstrap(cluster),
		features.ManagedSystemUpgradeController.Name(): enableMSUC,
	}
}

func ForCluster(cluster *apimgmtv3.Cluster, token string, taints []corev1.Taint, secretLister v1.SecretLister) ([]byte, error) {

	status := util.GetAgentSchedulingCustomizationStatus(cluster)
	pcExists := status != nil && status.PriorityClass != nil

	buf := &bytes.Buffer{}
	err := SystemTemplate(buf, GetDesiredAgentImage(cluster), GetDesiredAuthImage(cluster),
		cluster.Name, token, settings.ServerURL.Get(), capr.PreBootstrap(cluster), cluster, GetDesiredFeatures(cluster), taints, secretLister, pcExists)
	return buf.Bytes(), err
}

func InternalCAChecksum() string {
	ca := settings.InternalCACerts.Get()
	if ca != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		digest := sha256.Sum256([]byte(ca))
		return hex.EncodeToString(digest[:])
	}
	return ""
}

func CAChecksum() string {
	ca := settings.CACerts.Get()
	if ca != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		digest := sha256.Sum256([]byte(ca))
		return hex.EncodeToString(digest[:])
	}
	return ""
}

func GetDesiredAgentImage(cluster *apimgmtv3.Cluster) string {
	logrus.Tracef("clusterDeploy: deployAgent called for [%s]", cluster.Name)
	desiredAgent := cluster.Spec.DesiredAgentImage
	if cluster.Spec.AgentImageOverride != "" {
		desiredAgent = cluster.Spec.AgentImageOverride
	}
	if desiredAgent == "" || desiredAgent == "fixed" {
		desiredAgent = image.ResolveWithCluster(settings.AgentImage.Get(), cluster)
	}
	logrus.Tracef("clusterDeploy: deployAgent: desiredAgent is [%s] for cluster [%s]", desiredAgent, cluster.Name)
	return desiredAgent
}

func GetDesiredAuthImage(cluster *apimgmtv3.Cluster) string {
	var desiredAuth string
	if cluster.Spec.LocalClusterAuthEndpoint.Enabled {
		desiredAuth = cluster.Spec.DesiredAuthImage
		if desiredAuth == "" || desiredAuth == "fixed" {
			desiredAuth = image.ResolveWithCluster(settings.AuthImage.Get(), cluster)
		}
	}
	logrus.Tracef("clusterDeploy: deployAgent: desiredAuth is [%s] for cluster [%s]", desiredAuth, cluster.Name)
	return desiredAuth
}

func toYAML(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template so it doesn't affect remaining template lines
		logrus.Errorf("[ToYAML] Error marshaling %v: %v", v, err)
		return ""
	}
	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		// Swallow errors inside of a template so it doesn't affect remaining template lines
		logrus.Errorf("[ToYAML] Error converting json to yaml for %v: %v ", string(data), err)
		return ""
	}
	return strings.TrimSuffix(string(yamlData), "\n")
}
