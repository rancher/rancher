package systemtemplate

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/features"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/templates"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

var (
	templateFuncMap = sprig.TxtFuncMap()
	t               = template.Must(template.New("import").Funcs(templateFuncMap).Parse(templateSource))
)

type context struct {
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
	IsWindowsCluster      bool
	IsPreBootstrap        bool
	IsRKE                 bool
	PrivateRegistryConfig string
	Tolerations           string
	AppendTolerations     string
	Affinity              string
	ResourceRequirements  string
	ClusterRegistry       string
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

func SystemTemplate(resp io.Writer, agentImage, authImage, namespace, token, url string, isWindowsCluster bool, isPreBootstrap bool,
	cluster *apimgmtv3.Cluster, features map[string]bool, taints []corev1.Taint, secretLister v1.SecretLister) error {
	var tolerations, agentEnvVars, agentAppendTolerations, agentAffinity, agentResourceRequirements string
	d := md5.Sum([]byte(url + token + namespace))
	tokenKey := hex.EncodeToString(d[:])[:7]

	if authImage == "fixed" {
		authImage = settings.AuthImage.Get()
	}

	registryURL, registryConfig, err := util.GeneratePrivateRegistryEncodedDockerConfig(cluster, secretLister)
	if err != nil {
		return err
	}

	if taints != nil {
		tolerations = templates.ToYAML(taints)
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

	agentEnvVars = templates.ToYAML(envVars)

	if appendTolerations := util.GetClusterAgentTolerations(cluster); appendTolerations != nil {
		agentAppendTolerations = templates.ToYAML(appendTolerations)
		if agentAppendTolerations == "" {
			return fmt.Errorf("error converting agent append tolerations to YAML")
		}
	}

	affinity, err := util.GetClusterAgentAffinity(cluster)
	if err != nil {
		return err
	}
	agentAffinity = templates.ToYAML(affinity)
	if agentAffinity == "" {
		return fmt.Errorf("error converting agent affinity to YAML")
	}

	if resourceRequirements := util.GetClusterAgentResourceRequirements(cluster); resourceRequirements != nil {
		agentResourceRequirements = templates.ToYAML(resourceRequirements)
		if agentResourceRequirements == "" {
			return fmt.Errorf("error converting agent resource requirements to YAML")
		}
	}

	context := &context{
		Features:              toFeatureString(features),
		CAChecksum:            CAChecksum(),
		AgentImage:            agentImage,
		AgentEnvVars:          agentEnvVars,
		AuthImage:             authImage,
		TokenKey:              tokenKey,
		Token:                 base64.StdEncoding.EncodeToString([]byte(token)),
		URL:                   base64.StdEncoding.EncodeToString([]byte(url)),
		Namespace:             base64.StdEncoding.EncodeToString([]byte(namespace)),
		URLPlain:              url,
		IsWindowsCluster:      isWindowsCluster,
		IsPreBootstrap:        isPreBootstrap,
		IsRKE:                 cluster != nil && cluster.Status.Driver == apimgmtv3.ClusterDriverRKE,
		PrivateRegistryConfig: registryConfig,
		Tolerations:           tolerations,
		AppendTolerations:     agentAppendTolerations,
		Affinity:              agentAffinity,
		ResourceRequirements:  agentResourceRequirements,
		ClusterRegistry:       registryURL,
	}

	return t.Execute(resp, context)
}

func GetDesiredFeatures(cluster *apimgmtv3.Cluster) map[string]bool {
	return map[string]bool{
		features.MCM.Name():                      false,
		features.MCMAgent.Name():                 true,
		features.Fleet.Name():                    false,
		features.RKE2.Name():                     false,
		features.ProvisioningV2.Name():           false,
		features.EmbeddedClusterAPI.Name():       false,
		features.UISQLCache.Name():               features.UISQLCache.Enabled(),
		features.ProvisioningPreBootstrap.Name(): capr.PreBootstrap(cluster),
	}
}

func ForCluster(cluster *apimgmtv3.Cluster, token string, taints []corev1.Taint, secretLister v1.SecretLister) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := SystemTemplate(buf, GetDesiredAgentImage(cluster),
		GetDesiredAuthImage(cluster),
		cluster.Name, token, settings.ServerURL.Get(),
		cluster.Spec.WindowsPreferedCluster, capr.PreBootstrap(cluster),
		cluster, GetDesiredFeatures(cluster), taints, secretLister)
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
