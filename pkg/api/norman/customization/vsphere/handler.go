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
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
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

var cloudCredentialDataPrefix = "vmwarevspherecredentialConfig-"
var dataFields = map[string]string{
	"username": "username",
	"password": "password",
	"host":     "vcenter",
	"port":     "vcenterPort",
}

type handler struct {
	schemas       *types.Schemas
	secretsLister v1.SecretLister
	ac            types.AccessControl
}

func NewVsphereHandler(scaledContext *config.ScaledContext) http.Handler {
	return &handler{
		schemas:       scaledContext.Schemas,
		secretsLister: scaledContext.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
		ac:            scaledContext.AccessControl,
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

	var cc *v1.Secret
	var errcode httperror.ErrorCode

	if id := req.FormValue("cloudCredentialId"); id != "" {
		cc, errcode, err = v.getCloudCredential(id, req)
		if err != nil {
			util.ReturnHTTPError(res, req, errcode.Status, fmt.Sprintf("%s: %s", errcode.Code, err.Error()))
			return
		}
	} else if id := req.FormValue("secretId"); id != "" {
		cc, errcode, err = v.getSecret(id, req)
		if err != nil {
			util.ReturnHTTPError(res, req, errcode.Status, fmt.Sprintf("%s: %s", errcode.Code, err.Error()))
			return
		}
	}

	if cc == nil {
		util.ReturnHTTPError(res, req, httperror.NotFound.Status, fmt.Sprintf("%s: cloud credential not found", httperror.NotFound.Code))
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

func (v *handler) getCloudCredential(id string, req *http.Request) (*corev1.Secret, httperror.ErrorCode, error) {
	var accessCred client.CloudCredential //var to check access
	if err := access.ByID(v.generateAPIContext(req), &schema.Version, client.CloudCredentialType, id, &accessCred); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status || apiError.Code.Status == httperror.NotFound.Status {
				return nil, httperror.NotFound, fmt.Errorf("cloud credential not found")
			}
		}
		return nil, httperror.NotFound, err
	}

	ns, name := ref.Parse(id)
	if ns == "" || name == "" {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("invalid cloud credential %s", id)
	}

	cc, err := v.secretsLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("error getting cloud cred %s: %v", id, err)
	}

	if len(cc.Data) == 0 {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("empty credential ID data %s", id)
	}
	if !validCloudCredential(cc) {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("not a valid vsphere credential %s", id)
	}

	return cc, httperror.ErrorCode{}, nil
}

func (v *handler) getSecret(id string, req *http.Request) (*corev1.Secret, httperror.ErrorCode, error) {
	defaultNamespace := settings.FleetDefaultWorkspaceName.Get()

	secretState := map[string]interface{}{
		"name":        id,
		"id":          id,
		"namespaceId": defaultNamespace,
	}
	schema := types.Schema{ID: "secrets"}

	if err := v.ac.CanDo(v1.SecretGroupVersionKind.Group, v1.SecretResource.Name,
		"get", v.generateAPIContext(req), secretState, &schema); err != nil {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("not a valid vsphere credential %s", id)
	}

	cc, err := v.secretsLister.Get(defaultNamespace, id)
	if err != nil {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("error getting cloud cred %s: %v", id, err)
	}

	if !validSecret(cc) {
		return nil, httperror.InvalidBodyContent, fmt.Errorf("not a valid vsphere credential %s", id)
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
		if _, ok := cc.Data[cloudCredentialDataPrefix+v]; !ok {
			return false
		}
		cc.Data[v] = cc.Data[cloudCredentialDataPrefix+v] // move to new secret location
	}
	return true
}

func validSecret(cc *corev1.Secret) bool {
	for _, v := range dataFields {
		if _, ok := cc.Data[v]; !ok {
			return false
		}
	}
	return true
}
