package alibaba

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	cs "github.com/alibabacloud-go/cs-20151215/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v7/client"
	resourcemanager "github.com/alibabacloud-go/resourcemanager-20200331/v3/client"
	vpc "github.com/alibabacloud-go/vpc-20160428/v6/client"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"

	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"github.com/gorilla/mux"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/util"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	credentialsType = "access_key"
	defaultRegion   = "us-east-1"
	defaultProtocol = "https"
)

type Capabilities struct {
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	RegionId        string `json:"regionId"`
	AcceptLanguage  string `json:"acceptLanguage"`
}

type handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
}

func NewAlibabaHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
	}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	resourceType := mux.Vars(req)["resource"]

	if resourceType == "alibabaCheckCredentials" {
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

	capabilities := &Capabilities{}

	if credID := req.URL.Query().Get("cloudCredentialId"); credID != "" {
		cc, statusCode, err := h.getCloudCredential(req, credID)
		if err != nil {
			logrus.Debugf("[alibaba-handler] error accessing cloud credential %s:%s", credID, err.Error())
			handleErr(writer, statusCode, fmt.Errorf("error accessing cloud credential %s", credID))
			return
		}
		capabilities.AccessKeyID = string(cc.Data["alibabacredentialConfig-accessKeyId"])
		capabilities.AccessKeySecret = string(cc.Data["alibabacredentialConfig-accessKeySecret"])
	} else {
		handleErr(writer, http.StatusBadRequest, fmt.Errorf("cloud credential ID not found"))
		return
	}

	var (
		serialized []byte
		errCode    int
		err        error
	)

	regionId := req.URL.Query().Get("regionId")
	if regionId == "" {
		util.ReturnHTTPError(writer, req, http.StatusBadRequest, "regionId not set")
		return
	}
	capabilities.RegionId = regionId

	switch resourceType {
	case "alibabaRegions":
		if serialized, errCode, err = describeRegions(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeRegions: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaZones":
		if serialized, errCode, err = describeZones(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeZones: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaClusters":
		if serialized, errCode, err = describeClusters(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeClusters: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaResourceGroups":
		if serialized, errCode, err = describeResourceGroups(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeResourceGroups: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaInstanceTypes":
		if serialized, errCode, err = describeInstanceTypes(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeInstanceTypes: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaKeyPairs":
		if serialized, errCode, err = describeKeyPairs(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeKeyPairs: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaVpcs":
		if serialized, errCode, err = describeVpcs(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeVpcs: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaVSwitches":
		if serialized, errCode, err = describeVSwitches(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeVSwitches: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaAvailableResources":
		if serialized, errCode, err = describeAvailableResource(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeAvailableResource: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaKubernetesVersions":
		if serialized, errCode, err = describeKubernetesMetadata(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeKubernetesMetadata: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "alibabaImageSupportedInstanceTypes":
		if serialized, errCode, err = describeImageSupportedInstanceTypes(capabilities, req); err != nil {
			logrus.Debugf("[alibaba-handler] error call describeImageSupportedInstanceTypes: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	default:
		handleErr(writer, httperror.NotFound.Status, fmt.Errorf("invalid endpoint %v", resourceType))
	}
}

func (h *handler) checkCredentials(req *http.Request) (int, error) {
	cred := &Capabilities{}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot read request body: %v", err)
	}

	if err = json.Unmarshal(raw, &cred); err != nil {
		return http.StatusBadRequest, fmt.Errorf("cannot parse request body: %v", err)
	}

	if cred.RegionId == "" {
		cred.RegionId = defaultRegion
	}
	if cred.AccessKeyID == "" {
		return http.StatusBadRequest, fmt.Errorf("must provide access key ID")
	}
	if cred.AccessKeySecret == "" {
		return http.StatusBadRequest, fmt.Errorf("must provide access key secret")
	}

	client, err := CreateECSClient(cred.AccessKeyID, cred.AccessKeySecret, cred.RegionId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	request := &ecs.DescribeRegionsRequest{}
	if cred.AcceptLanguage != "" {
		request.AcceptLanguage = &cred.AcceptLanguage
	}

	_, err = client.DescribeRegions(request)
	if err != nil {

		logrus.Debugf("[alibaba-handler] error call describeRegions: %v", err)
		return handleSDKError(err)
	}

	return http.StatusOK, nil
}

func CreateCSClient(ak, sk, regionId string) (*cs.Client, error) {
	if regionId == "" {
		return nil, errors.New("regionId can not be empty")
	}

	credentials, err := getCredentials(ak, sk)
	if err != nil {
		return nil, err
	}

	protocol := defaultProtocol
	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   &protocol,
	}

	openAPICfg.Endpoint = tea.String("cs." + regionId + ".aliyuncs.com")
	return cs.NewClient(openAPICfg)
}

func CreateECSClient(ak, sk, regionId string) (*ecs.Client, error) {
	if regionId == "" {
		return nil, errors.New("regionId can not be empty")
	}

	credentials, err := getCredentials(ak, sk)
	if err != nil {
		return nil, err
	}

	protocol := defaultProtocol
	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   &protocol,
	}

	openAPICfg.Endpoint = tea.String("ecs." + regionId + ".aliyuncs.com")
	return ecs.NewClient(openAPICfg)
}

func CreateResourceManagerClient(ak, sk, regionId string) (*resourcemanager.Client, error) {
	if regionId == "" {
		return nil, errors.New("regionId can not be empty")
	}

	credentials, err := getCredentials(ak, sk)
	if err != nil {
		return nil, err
	}

	protocol := defaultProtocol
	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   &protocol,
	}

	openAPICfg.Endpoint = tea.String("resourcemanager." + regionId + ".aliyuncs.com")
	return resourcemanager.NewClient(openAPICfg)
}

func CreateVpcClient(ak, sk, regionId string) (*vpc.Client, error) {
	if regionId == "" {
		return nil, errors.New("regionId can not be empty")
	}

	credentials, err := getCredentials(ak, sk)
	if err != nil {
		return nil, err
	}

	protocol := defaultProtocol
	openAPICfg := &openapi.Config{
		Credential: credentials,
		Protocol:   &protocol,
	}

	openAPICfg.Endpoint = tea.String("vpc." + regionId + ".aliyuncs.com")
	return vpc.NewClient(openAPICfg)
}

func getCredentials(ak, sk string) (credential.Credential, error) {
	configCredsType := credentialsType

	config := &credential.Config{
		AccessKeyId:     &ak,
		AccessKeySecret: &sk,
		Type:            &configCredsType,
	}

	return credential.NewCredential(config)
}

func handleErr(writer http.ResponseWriter, errorCode int, originalErr error) {
	writer.WriteHeader(errorCode)

	payload := make(map[string]string)
	payload["error"] = originalErr.Error()
	payloadJSON, err := json.Marshal(payload)
	if err != nil { // This should not happen given fixed types on the payload - https://stackoverflow.com/a/33964549
		logrus.Errorf("[alibaba-handler] Failed to write payload JSON: %v", err)
		return
	}
	writer.Write(payloadJSON)
}

func (h *handler) generateAPIContext(req *http.Request) *types.APIContext {
	return &types.APIContext{
		Method:  req.Method,
		Request: req,
		Schemas: h.schemas,
		Query:   map[string][]string{},
	}
}

func (h *handler) getCloudCredential(req *http.Request, credID string) (*corev1.Secret, int, error) {
	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		logrus.Errorf("[alibaba-handler] invalid cloud credential ID %s", credID)
		return nil, http.StatusBadRequest, fmt.Errorf("invalid cloud credential ID %s", credID)
	}

	var accessCred client.CloudCredential //var to check access
	if err := access.ByID(h.generateAPIContext(req), &schema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
		apiError, ok := err.(*httperror.APIError)
		if !ok {
			return nil, httperror.NotFound.Status, err
		}
		if apiError.Code.Status == httperror.NotFound.Status {
			return nil, httperror.InvalidBodyContent.Status, fmt.Errorf("cloud credential not found")
		}
		if apiError.Code.Status != httperror.PermissionDenied.Status {
			return nil, httperror.InvalidBodyContent.Status, err
		}
	}

	cc, err := h.secretsLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("[alibaba-handler] error accessing cloud credential %s", credID)
		return nil, httperror.InvalidBodyContent.Status, fmt.Errorf("error accessing cloud credential %s", credID)
	}

	return cc, http.StatusOK, nil
}
