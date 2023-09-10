package gke

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/cluster"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	mgmtSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

type Capabilities struct {
	Credentials string `json:"credentials,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Zone        string `json:"zone,omitempty"`
	Region      string `json:"region,omitempty"`
}

// GKE handler lists available resources in Google API
type handler struct {
	Action        string
	schemas       *types.Schemas
	secretsLister v1.SecretLister
	clusterCache  mgmtv3.ClusterCache
	ac            types.AccessControl
}

func NewGKEHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
		clusterCache:  scaledContext.Wrangler.Mgmt.Cluster().Cache(),
		ac:            scaledContext.AccessControl,
	}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {

	writer.Header().Set("Content-Type", "application/json")

	capa := &Capabilities{}

	if credID := req.URL.Query().Get("cloudCredentialId"); credID != "" {
		if errCode, err := h.getCloudCredential(req, capa, credID); err != nil {
			handleErr(writer, errCode, err)
			return
		}
	} else if req.Method == http.MethodPost {
		if errCode, err := h.getCredentialsFromBody(writer, req, capa); err != nil {
			handleErr(writer, errCode, err)
			return
		}
	} else {
		handleErr(writer, http.StatusBadRequest, fmt.Errorf("cannot access Google API without credentials to authenticate"))
		return
	}

	var serialized []byte
	var errCode int
	var err error

	resourceType := mux.Vars(req)["resource"]

	switch resourceType {
	case "gkeMachineTypes":
		if serialized, errCode, err = listMachineTypes(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting machine types: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeNetworks":
		if serialized, errCode, err = listNetworks(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting networks: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeServiceAccounts":
		if serialized, errCode, err = listServiceAccounts(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting serviceaccounts: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeSubnetworks":
		if serialized, errCode, err = listSubnetworks(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting subnetworks: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeVersions":
		if serialized, errCode, err = listVersions(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting versions: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeZones":
		if serialized, errCode, err = listZones(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting zones: %v", err)
			handleErr(writer, errCode, err)
			return

		}
		writer.Write(serialized)
	case "gkeClusters":
		if serialized, errCode, err = listClusters(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting clusters: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "gkeSharedSubnets":
		if serialized, errCode, err = listSharedSubnets(req.Context(), capa); err != nil {
			logrus.Errorf("[gke-handler] error getting shared subnets: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	default:
		handleErr(writer, httperror.NotFound.Status, fmt.Errorf("invalid endpoint %v", resourceType))
	}
}

func (h *handler) getCloudCredential(req *http.Request, cap *Capabilities, credID string) (int, error) {
	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		logrus.Errorf("[GKE] invalid cloud credential ID %s", credID)
		return http.StatusBadRequest, fmt.Errorf("invalid cloud credential ID %s", credID)
	}

	var accessCred client.CloudCredential // var to check access
	if err := access.ByID(h.generateAPIContext(req), &schema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
		apiError, ok := err.(*httperror.APIError)
		if !ok {
			return httperror.NotFound.Status, err
		}
		if apiError.Code.Status == httperror.NotFound.Status {
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
		}
		if apiError.Code.Status != httperror.PermissionDenied.Status {
			return httperror.InvalidBodyContent.Status, err
		}
		if errCode, err := h.clusterCheck(h.generateAPIContext(req), req.URL.Query().Get("clusterID"), credID); err != nil {
			return errCode, err
		}
	}

	cc, err := h.secretsLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("[GKE] error accessing cloud credential %s", credID)
		return httperror.InvalidBodyContent.Status, fmt.Errorf("error accessing cloud credential %s", credID)
	}
	cap.Credentials = string(cc.Data["googlecredentialConfig-authEncodedJson"])

	cap.ProjectID = req.URL.Query().Get("projectId")
	if cap.ProjectID == "" {
		logrus.Errorf("[GKE] error getting projectId")
		return http.StatusBadRequest, fmt.Errorf("error getting projectId")
	}

	region := req.URL.Query().Get("region")
	if region != "" {
		cap.Region = region
	}
	zone := req.URL.Query().Get("zone")
	if zone != "" {
		cap.Zone = zone
	}

	return http.StatusOK, nil
}

func (h *handler) clusterCheck(apiContext *types.APIContext, clusterID, cloudCredentialID string) (int, error) {
	var (
		clusters []*v3.Cluster
		err      error
	)
	if clusterID == "" {
		// If no clusterID is passed, then we check all clusters that the user has access to and are associated to the cloud credential.
		clusters, err = h.clusterCache.GetByIndex(cluster.ByCloudCredential, cloudCredentialID)
		if err != nil {
			return httperror.InvalidBodyContent.Status, err
		}
		if len(clusters) == 0 {
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
		}
	} else {
		c, err := h.clusterCache.Get(clusterID)
		if err != nil {
			return httperror.ServerError.Status, err
		}
		clusters = []*v3.Cluster{c}
	}

	for _, c := range clusters {
		if c.Spec.GKEConfig == nil || c.Spec.GKEConfig.GoogleCredentialSecret != cloudCredentialID {
			continue
		}

		clusterSchema := h.schemas.Schema(&mgmtSchema.Version, client.ClusterType)
		if err := h.ac.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, map[string]interface{}{"id": c.Name}, clusterSchema); err == nil {
			return http.StatusOK, nil
		}

	}

	return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
}

func (h *handler) getCredentialsFromBody(writer http.ResponseWriter, req *http.Request, cap *Capabilities) (int, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot read request body: %v", err)
	}

	if err = json.Unmarshal(raw, &cap); err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot parse request body: %v", err)
	}

	if cap.Credentials == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid credentials")
	}
	if cap.ProjectID == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid projectId")
	}

	return http.StatusOK, nil
}

func (h *handler) generateAPIContext(req *http.Request) *types.APIContext {
	return &types.APIContext{
		Method:  req.Method,
		Request: req,
		Schemas: h.schemas,
		Query:   map[string][]string{},
	}
}

func handleErr(writer http.ResponseWriter, errorCode int, originalErr error) {
	asJSON := []byte(fmt.Sprintf(`{"error":"%v"}`, originalErr))

	writer.WriteHeader(errorCode)
	writer.Write(asJSON)
}
