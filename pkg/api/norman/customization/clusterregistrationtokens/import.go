package clusterregistrationtokens

import (
	"net/http"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/namespace"
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

	// Parse filename to extract token and clusterId
	// Expected format: {token}_{clusterId}.yaml
	filename := req.PathValue("filename")
	if !strings.HasSuffix(filename, ".yaml") {
		resp.WriteHeader(404)
		resp.Write([]byte("not found"))
		return
	}

	// Remove .yaml suffix
	filenameWithoutExt := strings.TrimSuffix(filename, ".yaml")

	// Split by underscore to get token and clusterId
	parts := strings.SplitN(filenameWithoutExt, "_", 2)
	token := ""
	clusterID := ""

	if len(parts) >= 1 {
		token = parts[0]
	}
	if len(parts) >= 2 {
		clusterID = parts[1]
	}

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

	agentImage := image.ResolveWithCluster(settings.AgentImage.Get(), cluster)
	if err = systemtemplate.SystemTemplate(resp, agentImage, authImage, "", token, url,
		false, cluster, nil, nil, nil, false, namespace.GetMutator()); err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
	}
}
