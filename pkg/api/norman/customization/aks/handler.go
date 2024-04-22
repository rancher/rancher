package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/gorilla/mux"
	"github.com/rancher/aks-operator/pkg/aks"
	"github.com/rancher/machine/drivers/azure/azureutil"
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

const (
	tenantIDAnnotation          = "cluster.management.cattle.io/azure-tenant-id"
	tenantIDTimestampAnnotation = "cluster.management.cattle.io/azure-tenant-id-created-at"
	tenantIDTimeout             = time.Hour
)

type Capabilities struct {
	AuthBaseURL      string `json:"authBaseUrl"`
	BaseURL          string `json:"baseUrl"`
	TenantID         string `json:"tenantId"`
	SubscriptionID   string `json:"subscriptionId"`
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	ResourceLocation string `json:"region"`
	Environment      string `json:"environment"`
	ClusterID        string `json:"clusterId"`
}

// AKS handler lists available resources in Azure API
type handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
	clusterCache  mgmtv3.ClusterCache
	ac            types.AccessControl
	secretClient  v1.SecretInterface
}

func NewAKSHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
		clusterCache:  scaledContext.Wrangler.Mgmt.Cluster().Cache(),
		ac:            scaledContext.AccessControl,
		secretClient:  scaledContext.Core.Secrets(namespace.GlobalNamespace),
	}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	resourceType := mux.Vars(req)["resource"]

	if resourceType == "aksCheckCredentials" {
		if req.Method != http.MethodPost {
			handleErr(writer, http.StatusMethodNotAllowed, fmt.Errorf("use POST for this endpoint"))
			return
		}
		if errCode, err := h.checkCredentials(req); err != nil {
			handleErr(writer, errCode, err)
			return
		}
		return
	}

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

	switch resourceType {
	case "aksUpgrades":
		if serialized, errCode, err = listKubernetesUpgradeVersions(req.Context(), h.clusterCache, capa); err != nil {
			logrus.Errorf("[aks-handler] error getting kubernetes upgrade versions: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
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
		if serialized, errCode, err = listVMSizesV1(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting VM sizes: %v", err)
			handleErr(writer, errCode, err)
			return
		}
		writer.Write(serialized)
	case "aksVMSizesV2":
		if serialized, errCode, err = listVMSizesV2(req.Context(), capa); err != nil {
			logrus.Errorf("[aks-handler] error getting VM sizes (v2): %v", err)
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

func (h *handler) checkCredentials(req *http.Request) (int, error) {
	cred := &Capabilities{}
	raw, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot read request body: %v", err)
	}

	if err = json.Unmarshal(raw, &cred); err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot parse request body: %v", err)
	}

	if cred.SubscriptionID == "" {
		return http.StatusBadRequest, fmt.Errorf("must provide subscriptionId")
	}
	if cred.ClientID == "" {
		return http.StatusBadRequest, fmt.Errorf("must provide clientId")
	}
	if cred.ClientSecret == "" {
		return http.StatusBadRequest, fmt.Errorf("must provide clientSecret")
	}

	clientEnvironment := ""
	if cred.Environment != "" {
		clientEnvironment = cred.Environment
	}
	_, azureEnvironment := GetEnvironment(clientEnvironment)

	cred.BaseURL = azureEnvironment.ResourceManagerEndpoint
	cred.AuthBaseURL = azureEnvironment.ActiveDirectoryEndpoint

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if cred.TenantID == "" {
		cred.TenantID, err = azureutil.FindTenantID(ctx, azureEnvironment, cred.SubscriptionID)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("could not find tenant ID for Azure environment %s: %w", azureEnvironment.Name, err)
		}
	}

	client, err := NewSubscriptionServiceClient(cred)
	if err != nil {
		logrus.Errorf("[AKS] failed to create new subscription client: %v", err)
		return http.StatusUnauthorized, fmt.Errorf("invalid credentials: %w", err)
	}
	_, err = client.Get(ctx, cred.SubscriptionID, nil)
	if err != nil {
		logrus.Errorf("[AKS] failed to get subscription details: %v", err)
		return http.StatusUnauthorized, fmt.Errorf("invalid credentials: %w", err)
	}

	return http.StatusOK, nil
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
	cap.Environment = string(cc.Data["azurecredentialConfig-environment"])

	clientEnvironment := ""
	if cap.Environment != "" {
		clientEnvironment = cap.Environment
	}
	_, azureEnvironment := GetEnvironment(clientEnvironment)

	if cap.TenantID == "" {
		cap.TenantID, err = aks.GetCachedTenantID(h.secretClient, cap.SubscriptionID, cc)
		if err != nil {
			return httperror.ServerError.Status, err
		}
	}

	cap.BaseURL = req.URL.Query().Get("baseUrl")
	if cap.BaseURL == "" {
		cap.BaseURL = azureEnvironment.ResourceManagerEndpoint
	}
	cap.AuthBaseURL = req.URL.Query().Get("authBaseUrl")
	if cap.AuthBaseURL == "" {
		cap.AuthBaseURL = azureEnvironment.ActiveDirectoryEndpoint
	}

	cap.ResourceLocation = req.URL.Query().Get("region")
	cap.ClusterID = req.URL.Query().Get("clusterId")

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
		if c.Spec.AKSConfig == nil || c.Spec.AKSConfig.AzureCredentialSecret != cloudCredentialID {
			continue
		}

		clusterSchema := h.schemas.Schema(&mgmtSchema.Version, client.ClusterType)
		if err := h.ac.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, map[string]interface{}{"id": c.Name}, clusterSchema); err == nil {
			return http.StatusOK, nil
		}
	}

	return httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
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

	clientEnvironment := ""
	if cap.Environment != "" {
		clientEnvironment = cap.Environment
	}
	_, azureEnvironment := GetEnvironment(clientEnvironment)

	if cap.BaseURL == "" {
		cap.BaseURL = azureEnvironment.ResourceManagerEndpoint
	}
	if cap.AuthBaseURL == "" {
		cap.AuthBaseURL = azureEnvironment.ActiveDirectoryEndpoint
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
	writer.WriteHeader(errorCode)

	payload := make(map[string]string)
	payload["error"] = originalErr.Error()
	payloadJSON, err := json.Marshal(payload)
	if err != nil { // This should not happen given fixed types on the payload - https://stackoverflow.com/a/33964549
		logrus.Errorf("[AKS] Failed to write payload JSON: %v", err)
		return
	}
	writer.Write(payloadJSON)
}

func GetEnvironment(env string) (cloud.Configuration, azure.Environment) {
	switch env {
	case "AzureChinaCloud":
		return cloud.AzureChina, azure.ChinaCloud
	case "AzureUSGovernmentCloud":
		return cloud.AzureGovernment, azure.USGovernmentCloud
	default:
		return cloud.AzurePublic, azure.PublicCloud
	}
}
