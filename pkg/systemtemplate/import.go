package systemtemplate

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
	"text/template"

	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	t = template.Must(template.New("import").Parse(templateSource))
)

type context struct {
	Features              string
	CAChecksum            string
	AgentImage            string
	AuthImage             string
	TokenKey              string
	Token                 string
	URL                   string
	Namespace             string
	URLPlain              string
	IsWindowsCluster      bool
	PrivateRegistryConfig string
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
	cluster *v3.Cluster, features map[string]bool) error {
	d := md5.Sum([]byte(url + token + namespace))
	tokenKey := hex.EncodeToString(d[:])[:7]

	if authImage == "fixed" {
		authImage = settings.AuthImage.Get()
	}

	privateRegistryConfig, err := util.GeneratePrivateRegistryDockerConfig(util.GetPrivateRepo(cluster))
	if err != nil {
		return err
	}

	context := &context{
		Features:              toFeatureString(features),
		CAChecksum:            CAChecksum(),
		AgentImage:            agentImage,
		AuthImage:             authImage,
		TokenKey:              tokenKey,
		Token:                 base64.StdEncoding.EncodeToString([]byte(token)),
		URL:                   base64.StdEncoding.EncodeToString([]byte(url)),
		Namespace:             base64.StdEncoding.EncodeToString([]byte(namespace)),
		URLPlain:              url,
		IsWindowsCluster:      isWindowsCluster,
		PrivateRegistryConfig: privateRegistryConfig,
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
