package clusterregistrationtokens

import (
	"net/http"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/namespace"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/tunnelserver/mcmauthorizer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8scache "k8s.io/client-go/tools/cache"
)

type ClusterImport struct {
	Clusters        v3.ClusterInterface
	SecretLister    v1.SecretLister
	SecretIndexer   k8scache.Indexer
	NamespaceLister v1.NamespaceLister
}

func (ch *ClusterImport) ClusterImportHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain")

	// Parse filename to extract token and clusterId
	// Expected format: {token}_{clusterId}.yaml
	filename := req.PathValue("filename")
	filenameWithoutExt := strings.TrimSuffix(filename, ".yaml")
	parts := strings.SplitN(filenameWithoutExt, "_", 2)
	if len(parts) != 2 {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("invalid filename format, expected {token}_{clusterId}.yaml"))
		return
	}
	token := parts[0]
	clusterID := parts[1]

	cluster, err := ch.Clusters.Get(clusterID, metav1.GetOptions{})
	if err != nil || cluster == nil {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("cluster not found or invalid token"))
		return
	}

	if !ch.isValidToken(cluster, token) {
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
	assetsImage := image.ResolveWithCluster(settings.AssetsImage.Get(), cluster)
	ops := &systemtemplate.TemplateOps{
		AgentImage:     agentImage,
		AuthImage:      authImage,
		AssetsImage:    assetsImage,
		Namespace:      "",
		Token:          token,
		URL:            url,
		IsPreBootstrap: false,
		Cluster:        cluster,
		AgentFeatures:  systemtemplate.GetDesiredFeatures(cluster),
		Taints:         nil,
		SecretLister:   ch.SecretLister,
		PcExists:       false,
		Mutator:        namespace.GetMutator(),
	}
	if err = systemtemplate.SystemTemplate(resp, ops); err != nil {
		logrus.Errorf("[cluster-registration-tokens] failed to generate template: %v", err)
		resp.WriteHeader(500)
		resp.Write([]byte(err.Error()))
	}
}

func validateAuthImage(authImage string) error {
	if authImage == "" {
		return nil
	}
	_, err := reference.ParseNormalizedNamed(authImage)
	return err
}

// isValidToken reports whether token registers with cluster. The token is checked against cluster itself,
// the cluster the manifest is rendered for, and not against a cluster looked up by name again.
func (ch *ClusterImport) isValidToken(cluster *apimgmtv3.Cluster, token string) bool {
	clusterID := cluster.Name
	objs, err := ch.SecretIndexer.ByIndex(mcmauthorizer.SecretTokenIndex, token)
	if err != nil {
		logrus.Errorf("[cluster-registration-tokens] CRT token secret index lookup failed: %v", err)
		return false
	}
	for _, obj := range objs {
		secret, ok := obj.(*corev1.Secret)
		if !ok || secret.Namespace != clusterID {
			continue
		}
		validated, stale, err := clusterregistrationtoken.TokenSecretCluster(secret, ch.getNamespace, func(string) (*apimgmtv3.Cluster, error) {
			return cluster, nil
		})
		if err != nil {
			logrus.Errorf("[cluster-registration-tokens] %v", err)
			return false
		}
		if !stale && validated != nil {
			return true
		}
		logrus.Infof("[cluster-registration-tokens] rejecting registration token that belongs to a previous cluster named %s", clusterID)
	}
	return false
}

func (ch *ClusterImport) getNamespace(name string) (*corev1.Namespace, error) {
	return ch.NamespaceLister.Get("", name)
}
