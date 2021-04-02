package systemtemplate

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/templates"
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

func toFeatureString(features map[string]bool) string {
	buf := &strings.Builder{}
	for k, v := range features {
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
	cluster *v3.Cluster, features map[string]bool, taints []corev1.Taint) error {
	var tolerations, agentEnvVars string
	d := md5.Sum([]byte(url + token + namespace))
	tokenKey := hex.EncodeToString(d[:])[:7]

	if authImage == "fixed" {
		authImage = settings.AuthImage.Get()
	}

	privateRepo := util.GetPrivateRepo(cluster)
	privateRegistryConfig, err := util.GeneratePrivateRegistryDockerConfig(privateRepo)
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

	if cluster != nil && len(cluster.Spec.AgentEnvVars) > 0 {
		agentEnvVars = templates.ToYAML(cluster.Spec.AgentEnvVars)
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
		IsRKE:                 cluster != nil && cluster.Status.Driver == apimgmtv3.ClusterDriverRKE,
		PrivateRegistryConfig: privateRegistryConfig,
		Tolerations:           tolerations,
		ClusterRegistry:       clusterRegistry,
	}

	return t.Execute(resp, context)
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
