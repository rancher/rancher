package oci

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/util"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

type Credentials struct {
	Tenancy              string `json:"tenancyOCID"`
	User                 string `json:"userOCID"`
	Region               string `json:"region"`
	FingerPrint          string `json:"fingerprint"`
	PrivateKey           string `json:"privateKey"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
	Compartment          string `json:"compartmentOCID"`
}

// Cloud Credential Secret Fields
var requiredDataFields = map[string]string{
	"fingerprint":        "ocicredentialConfig-fingerprint",
	"tenancyId":          "ocicredentialConfig-tenancyId",
	"userId":             "ocicredentialConfig-userId",
	"privateKeyContents": "ocicredentialConfig-privateKeyContents",
}

type handler struct {
	Action        string
	schemas       *types.Schemas
	secretsLister v1.SecretLister
}

func NewOCIHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
	}
}

func (handler *handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {

	writer.Header().Set("Content-Type", "application/json")

	// New credential every invocation
	creds := Credentials{}
	errCode, err := handler.extractCreds(req, &creds)
	if err != nil {
		util.ReturnHTTPError(writer, req, errCode, err.Error())
		return
	}

	err = handler.validateCreds(&creds)
	if err != nil {
		util.ReturnHTTPError(writer, req, httperror.InvalidBodyContent.Status, err.Error())
		return
	}

	provider := common.NewRawConfigurationProvider(creds.Tenancy, creds.User, creds.Region, creds.FingerPrint,
		creds.PrivateKey, &creds.PrivateKeyPassphrase)

	resourceType := mux.Vars(req)["resource"]

	var serialized []byte

	switch resourceType {
	case "vcns":
		if serialized, errCode, err = processVcns(provider, creds.Compartment); err != nil {
			logrus.Debugf("[oci-handler] error processing VCNs: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "okeVersions":
		if serialized, errCode, err = processOkeVersions(provider); err != nil {
			logrus.Debugf("[oci-handler] error processing OKE versions: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "availabilityDomains":
		if serialized, errCode, err = processAvailabilityDomains(provider, creds.Compartment); err != nil {
			logrus.Debugf("[oci-handler] error processing ADs: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "regions":
		if serialized, errCode, err = processRegions(provider, creds.Tenancy); err != nil {
			logrus.Debugf("[oci-handler] error processing regions: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "nodeShapes":
		if serialized, errCode, err = processNodeShapes(provider, creds.Compartment); err != nil {
			logrus.Debugf("[oci-handler] error processing node shapes: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "nodeImages":
		if serialized, errCode, err = processImages(provider, creds.Compartment); err != nil {
			logrus.Debugf("[oci-handler] error processing images: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	case "nodeOkeImages":
		if serialized, errCode, err = processNodeOkeImages(provider); err != nil {
			logrus.Debugf("[oci-handler] error processing OKE images: %v", err)
			util.ReturnHTTPError(writer, req, errCode, err.Error())
			return
		}
		writer.Write(serialized)
	default:
		util.ReturnHTTPError(writer, req, httperror.NotFound.Status, "invalid endpoint "+resourceType)
	}
}

// extractCreds attempts to extract the credentials from a given request either from the body or cloud credentials.
func (handler *handler) extractCreds(req *http.Request, creds *Credentials) (int, error) {

	if credID := req.URL.Query().Get("cloudCredentialId"); credID != "" {
		ns, name := ref.Parse(credID)
		if ns == "" || name == "" {
			logrus.Debugf("[oci-handler] invalid cloud credential ID %s", credID)
			return httperror.InvalidBodyContent.Status, fmt.Errorf("invalid cloud credential ID %s", credID)
		}

		var accessCred client.CloudCredential //var to check access
		if err := access.ByID(handler.generateAPIContext(req), &schema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
			if apiError, ok := err.(*httperror.APIError); ok {
				if apiError.Code.Status == httperror.PermissionDenied.Status || apiError.Code.Status == httperror.NotFound.Status {
					return httperror.NotFound.Status, fmt.Errorf("cloud credential not found")
				}
			}
			return httperror.NotFound.Status, err
		}

		cc, err := handler.secretsLister.Get(namespace.GlobalNamespace, name)
		if err != nil {
			logrus.Debugf("[oci-handler] error accessing cloud credential %s", credID)
			return httperror.InvalidBodyContent.Status, fmt.Errorf("error accessing cloud credential %s", credID)
		}

		creds.Tenancy = string(cc.Data[requiredDataFields["tenancyId"]])
		creds.User = string(cc.Data[requiredDataFields["userId"]])
		creds.FingerPrint = string(cc.Data[requiredDataFields["fingerprint"]])
		creds.PrivateKey = string(cc.Data[requiredDataFields["privateKeyContents"]])
		creds.PrivateKeyPassphrase = string(cc.Data[requiredDataFields["privateKeyPassphrase"]])
		region := req.URL.Query().Get("region")
		if region != "" {
			creds.Region = region
		}
		compartment := req.URL.Query().Get("compartment")
		if compartment != "" {
			creds.Compartment = compartment
		}
	} else if req.Method == http.MethodPost {
		// Get credentials from body
		raw, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logrus.Debugf("[oci-handler] cannot read request body: " + err.Error())
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cannot read request body: " + err.Error())
		}

		err = json.Unmarshal(raw, &creds)
		if err != nil {
			logrus.Debugf("[oci-handler] cannot parse request body: " + err.Error())
			return httperror.InvalidBodyContent.Status, fmt.Errorf("cannot parse request body: " + err.Error())
		}
	} else {
		return httperror.Unauthorized.Status, fmt.Errorf("cannot access OCI API without credentials to authenticate")
	}

	return http.StatusOK, nil
}

// validateCreds validates that all the required credential fields are populated. Optional values may be defaulted.
func (handler *handler) validateCreds(creds *Credentials) error {

	// We can default these two if missing
	if creds.Compartment == "" {
		creds.Compartment = creds.Tenancy
	}
	if creds.Region == "" {
		// Default to PHX
		creds.Region = "us-phoenix-1"
	}

	// The rest are required
	if creds.PrivateKey == "" {
		logrus.Debugf("[oci-handler] OCI API private key is required")
		return fmt.Errorf("OCI API private key is required")
	}
	if creds.FingerPrint == "" {
		logrus.Debugf("[oci-handler] OCI fingerprint is required")
		return fmt.Errorf("OCI fingerprint is required")
	}
	if creds.Tenancy == "" {
		logrus.Debugf("[oci-handler] OCI tenancy is required")
		return fmt.Errorf("OCI tenancy is required")
	}

	if creds.User == "" {
		logrus.Debugf("[oci-handler] OCI user is required")
		return fmt.Errorf("OCI user is required")
	}

	return nil
}

func (handler *handler) generateAPIContext(req *http.Request) *types.APIContext {
	return &types.APIContext{
		Method:  req.Method,
		Request: req,
		Schemas: handler.schemas,
		Query:   map[string][]string{},
	}
}
