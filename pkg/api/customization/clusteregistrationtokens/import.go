package clusteregistrationtokens

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
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

func ClusterImportHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain")
	token := mux.Vars(req)["token"]

	d := md5.Sum([]byte(token))
	tokenKey := hex.EncodeToString(d[:])[:7]

	urlBuilder, err := urlbuilder.New(req, schema.Version, types.NewSchemas())
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
		return
	}

	context := &context{
		CAChecksum: CAChecksum(),
		AgentImage: settings.AgentImage.Get(),
		TokenKey:   tokenKey,
		Token:      base64.StdEncoding.EncodeToString([]byte(token)),
		URL:        base64.StdEncoding.EncodeToString([]byte(urlBuilder.RelativeToRoot(""))),
		URLPlain:   urlBuilder.RelativeToRoot(""),
	}

	err = t.Execute(resp, context)
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
	}
}
