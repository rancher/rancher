package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPolicies(t *testing.T) {

	machineDataInput, machineDataWant := make(map[string]interface{}), make(map[string]interface{})
	for driverName, fields := range management.DriverData {
		inputFields := make(map[string]string)
		wantFields := make(map[string]string)

		for _, item := range fields.PublicCredentialFields {
			inputFields[item] = "fake_" + item

			// This exception is needed because the management.ExoscaleDriver config has the 'apiKey' set to Public, contrary to the other drivers that set it to Private.
			// With the way the auditLog currently works, it will simply redact *all apiKeys* if at least one of them is set to private.
			// A more granular policy will be implemented in the future to better handle this scenario and have the AuditLog *not* redact the apiKey for the
			// Exoscale driver. For now, however, the if condition will guarantee the tests can handle this edge case without failing.
			if driverName == management.ExoscaleDriver {
				wantFields[item] = redacted
			} else {
				wantFields[item] = "fake_" + item
			}
		}

		for _, item := range fields.PrivateCredentialFields {
			inputFields[item] = "fake_" + item
			wantFields[item] = redacted
		}

		for _, item := range fields.PasswordFields {
			inputFields[item] = "fake_" + item
			wantFields[item] = redacted
		}

		machineDataInput[driverName] = inputFields
		machineDataWant[driverName] = wantFields
	}

	type testCase struct {
		Name            string
		Uri             string
		Headers         http.Header
		ExpectedHeaders http.Header
		Body            []byte
		ExpectedBody    []byte
	}

	cases := []testCase{
		{
			Name:         "password entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"password":"fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "Password entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"Password":"fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "password entry no space",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"password":"whatever you want","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "Password entry no space",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "currentPassword entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"currentPassword":"something super secret","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"currentPassword":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "newPassword entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"newPassword":"don't share this","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"newPassword":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "Multiple password entries",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"currentPassword":"fake_password","newPassword":"new_fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"currentPassword":"%s","newPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "No password entries",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
			ExpectedBody: []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
		},
		{
			Name:         "Strategic password examples",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "Token entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"accessToken":"fake_access_token","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"accessToken":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "Token entry in slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":[{"accessToken":"fake_access_token","user":"fake_user"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":[{"accessToken":"%s","user":"fake_user"}]}`, redacted)),
		},
		{
			Name:         "Token entry in args slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":{"commands":["--user","user","--token","sometoken"]}}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":{"commands":["--user","user","--token","%s"]}}`, redacted)),
		},
		{
			Name:         "Token entry in args slice but is last element of slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":{"commands":["--user","user","--token"]}}`),
			ExpectedBody: []byte(`{"data":{"commands":["--user","user","--token"]}}`),
		},
		{
			Name:         "With public fields",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"accessKey":"fake_access_key","secretKey":"fake_secret_key","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"accessKey":"fake_access_key","secretKey":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:         "With secret data",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s","accessToken" :"%[1]s"}`, redacted)),
		},
		{
			Name:         "With secret list data",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":"%[1]s"}]}`, redacted)),
			Uri:          "/v1/secrets",
		},
		{
			// norman transforms some secret subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:         "With secret list data but no data field for array elements",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"id":"p-12345:testsecret","baseType":"secret","type":"Opaque","_type":"Opaque","foo":"something","bar":"something","accessToken":"token"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":[{"_type":"%s","accessToken":"%[1]s","bar":"%[1]s","baseType":"secret","foo":"%[1]s","id":"p-12345:testsecret","type":"%[1]s"}],"type":"collection"}`, redacted)),
		},
		{
			Name:         "With secret list data from k8s proxy",
			Uri:          "/k8s/clusters/local/api/v1/secrets?limit=500",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"}]}`, redacted)),
		},
		{
			Name:         "With secret data and wrong URI",
			Uri:          "/not-secret",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"%s"}`, redacted)),
		},
		{
			Name:         "With nested sensitive information",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"sensitiveData": {"accessToken":"fake_access_token","user":"fake_user"}}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"sensitiveData": {"accessToken":"%s","user":"fake_user"}}`, redacted)),
		},
		{
			Name:         "With no secret uri but secret base type slice",
			Uri:          `/v3/project/local:p-12345/namespacedcertificates?limit=-1&sort=name`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":{"testfield":"somesecretencodeddata"},"id":"cattle-system:test","kind":"Opaque"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":"%s","id":"cattle-system:test","kind":"Opaque"}]}`, redacted)),
		},
		{
			Name:         "With kubeconfig from generateKubeconfig action",
			Uri:          `/v3/clusters/c-xxxxx?action=generateKubeconfig`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"baseType":"generateKubeConfigOutput","config":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","type":"generateKubeConfigOutput"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"baseType":"generateKubeConfigOutput","config":"%s","type":"generateKubeConfigOutput"}`, redacted)),
		},
		{
			Name:         "With kubeconfig from connect agent",
			Uri:          `/v3/connect/agent`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
		},
		{
			Name:         "With kubeconfig from random request",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
		},
		{
			Name:         "With items from sensitiveBodyFields",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"credentials":"{'fakeCredName': 'fakeCred'}","applicationSecret":"fakeAppSecret","oauthCredential":"fakeOauth","serviceAccountCredential":"fakeSACred","spKey":"fakeSPKey","spCert":"fakeSPCERT","certificate":"fakeCert","privateKey":"fakeKey"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"credentials":"%s","applicationSecret":"%[1]s","oauthCredential":"%[1]s","serviceAccountCredential":"%[1]s","spKey":"%[1]s","spCert":"%[1]s","certificate":"%[1]s","privateKey":"%[1]s"}`, redacted)),
		},
		{
			Name:         "With malformed input",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"key":"value","response":}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"%s":"failed to unmarshal request body: invalid character '}' looking for beginning of value"}`, auditLogErrorKey)),
		},
		{
			Name:         "With secret string data with last-applied-configuration annotation",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"apiVersion": "v1", "stringData": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"stringData\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"apiVersion": "v1", "stringData": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted)),
		},
		{
			Name:         "With secret data with last-applied-configuration annotation",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"apiVersion": "v1", "data": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"data\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"apiVersion": "v1", "data": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted)),
		},
		{
			Name:         "password entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"password":"fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "Password entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"Password":"fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "password entry no space",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"password":"whatever you want","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "Password entry no space",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "currentPassword entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"currentPassword":"something super secret","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"currentPassword":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "newPassword entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"newPassword":"don't share this","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"newPassword":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "Multiple password entries",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"currentPassword":"fake_password","newPassword":"new_fake_password","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"currentPassword":"%s","newPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "No password entries",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
			ExpectedBody: []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
		},

		{
			Name:         "Strategic password examples",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "Token entry",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"accessToken":"fake_access_token","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"accessToken":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "Token entry in slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":[{"accessToken":"fake_access_token","user":"fake_user"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":[{"accessToken":"%s","user":"fake_user"}]}`, redacted)),
		},

		{
			Name:         "Token entry in args slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":{"commands":["--user","user","--token","sometoken"]}}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":{"commands":["--user","user","--token","%s"]}}`, redacted)),
		},

		{
			Name:         "Token entry in args slice but is last element of slice",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"data":{"commands":["--user","user","--token"]}}`),
			ExpectedBody: []byte(`{"data":{"commands":["--user","user","--token"]}}`),
		},

		{
			Name:         "With public fields",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"accessKey":"fake_access_key","secretKey":"fake_secret_key","user":"fake_user"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"accessKey":"fake_access_key","secretKey":"%s","user":"fake_user"}`, redacted)),
		},

		{
			Name:         "With secret data",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s","accessToken" :"%[1]s"}`, redacted)),
		},

		{
			Name:         "With secret list data",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":"%[1]s"}]}`, redacted)),
			Uri:          "/v1/secrets",
		},

		{
			// norman transforms some secret subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:         "With secret list data but no data field for array elements",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"id":"p-12345:testsecret","baseType":"secret","type":"Opaque","_type":"Opaque","foo":"something","bar":"something","accessToken":"token"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":[{"_type":"%s","accessToken":"%[1]s","bar":"%[1]s","baseType":"secret","foo":"%[1]s","id":"p-12345:testsecret","type":"%[1]s"}],"type":"collection"}`, redacted)),
		},

		{
			Name:         "With secret list data from k8s proxy",
			Uri:          "/k8s/clusters/local/api/v1/secrets?limit=500",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"}]}`, redacted)),
		},

		{
			Name:         "With secret data and wrong URI",
			Uri:          "/not-secret",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"%s"}`, redacted)),
		},

		{
			Name:         "With nested sensitive information",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"sensitiveData": {"accessToken":"fake_access_token","user":"fake_user"}}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"sensitiveData": {"accessToken":"%s","user":"fake_user"}}`, redacted)),
		},

		{
			Name:         "With no secret uri but secret base type slice",
			Uri:          `/v3/project/local:p-12345/namespacedcertificates?limit=-1&sort=name`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":{"testfield":"somesecretencodeddata"},"id":"cattle-system:test","kind":"Opaque"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":"%s","id":"cattle-system:test","kind":"Opaque"}]}`, redacted)),
		},

		{
			Name:         "With kubeconfig from generateKubeconfig action",
			Uri:          `/v3/clusters/c-xxxxx?action=generateKubeconfig`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"baseType":"generateKubeConfigOutput","config":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","type":"generateKubeConfigOutput"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"baseType":"generateKubeConfigOutput","config":"%s","type":"generateKubeConfigOutput"}`, redacted)),
		},

		{
			Name:         "With kubeconfig from connect agent",
			Uri:          `/v3/connect/agent`,
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
		},

		{
			Name:         "With kubeconfig from random request",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
		},

		{
			Name:         "With items from sensitiveBodyFields",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"credentials":"{'fakeCredName': 'fakeCred'}","applicationSecret":"fakeAppSecret","oauthCredential":"fakeOauth","serviceAccountCredential":"fakeSACred","spKey":"fakeSPKey","spCert":"fakeSPCERT","certificate":"fakeCert","privateKey":"fakeKey"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"credentials":"%s","applicationSecret":"%[1]s","oauthCredential":"%[1]s","serviceAccountCredential":"%[1]s","spKey":"%[1]s","spCert":"%[1]s","certificate":"%[1]s","privateKey":"%[1]s"}`, redacted)),
		},

		{
			Name:         "With malformed input",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"key":"value","response":}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"%s":"failed to unmarshal request body: invalid character '}' looking for beginning of value"}`, auditLogErrorKey)),
		},

		{
			Name:         "With secret string data with last-applied-configuration annotation",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"apiVersion": "v1", "stringData": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"stringData\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"apiVersion": "v1", "stringData": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted)),
		},

		{
			Name:         "With secret data with last-applied-configuration annotation",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"apiVersion": "v1", "data": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"data\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"apiVersion": "v1", "data": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted)),
		},

		{
			Name:         "With secret list with last-applied-configuration annotation",
			Uri:          "/secrets",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type": "collection", "data": [{"apiVersion": "v1", "data": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"data\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type": "collection", "data": [{"apiVersion": "v1", "data": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}]}`, redacted)),
		},

		{
			Name:         "With configmap data",
			Uri:          "/configmaps",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s", "accessToken" : "%[1]s"}`, redacted)),
		},
		{
			Name:         "With config map list data",
			Uri:          "/configmaps",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type": "collection", "data":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}},{"metadata":{"namespace":"default","name":"my_configmap_2"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type": "collection", "data":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s"},{"metadata":{"namespace":"default","name":"my_configmap_2"},"data":"%[1]s"}]}`, redacted)),
		},
		{
			// norman transforms some configmap subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:         "With configmap list data but no data field for array elements",
			Uri:          "/configmaps",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type": "collection", "data":[{"id":"p-12345:testconfigmap","baseType":"configmap","foo":"something","bar":"something","accessToken":"token"}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"data":[{"accessToken":"%[1]s","bar":"%[1]s","baseType":"configmap","foo":"%[1]s","id":"p-12345:testconfigmap"}],"type":"collection"}`, redacted)),
		},
		{
			Name:         "With secret list data from k8s proxy",
			Uri:          "/k8s/clusters/local/api/v1/configmaps?limit=500",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"kind": "ConfigMapList", "items":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"kind": "ConfigMapList", "items":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s"}]}`, redacted)),
		},
		{
			Name:         "With configmap data and wrong URI",
			Uri:          "nont-configmap",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "%s"}`, redacted)),
		},
		{
			Name:         "With enabled data encryption",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"type": "cluster", "rancherKubernetesEngineConfig": {"services": {"kubeApi": {"secretsEncryptionConfig": {"enabled": true, "customConfig": {}}}}}}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"type": "cluster", "rancherKubernetesEngineConfig": {"services": {"kubeApi": {"secretsEncryptionConfig": "%s"}}}}`, redacted)),
		},
		{
			Name:         "With create new custom clusters",
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         []byte(`{"normalField": "some data", "manifestUrl": "https://localhost:8443/v3/import/abcd.yaml", "insecureWindowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "command": "curl https://localhost:8443/v3/import/abcd.yaml", "windowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml"}`),
			ExpectedBody: []byte(fmt.Sprintf(`{"normalField": "some data", "manifestUrl": "%s", "insecureWindowsNodeCommand": "%[1]s", "insecureNodeCommand": "%[1]s", "insecureCommand": "%[1]s", "command": "%[1]s", "windowsNodeCommand": "%[1]s"}`, redacted)),
		},
		{
			Name:            "With redactable Referer header",
			Headers:         http.Header{"Content-Type": {contentTypeJSON}, "Referer": {"/v3/import/redactMe.yaml"}},
			ExpectedHeaders: http.Header{"Content-Type": {contentTypeJSON}, "Referer": {"/v3/import/[redacted]"}},
			Body:            []byte(`{"normalField": "some data", "manifestUrl": "https://localhost:8443/v3/import/abcd.yaml", "insecureWindowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "command": "curl https://localhost:8443/v3/import/abcd.yaml", "windowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml"}`),
			ExpectedBody:    []byte(fmt.Sprintf(`{"normalField": "some data", "manifestUrl": "%s", "insecureWindowsNodeCommand": "%[1]s", "insecureNodeCommand": "%[1]s", "insecureCommand": "%[1]s", "command": "%[1]s", "windowsNodeCommand": "%[1]s"}`, redacted)),
		},
		{
			Name:            "With non-redactable Referrer header",
			Headers:         http.Header{"Content-Type": {contentTypeJSON}, "Referrer": {"/v3/import/redactMe.yaml"}},
			ExpectedHeaders: http.Header{"Content-Type": {contentTypeJSON}, "Referrer": {"/v3/import/redactMe.yaml"}},
			Body:            []byte(`{"normalField": "some data", "manifestUrl": "https://localhost:8443/v3/import/abcd.yaml", "insecureWindowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "command": "curl https://localhost:8443/v3/import/abcd.yaml", "windowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml"}`),
			ExpectedBody:    []byte(fmt.Sprintf(`{"normalField": "some data", "manifestUrl": "%s", "insecureWindowsNodeCommand": "%[1]s", "insecureNodeCommand": "%[1]s", "insecureCommand": "%[1]s", "command": "%[1]s", "windowsNodeCommand": "%[1]s"}`, redacted)),
		},
	}

	for driverName, expectedFields := range machineDataWant {
		expectedFieldsBytes, err := json.Marshal(expectedFields)
		if err != nil {
			t.Fatalf("failed to marshal expected fields: %v", err)
		}

		inputFieldsBytes, err := json.Marshal(machineDataInput[driverName])
		if err != nil {
			t.Fatalf("failed to marshal input fields: %v", err)
		}

		cases = append(cases, testCase{
			Name:         fmt.Sprintf("With machine driver fields for %s", driverName),
			Headers:      http.Header{"Content-Type": {contentTypeJSON}},
			Body:         inputFieldsBytes,
			ExpectedBody: expectedFieldsBytes,
		})
	}

	buffer := bytes.NewBuffer(nil)
	writer, err := NewWriter(buffer, WriterOptions{
		DefaultPolicyLevel: auditlogv1.LevelRequestResponse,
	})
	require.NoError(t, err)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			log := &logEntry{
				AuditID:       "0123456789",
				RequestURI:    c.Uri,
				RequestHeader: c.Headers,
			}

			// In production, request bodies are prepared by newLog(); tests that
			// construct logEntry directly must populate RequestBody themselves.
			prepareLogEntry(log, &testLogData{
				verbosity:  verbosityForLevel(auditlogv1.LevelRequestResponse),
				reqHeaders: c.Headers,
				rawReqBody: c.Body,
				resHeaders: c.Headers,
				rawResBody: c.ExpectedBody,
			})

			err = writer.Write(log)
			assert.NoError(t, err)

			err = json.Unmarshal(buffer.Bytes(), log)
			assert.NoError(t, err)

			actual := log.RequestBody

			expected := map[string]any{}
			err = json.Unmarshal(c.ExpectedBody, &expected)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)

			if c.ExpectedHeaders != nil {
				actualHeaders := log.RequestHeader
				assert.Equal(t, c.ExpectedHeaders, actualHeaders)
			}

			buffer.Reset()
		})
	}
}
