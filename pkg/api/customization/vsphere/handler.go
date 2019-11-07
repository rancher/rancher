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
	"data-centers":        SoapFinder,
	"hosts":               SoapFinder,
	"data-stores":         SoapFinder,
	"data-store-clusters": SoapFinder,
	"folders":             SoapFinder,
	"networks":            SoapFinder,
	"virtual-machines":    SoapFinder,
	"templates":           SoapFinder,
	"clusters":            SoapFinder,
	"resource-pools":      SoapFinder,
	"content-libraries":   ContentLibraryManager,
	"library-templates":   ContentLibraryManager,
	"tags":                TagsManager,
	"tag-categories":      TagsManager,
	"custom-attributes":   CustomFieldsFinder,
}

var dataFields = map[string]string{
	"username": "vmwarevspherecredentialConfig-username",
	"password": "vmwarevspherecredentialConfig-password",
	"host":     "vmwarevspherecredentialConfig-vcenter",
	"port":     "vmwarevspherecredentialConfig-vcenterPort",
}

type handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
}

func NewVsphereHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
	}
}

func (v *handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var err error

	fieldName := mux.Vars(req)["field"]
	dc := req.FormValue("dataCenter")

	if fieldName == "" || !validFieldName(fieldName) {
		util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: invalid field name", httperror.NotFound.Code))
		return
	}

	cc, errcode, err := v.getCloudCredential(req)
	if err != nil {
		util.ReturnHTTPError(res, req, errcode.Status, fmt.Sprintf("%s: %s", errcode.Code, err.Error()))
		return
	}

	var js []byte
	switch fieldNames[fieldName] {
	case SoapFinder:
		var data []string
		if data, err = processSoapFinder(req.Context(), fieldName, cc, dc); err != nil {
			invalidBody(res, req, err)
			return
		}
		js, err = json.Marshal(map[string][]string{"data": data})
	case ContentLibraryManager:
		l := req.FormValue("library")
		var data []string
		if data, err = processContentLibraryManager(req.Context(), fieldName, cc, l); err != nil {
			invalidBody(res, req, err)
			return
		}
		js, err = json.Marshal(map[string][]string{"data": data})
	case CustomFieldsFinder:
		var data []map[string]interface{}
		if data, err = processCustomFieldsFinder(req.Context(), cc); err != nil {
			invalidBody(res, req, err)
			return
		}
		js, err = json.Marshal(map[string][]map[string]interface{}{"data": data})
	case TagsManager:
		c := req.FormValue("category")
		var data []map[string]string
		if data, err = processTagsManager(req.Context(), fieldName, cc, c); err != nil {
			invalidBody(res, req, err)
			return
		}
		js, err = json.Marshal(map[string][]map[string]string{"data": data})
	}

	if err != nil {
		util.ReturnHTTPError(res, req, httperror.ServerError.Status, fmt.Sprintf("%s: %s", httperror.ServerError.Code, err.Error()))
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.Write(js)
}

func (v *handler) getCloudCredential(req *http.Request) (*corev1.Secret, httperror.ErrorCode, error) {
	credID := req.FormValue("cloudCredentialId")
	if credID == "" {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("cloudCredentialId required")
	}

	var accessCred client.CloudCredential //var to check access
	if err := access.ByID(v.generateAPIContext(req), &schema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status || apiError.Code.Status == httperror.NotFound.Status {
				return nil, httperror.NotFound, fmt.Errorf("cloud credential not found")
			}
		}
		return nil, httperror.NotFound, err
	}

	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("invalid cloud credential %s", credID)
	}

	cc, err := v.secretsLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("error getting cloud cred %s: %v", credID, err)
	}

	if len(cc.Data) == 0 {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("empty credential ID data %s", credID)
	}
	if !validCloudCredential(cc) {

		return nil, httperror.InvalidBodyContent, fmt.Errorf("not a valid vsphere credential %s", credID)
	}

	return cc, httperror.ErrorCode{}, nil
}

func (v *handler) generateAPIContext(req *http.Request) *types.APIContext {
	return &types.APIContext{
		Method:  req.Method,
		Request: req,
		Schemas: v.schemas,
		Query:   map[string][]string{},
	}
}

func invalidBody(res http.ResponseWriter, req *http.Request, err error) {
	util.ReturnHTTPError(res, req, httperror.InvalidBodyContent.Status, fmt.Sprintf("%s: %s", httperror.InvalidBodyContent.Code, err.Error()))
}

func validFieldName(s string) bool {
	_, ok := fieldNames[s]
	return ok
}

func validCloudCredential(cc *corev1.Secret) bool {
	for _, v := range dataFields {
		if _, ok := cc.Data[v]; !ok {
			return false
		}
	}
	return true
}
