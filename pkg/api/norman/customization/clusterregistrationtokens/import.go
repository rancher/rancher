package clusterregistrationtokens

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/docker/distribution/reference"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/tunnelserver/mcmauthorizer"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8scache "k8s.io/client-go/tools/cache"
)

func validateAuthImage(authImage string) error {
	if authImage == "" {
		return nil
	}
	_, err := reference.ParseNormalizedNamed(authImage)
	return err
}

type ClusterImport struct {
	Clusters     v3.ClusterInterface
	CRTIndexer   k8scache.Indexer
}

func (ch *ClusterImport) ClusterImportHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain")
	token := mux.Vars(req)["token"]
	clusterID := mux.Vars(req)["clusterId"]

	cluster, err := ch.Clusters.Get(clusterID, metav1.GetOptions{})
	if err != nil || cluster == nil {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("cluster not found or invalid token"))
		return
	}

	if !ch.isValidToken(clusterID, token) {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("cluster not found or invalid token"))
		return
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

	if err := validateAuthImage(authImage); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("invalid authImage - " + err.Error()))
		return
	}

	agentImage := image.ResolveWithCluster(settings.AgentImage.Get(), cluster)
	if err = systemtemplate.SystemTemplate(resp, agentImage, authImage, "", token, url,
		false, cluster, nil, nil, nil, false); err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
	}
}

func (ch *ClusterImport) isValidToken(clusterID, token string) bool {
	objs, err := ch.CRTIndexer.ByIndex(mcmauthorizer.CRTKeyIndex, token)
	if err != nil {
		logrus.Errorf("[cluster-registration-tokens] CRT index lookup failed: %v", err)
		return false
	}
	for _, obj := range objs {
		crt, ok := obj.(*v32.ClusterRegistrationToken)
		if ok && crt.Namespace == clusterID {
			return true
		}
	}
	return false
}
