package aks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/gorilla/mux"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
	AuthBaseURL      string `json:"authBaseUrl"`
	BaseURL          string `json:"baseUrl"`
	TenantID         string `json:"tenantId"`
	SubscriptionID   string `json:"subscriptionId"`
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	ResourceLocation string `json:"region"`
}

// AKS handler lists available resources in Azure API
type handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
	clusterLister v3.ClusterLister
	ac            types.AccessControl
}

func NewAKSHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
		clusterLister: scaledContext.Management.Clusters("").Controller().Lister(),
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
		if errCode, err := h.getCredentialsFromBody(req, capa); err != nil {
			handleErr(writer, errCode, err)
			return
		}
	} else {
		handleErr(writer, http.StatusBadRequest, fmt.Errorf("cannot access Azure API without credentials to authenticate"))
		return
	}

	var serialized []byte
	var errCode int
	var err error

	resourceType := mux.Vars(req)["resource"]

	switch resourceType {
	case "aksVersions":
		if serialized, errCode, err = listKubernetesVersions(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting kubernetes versions: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "aksVirtualNetworks":
		if serialized, errCode, err = listVirtualNetworks(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting networks: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "aksClusters":
		if serialized, errCode, err = listClusters(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting clusters: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "aksVMSizes":
		if serialized, errCode, err = listVMSizes(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting VM sizes: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "aksLocations":
		if serialized, errCode, err = listLocations(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting locations: %v", err)
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
		logrus.Errorf("[AKS] invalid cloud credential ID %s", credID)
		return http.StatusBadRequest, fmt.Errorf("invalid cloud credential ID %s", credID)
	}

	var accessCred client.CloudCredential //var to check access
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
		var clusterID string
		if clusterID = req.URL.Query().Get("clusterID"); clusterID == "" {
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
		}
		if errCode, err := h.clusterCheck(h.generateAPIContext(req), clusterID, credID); err != nil {
			return errCode, err
		}
	}

	cc, err := h.secretsLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("[AKS] error accessing cloud credential %s", credID)
		return httperror.InvalidBodyContent.Status, fmt.Errorf("error accessing cloud credential %s", credID)
	}
	cap.TenantID = string(cc.Data["azurecredentialConfig-tenantId"])
	cap.SubscriptionID = string(cc.Data["azurecredentialConfig-subscriptionId"])
	cap.ClientID = string(cc.Data["azurecredentialConfig-clientId"])
	cap.ClientSecret = string(cc.Data["azurecredentialConfig-clientSecret"])

	cap.BaseURL = req.URL.Query().Get("baseUrl")
	if cap.BaseURL == "" {
		cap.BaseURL = azure.PublicCloud.ResourceManagerEndpoint
	}
	cap.AuthBaseURL = req.URL.Query().Get("authBaseUrl")
	if cap.AuthBaseURL == "" {
		cap.AuthBaseURL = azure.PublicCloud.ActiveDirectoryEndpoint
	}
	region := req.URL.Query().Get("region")
	if region != "" {
		cap.ResourceLocation = region
	}

	return http.StatusOK, nil
}

func (h *handler) clusterCheck(apiContext *types.APIContext, clusterID, cloudCredentialID string) (int, error) {
	clusterInfo := map[string]interface{}{
		"id": clusterID,
	}

	clusterSchema := h.schemas.Schema(&mgmtSchema.Version, client.ClusterType)
	if err := h.ac.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, clusterInfo, clusterSchema); err != nil {
		return httperror.InvalidBodyContent.Status, fmt.Errorf("cluster not found")
	}

	cluster, err := h.clusterLister.Get("", clusterID)
	if err != nil {
		if httperror.IsNotFound(err) {
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cluster not found")
		}
		return httperror.ServerError.Status, err
	}

	if cluster.Spec.AKSConfig.AzureCredentialSecret != cloudCredentialID {
		return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
	}

	return http.StatusOK, nil
}

func (h *handler) getCredentialsFromBody(req *http.Request, cap *Capabilities) (int, error) {
	raw, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot read request body: %v", err)
	}

	if err = json.Unmarshal(raw, &cap); err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot parse request body: %v", err)
	}

	if cap.SubscriptionID == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid subscriptionId")
	}
	if cap.TenantID == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid tenantId")
	}
	if cap.ClientID == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid clientId")
	}
	if cap.ClientSecret == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid clientSecret")
	}
	if cap.BaseURL == "" {
		cap.BaseURL = azure.PublicCloud.ResourceManagerEndpoint
	}
	if cap.AuthBaseURL == "" {
		cap.AuthBaseURL = azure.PublicCloud.ActiveDirectoryEndpoint
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
