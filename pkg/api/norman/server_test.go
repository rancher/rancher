package norman

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/norman/customization/authn"
	"github.com/rancher/rancher/pkg/api/norman/store/scoped"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	webhookerrors "k8s.io/apiserver/pkg/admission/plugin/webhook/errors"
	"k8s.io/client-go/rest"
)

func TestRoleBindingAdmissionResponse(t *testing.T) {
	const escalation = "user \"u-test\" (groups=[\"system:authenticated\"]) is attempting to grant RBAC permissions not currently held:\n{APIGroups:[\"\"], Resources:[\"pods\"], Verbs:[\"create\"]}"

	for _, binding := range []struct {
		schemaID  string
		kind      string
		resource  string
		scopeKey  string
		scopeID   string
		namespace string
		role      string
	}{
		{"projectRoleTemplateBinding", "ProjectRoleTemplateBinding", "projectroletemplatebindings", "projectId", "local:p-test", "p-test", "project-owner"},
		{"clusterRoleTemplateBinding", "ClusterRoleTemplateBinding", "clusterroletemplatebindings", "clusterId", "local", "local", "cluster-owner"},
	} {
		t.Run(binding.schemaID, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				status int
				reason metav1.StatusReason
				detail string
				want   int
				code   string
			}{
				{"legacy_escalation", http.StatusUnprocessableEntity, metav1.StatusReasonInvalid, escalation, http.StatusForbidden, "Forbidden"},
				{"invalid_binding", http.StatusUnprocessableEntity, metav1.StatusReasonInvalid, "roleTemplateName: Invalid value", http.StatusUnprocessableEntity, "Invalid"},
				{"forbidden", http.StatusForbidden, metav1.StatusReasonForbidden, escalation, http.StatusForbidden, "Forbidden"},
				{"allowed", http.StatusCreated, "", "", http.StatusCreated, ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var denial *metav1.Status
					if tc.status != http.StatusCreated {
						status := webhookerrors.ToStatusErr("rancher.cattle.io."+binding.resource+".management.cattle.io", &metav1.Status{
							Code: int32(tc.status), Reason: tc.reason, Message: tc.detail,
						})
						denial = &status.ErrStatus
						denial.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}
					}

					kube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, http.MethodPost, r.Method)
						assert.Equal(t, "/apis/management.cattle.io/v3/namespaces/"+binding.namespace+"/"+binding.resource, r.URL.Path)
						assert.Equal(t, "u-test", r.Header.Get("Impersonate-User"))
						var data map[string]interface{}
						if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&data)) {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						assert.Equal(t, binding.role, data["roleTemplateName"])
						assert.Equal(t, "local://u-test", data["userPrincipalName"])
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(tc.status)
						if denial != nil {
							assert.NoError(t, json.NewEncoder(w).Encode(denial))
							return
						}
						data["metadata"].(map[string]interface{})["name"] = "test-binding"
						assert.NoError(t, json.NewEncoder(w).Encode(data))
					}))
					defer kube.Close()

					getter, err := proxy.NewClientGetterFromConfig(rest.Config{Host: kube.URL})
					require.NoError(t, err)
					schemas := types.NewSchemas().AddSchemas(managementschema.Schemas)
					schema := schemas.Schema(&managementschema.Version, binding.schemaID)
					schema.ResourceFields = maps.Clone(schema.ResourceFields)
					namespaceField := schema.ResourceFields["namespaceId"]
					namespaceField.Required = false
					schema.ResourceFields["namespaceId"] = namespaceField
					store := proxy.NewProxyStore(t.Context(), getter, config.ManagementStorageContext, nil, []string{"apis"},
						"management.cattle.io", "v3", binding.kind, binding.resource)
					projects := &fakes.ProjectListerMock{GetFunc: func(namespace, name string) (*v3.Project, error) {
						assert.Equal(t, "local", namespace)
						assert.Equal(t, "p-test", name)
						return &v3.Project{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}, nil
					}}
					schema.Store = scoped.NewScopedStore(binding.scopeKey, &authn.Store{Store: store}, projects)
					server, err := NewServer(schemas)
					require.NoError(t, err)

					body := fmt.Sprintf(`{%q:%q,"roleTemplateId":%q,"userPrincipalId":"local://u-test"}`, binding.scopeKey, binding.scopeID, binding.role)
					req := httptest.NewRequest(http.MethodPost, "/v3/"+binding.resource, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Impersonate-User", "u-test")
					rec := httptest.NewRecorder()
					server.ServeHTTP(rec, req)

					require.Equal(t, tc.want, rec.Code, rec.Body.String())
					var response struct {
						Code    string `json:"code"`
						Message string `json:"message"`
						ID      string `json:"id"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
					if denial != nil {
						assert.Equal(t, tc.code, response.Code)
						assert.Equal(t, denial.Message, response.Message)
					} else {
						assert.Equal(t, binding.namespace+":test-binding", response.ID)
					}
				})
			}
		})
	}
}
