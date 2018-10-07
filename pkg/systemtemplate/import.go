package systemtemplate

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"text/template"

	"io"

	"crypto/sha256"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

var (
	t = template.Must(template.New("import").Parse(templateSource))
)

type context struct {
	CAChecksum                string
	AgentImage                string
	TokenKey                  string
	Token                     string
	URL                       string
	URLPlain                  string
	TolerationsOfClusterAgent []v1.Toleration
	TolerationsOfNodeAgent    []v1.Toleration
}

func SystemTemplate(resp io.Writer, agentImage, token, url string) error {
	d := md5.Sum([]byte(token))
	tokenKey := hex.EncodeToString(d[:])[:7]

	context := &context{
		CAChecksum:                CAChecksum(),
		AgentImage:                agentImage,
		TokenKey:                  tokenKey,
		Token:                     base64.StdEncoding.EncodeToString([]byte(token)),
		URL:                       base64.StdEncoding.EncodeToString([]byte(url)),
		URLPlain:                  url,
		TolerationsOfClusterAgent: TolerationsOfClusterAgent(),
		TolerationsOfNodeAgent:    TolerationsOfNodeAgent(),
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

func TolerationsOfClusterAgent() []v1.Toleration {
	var tolerationsOfClusterAgents []v1.Toleration
	tolerationsOfClusterAgentJSON := settings.TolerationsOfClusterAgent.Get()
	if tolerationsOfClusterAgentJSON != "" {
		err := json.Unmarshal([]byte(tolerationsOfClusterAgentJSON), &tolerationsOfClusterAgents)
		if err != nil {
			logrus.Errorf("Failed to prase tolerations-of-clusteragent in settings, err: %v", err)
			return nil
		}
		return tolerationsOfClusterAgents
	}
	return nil
}

func TolerationsOfNodeAgent() []v1.Toleration {
	var tolerationsOfNodeAgents []v1.Toleration
	tolerationsOfNodeAgentJSON := settings.TolerationsOfNodeAgent.Get()
	if tolerationsOfNodeAgentJSON != "" {
		err := json.Unmarshal([]byte(tolerationsOfNodeAgentJSON), &tolerationsOfNodeAgents)
		if err != nil {
			logrus.Errorf("Failed to prase tolerations-of-nodeagent in settings, err: %v", err)
			return nil
		}
		return tolerationsOfNodeAgents
	}
	return nil
}
