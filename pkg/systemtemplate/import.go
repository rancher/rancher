package systemtemplate

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"text/template"

	"io"

	"crypto/sha256"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

var (
	t = template.Must(template.New("import").Parse(templateSource))
)

type context struct {
	CAChecksum string
	AgentImage string
	TokenKey   string
	Token      string
	URL        string
	URLPlain   string
}

func SystemTemplate(resp io.Writer, agentImage, token, url string) error {
	d := md5.Sum([]byte(token))
	tokenKey := hex.EncodeToString(d[:])[:7]

	context := &context{
		CAChecksum: CAChecksum(),
		AgentImage: agentImage,
		TokenKey:   tokenKey,
		Token:      base64.StdEncoding.EncodeToString([]byte(token)),
		URL:        base64.StdEncoding.EncodeToString([]byte(url)),
		URLPlain:   url,
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
