package vsphere

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	UnknownFinder = iota
	SoapFinder
	ContentLibraryManager
	TagsManager
	CustomFieldsFinder
)

var fieldNames = map[string]int{
	"data-centers":      SoapFinder,
	"hosts":             SoapFinder,
	"data-stores":       SoapFinder,
	"folders":           SoapFinder,
	"networks":          SoapFinder,
	"virtual-machines":  SoapFinder,
	"templates":         SoapFinder,
	"clusters":          SoapFinder,
	"resource-pools":    SoapFinder,
	"content-libraries": ContentLibraryManager,
	"library-templates": ContentLibraryManager,
	"tags":              TagsManager,
	"tag-categories":    TagsManager,
	"fields":            CustomFieldsFinder,
}

var dataFields = map[string]string{
	"username": "vmwarevspherecredentialConfig-username",
	"password": "vmwarevspherecredentialConfig-password",
	"host":     "vmwarevspherecredentialConfig-vcenter",
	"port":     "vmwarevspherecredentialConfig-vcenterPort",
}

type Handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
}

func NewVsphereHandler(scaledContext *config.ScaledContext) *Handler {
	return &Handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
	}
}

func (v *Handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	fieldName := mux.Vars(req)["field"]
	dc := req.FormValue("dataCenter")

	if fieldName == "" || !validFieldName(fieldName) {
		util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: invalid field name", httperror.NotFound.Code))
		return
	}

	cc, err := v.getCloudCredential(req)
	if err != nil {
		util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
		return
	}

	var data []string
	if fieldNames[fieldName] == SoapFinder {
		data, err = soapLister(req.Context(), fieldName, cc, dc)
		if err != nil {
			util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
			return
		}
	}

	if fieldNames[fieldName] == ContentLibraryManager {
		l := req.FormValue("library")
		data, err = libraryLister(req.Context(), fieldName, cc, l)
		if err != nil {
			util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
			return
		}
	}

	if fieldNames[fieldName] == TagsManager {
		c := req.FormValue("category")
		data, err = tagsLister(req.Context(), fieldName, cc, c)
		if err != nil {
			util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
			return
		}
	}

	if fieldNames[fieldName] == CustomFieldsFinder {
		data, err = listCustomFields(req.Context(), cc)
		if err != nil {
			util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
			return
		}
	}

	js, err := json.Marshal(map[string][]string{
		"data": data,
	})

	if err != nil {
		util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: %s", httperror.ServerError.Code, err.Error()))
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write(js)
}

func (v *Handler) getCloudCredential(req *http.Request) (*corev1.Secret, error) {
	credID := req.FormValue("cloudCredentialId")
	if credID == "" {
		return nil, fmt.Errorf("cloud credential required")
	}

	var accessCred client.CloudCredential
	if err := access.ByID(v.generateAPIContext(req), &schema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status || apiError.Code.Status == httperror.NotFound.Status {
				return nil, fmt.Errorf("cloud credential not found")
			}
		}
		return nil, fmt.Errorf("error accessing cloud credential")
	}

	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		return nil, fmt.Errorf("invalid cloud credential %s", credID)
	}

	cc, err := v.secretsLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("error getting cloud cred %s: %v", credID, err)
	}

	if len(cc.Data) == 0 {
		return nil, fmt.Errorf("empty credential ID data %s", credID)
	}
	if !validCloudCredential(cc) {
		return nil, fmt.Errorf("not a valid vsphere credential %s", credID)
	}

	return cc, nil
}

func (v *Handler) generateAPIContext(req *http.Request) *types.APIContext {
	return &types.APIContext{
		Method:  req.Method,
		Request: req,
		Schemas: v.schemas,
		Query:   map[string][]string{},
	}
}

func validFieldName(s string) bool {
	if _, ok := fieldNames[s]; !ok {
		return false
	}

	return true
}

func validCloudCredential(cc *corev1.Secret) bool {
	for _, v := range dataFields {
		if _, ok := cc.Data[v]; !ok {
			return false
		}
	}
	return true
}
