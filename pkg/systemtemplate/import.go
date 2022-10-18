package systemtemplate

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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
	IsRKE                 bool
	PrivateRegistryConfig string
	Tolerations           string
	ClusterRegistry       string
}

var (
	staticFeatures = features.MCM.Name() + "=false," +
		features.MCMAgent.Name() + "=true," +
		features.Fleet.Name() + "=false," +
		features.RKE2.Name() + "=false," +
		features.ProvisioningV2.Name() + "=false," +
		features.EmbeddedClusterAPI.Name() + "=false"
)

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

func SystemTemplate(resp io.Writer, agentImage, authImage, namespace, token, url string, isWindowsCluster bool,
	cluster *v3.Cluster, features map[string]bool, taints []corev1.Taint, privateRegistries *corev1.Secret) error {
	var tolerations, agentEnvVars string
	d := md5.Sum([]byte(url + token + namespace))
	tokenKey := hex.EncodeToString(d[:])[:7]

	if authImage == "fixed" {
		authImage = settings.AuthImage.Get()
	}

	privateRepo := util.GetPrivateRepo(cluster)
	privateRegistryConfig, err := util.GeneratePrivateRegistryDockerConfig(privateRepo, privateRegistries)
	if err != nil {
		return err
	}
	var clusterRegistry string
	if privateRepo != nil {
		clusterRegistry = privateRepo.URL
	}

	if taints != nil {
		tolerations = templates.ToYAML(taints)
	}

	envVars := settings.DefaultAgentSettingsAsEnvVars()
	if cluster != nil {
		envVars = append(envVars, cluster.Spec.AgentEnvVars...)
	}

	agentEnvVars = templates.ToYAML(envVars)

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
		IsRKE:                 cluster != nil && cluster.Status.Driver == apimgmtv3.ClusterDriverRKE,
		PrivateRegistryConfig: privateRegistryConfig,
		Tolerations:           tolerations,
		ClusterRegistry:       clusterRegistry,
	}

	return t.Execute(resp, context)
}

func GetDesiredFeatures(cluster *v3.Cluster) map[string]bool {
	return map[string]bool{
		features.MCM.Name():                false,
		features.MCMAgent.Name():           true,
		features.Fleet.Name():              false,
		features.RKE2.Name():               false,
		features.ProvisioningV2.Name():     false,
		features.EmbeddedClusterAPI.Name(): false,
		features.MonitoringV1.Name():       cluster.Spec.EnableClusterMonitoring,
	}
}

func ForCluster(cluster *v3.Cluster, token string, taints []corev1.Taint) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := SystemTemplate(buf, GetDesiredAgentImage(cluster),
		GetDesiredAuthImage(cluster),
		cluster.Name, token, settings.ServerURL.Get(), cluster.Spec.WindowsPreferedCluster,
		cluster, GetDesiredFeatures(cluster), taints, nil)
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

func GetDesiredAgentImage(cluster *v3.Cluster) string {
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

func GetDesiredAuthImage(cluster *v3.Cluster) string {
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
