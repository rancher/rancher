package clusterregistrationtokens

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterImport struct {
	Clusters v3.ClusterInterface
}

func (ch *ClusterImport) ClusterImportHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain")
	token := mux.Vars(req)["token"]
	clusterID := mux.Vars(req)["clusterId"]

	urlBuilder, err := urlbuilder.New(req, schema.Version, types.NewSchemas())
	if err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
		return
	}
	url := settings.ServerURL.Get()
	if url == "" {
		url = urlBuilder.RelativeToRoot("")
	}

	authImage := ""
	authImages := req.URL.Query()["authImage"]
	if len(authImages) > 0 {
		authImage = authImages[0]
	}

	var cluster *apimgmtv3.Cluster
	if clusterID != "" {
		cluster, _ = ch.Clusters.Get(clusterID, metav1.GetOptions{})
	}

	if err = systemtemplate.SystemTemplate(resp, image.Resolve(settings.AgentImage.Get()), authImage, "", token, url,
		false, false, cluster, nil, nil, nil); err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
	}
}
