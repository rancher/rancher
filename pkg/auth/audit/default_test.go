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
	machineDataInput, machineDataWant := []byte("{"), []byte("{")
	addToMachineData := func(item string, redact bool) {
		machineDataInput = append(machineDataInput, []byte("\""+item+"\" : \"fake_"+item+"\",")...)

		if redact {
			machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \""+redacted+"\",")...)
		} else {
			machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
		}
	}

	for _, fields := range management.DriverData {
		for _, item := range fields.PublicCredentialFields {
			addToMachineData(item, false)
		}

		for _, item := range fields.PrivateCredentialFields {
			addToMachineData(item, true)
		}

		for _, item := range fields.PasswordFields {
			addToMachineData(item, true)
		}
	}

	machineDataInput[len(machineDataInput)-1] = byte('}')
	machineDataWant[len(machineDataWant)-1] = byte('}')

	type testCase struct {
		Name     string
		Uri      string
		Headers  http.Header
		Body     []byte
		Expected []byte
	}

	cases := []testCase{
		{
			Name:     "password entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"password":"fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"password":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "Password entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"Password":"fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"Password":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "password entry no space",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"password":"whatever you want","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"password":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "Password entry no space",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"Password":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "currentPassword entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"currentPassword":"something super secret","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"currentPassword":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "newPassword entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"newPassword":"don't share this","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"newPassword":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "Multiple password entries",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"currentPassword":"fake_password","newPassword":"new_fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"currentPassword":"%s","newPassword":"%[1]s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "No password entries",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
			Expected: []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
		},
		{
			Name:     "Strategic password examples",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "Token entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"accessToken":"fake_access_token","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"accessToken":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "Token entry in slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":[{"accessToken":"fake_access_token","user":"fake_user"}]}`),
			Expected: fmt.Appendf(nil, `{"data":[{"accessToken":"%s","user":"fake_user"}]}`, redacted),
		},
		{
			Name:     "Token entry in args slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":{"commands":["--user","user","--token","sometoken"]}}`),
			Expected: fmt.Appendf(nil, `{"data":{"commands":["--user","user","--token","%s"]}}`, redacted),
		},
		{
			Name:     "Token entry in args slice but is last element of slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":{"commands":["--user","user","--token"]}}`),
			Expected: []byte(`{"data":{"commands":["--user","user","--token"]}}`),
		},
		{
			Name:     "With public fields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"accessKey":"fake_access_key","secretKey":"fake_secret_key","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"accessKey":"fake_access_key","secretKey":"%s","user":"fake_user"}`, redacted),
		},
		{
			Name:     "With secret data",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s","accessToken" :"%[1]s"}`, redacted),
		},
		{
			Name:     "With secret list data",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":"%[1]s"}]}`, redacted),
			Uri:      "/v1/secrets",
		},
		{
			// norman transforms some secret subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:     "With secret list data but no data field for array elements",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"id":"p-12345:testsecret","baseType":"secret","type":"Opaque","_type":"Opaque","foo":"something","bar":"something","accessToken":"token"}]}`),
			Expected: fmt.Appendf(nil, `{"data":[{"_type":"%s","accessToken":"%[1]s","bar":"%[1]s","baseType":"secret","foo":"%[1]s","id":"p-12345:testsecret","type":"%[1]s"}],"type":"collection"}`, redacted),
		},
		{
			Name:     "With secret list data from k8s proxy",
			Uri:      "/k8s/clusters/local/api/v1/secrets?limit=500",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"}]}`, redacted),
		},
		{
			Name:     "With secret data and wrong URI",
			Uri:      "/not-secret",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"%s"}`, redacted),
		},
		{
			Name:     "With nested sensitive information",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"sensitiveData": {"accessToken":"fake_access_token","user":"fake_user"}}`),
			Expected: fmt.Appendf(nil, `{"sensitiveData": {"accessToken":"%s","user":"fake_user"}}`, redacted),
		},
		{
			Name:     "With all machine driver fields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     machineDataInput,
			Expected: machineDataWant,
		},
		{
			Name:     "With no secret uri but secret base type slice",
			Uri:      `/v3/project/local:p-12345/namespacedcertificates?limit=-1&sort=name`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":{"testfield":"somesecretencodeddata"},"id":"cattle-system:test","kind":"Opaque"}]}`),
			Expected: fmt.Appendf(nil, `{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":"%s","id":"cattle-system:test","kind":"Opaque"}]}`, redacted),
		},
		{
			Name:     "With kubeconfig from generateKubeconfig action",
			Uri:      `/v3/clusters/c-xxxxx?action=generateKubeconfig`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"baseType":"generateKubeConfigOutput","config":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","type":"generateKubeConfigOutput"}`),
			Expected: fmt.Appendf(nil, `{"baseType":"generateKubeConfigOutput","config":"%s","type":"generateKubeConfigOutput"}`, redacted),
		},
		{
			Name:     "With kubeconfig from connect agent",
			Uri:      `/v3/connect/agent`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: fmt.Appendf(nil, `{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted),
		},
		{
			Name:     "With kubeconfig from random request",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: fmt.Appendf(nil, `{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted),
		},
		{
			Name:     "With items from sensitiveBodyFields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"credentials":"{'fakeCredName': 'fakeCred'}","applicationSecret":"fakeAppSecret","oauthCredential":"fakeOauth","serviceAccountCredential":"fakeSACred","spKey":"fakeSPKey","spCert":"fakeSPCERT","certificate":"fakeCert","privateKey":"fakeKey"}`),
			Expected: fmt.Appendf(nil, `{"credentials":"%s","applicationSecret":"%[1]s","oauthCredential":"%[1]s","serviceAccountCredential":"%[1]s","spKey":"%[1]s","spCert":"%[1]s","certificate":"%[1]s","privateKey":"%[1]s"}`, redacted),
		},
		{
			Name:     "With malformed input",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"key":"value","response":}`),
			Expected: fmt.Appendf(nil, `{"%s":"failed to unmarshal request body: invalid character '}' looking for beginning of value"}`, auditLogErrorKey),
		},
		{
			Name:     "With secret string data with last-applied-configuration annotation",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"apiVersion": "v1", "stringData": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"stringData\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			Expected: fmt.Appendf(nil, `{"apiVersion": "v1", "stringData": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted),
		},
		{
			Name:     "With secret data with last-applied-configuration annotation",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"apiVersion": "v1", "data": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"data\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			Expected: fmt.Appendf(nil, `{"apiVersion": "v1", "data": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted),
		},
		{
			Name:     "password entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"password":"fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"password":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "Password entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"Password":"fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"Password":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "password entry no space",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"password":"whatever you want","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"password":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "Password entry no space",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"Password":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "currentPassword entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"currentPassword":"something super secret","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"currentPassword":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "newPassword entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"newPassword":"don't share this","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"newPassword":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "Multiple password entries",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"currentPassword":"fake_password","newPassword":"new_fake_password","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"currentPassword":"%s","newPassword":"%[1]s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "No password entries",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
			Expected: []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
		},

		{
			Name:     "Strategic password examples",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "Token entry",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"accessToken":"fake_access_token","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"accessToken":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "Token entry in slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":[{"accessToken":"fake_access_token","user":"fake_user"}]}`),
			Expected: fmt.Appendf(nil, `{"data":[{"accessToken":"%s","user":"fake_user"}]}`, redacted),
		},

		{
			Name:     "Token entry in args slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":{"commands":["--user","user","--token","sometoken"]}}`),
			Expected: fmt.Appendf(nil, `{"data":{"commands":["--user","user","--token","%s"]}}`, redacted),
		},

		{
			Name:     "Token entry in args slice but is last element of slice",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"data":{"commands":["--user","user","--token"]}}`),
			Expected: []byte(`{"data":{"commands":["--user","user","--token"]}}`),
		},

		{
			Name:     "With public fields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"accessKey":"fake_access_key","secretKey":"fake_secret_key","user":"fake_user"}`),
			Expected: fmt.Appendf(nil, `{"accessKey":"fake_access_key","secretKey":"%s","user":"fake_user"}`, redacted),
		},

		{
			Name:     "With secret data",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s","accessToken" :"%[1]s"}`, redacted),
		},

		{
			Name:     "With secret list data",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":"%[1]s"}]}`, redacted),
			Uri:      "/v1/secrets",
		},

		{
			// norman transforms some secret subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:     "With secret list data but no data field for array elements",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"id":"p-12345:testsecret","baseType":"secret","type":"Opaque","_type":"Opaque","foo":"something","bar":"something","accessToken":"token"}]}`),
			Expected: fmt.Appendf(nil, `{"data":[{"_type":"%s","accessToken":"%[1]s","bar":"%[1]s","baseType":"secret","foo":"%[1]s","id":"p-12345:testsecret","type":"%[1]s"}],"type":"collection"}`, redacted),
		},

		{
			Name:     "With secret list data from k8s proxy",
			Uri:      "/k8s/clusters/local/api/v1/secrets?limit=500",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"}]}`, redacted),
		},

		{
			Name:     "With secret data and wrong URI",
			Uri:      "/not-secret",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"%s"}`, redacted),
		},

		{
			Name:     "With nested sensitive information",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"sensitiveData": {"accessToken":"fake_access_token","user":"fake_user"}}`),
			Expected: fmt.Appendf(nil, `{"sensitiveData": {"accessToken":"%s","user":"fake_user"}}`, redacted),
		},

		{
			Name:     "With all machine driver fields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     machineDataInput,
			Expected: machineDataWant,
		},

		{
			Name:     "With no secret uri but secret base type slice",
			Uri:      `/v3/project/local:p-12345/namespacedcertificates?limit=-1&sort=name`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":{"testfield":"somesecretencodeddata"},"id":"cattle-system:test","kind":"Opaque"}]}`),
			Expected: fmt.Appendf(nil, `{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":"%s","id":"cattle-system:test","kind":"Opaque"}]}`, redacted),
		},

		{
			Name:     "With kubeconfig from generateKubeconfig action",
			Uri:      `/v3/clusters/c-xxxxx?action=generateKubeconfig`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"baseType":"generateKubeConfigOutput","config":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","type":"generateKubeConfigOutput"}`),
			Expected: fmt.Appendf(nil, `{"baseType":"generateKubeConfigOutput","config":"%s","type":"generateKubeConfigOutput"}`, redacted),
		},

		{
			Name:     "With kubeconfig from connect agent",
			Uri:      `/v3/connect/agent`,
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: fmt.Appendf(nil, `{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted),
		},

		{
			Name:     "With kubeconfig from random request",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: fmt.Appendf(nil, `{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted),
		},

		{
			Name:     "With items from sensitiveBodyFields",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"credentials":"{'fakeCredName': 'fakeCred'}","applicationSecret":"fakeAppSecret","oauthCredential":"fakeOauth","serviceAccountCredential":"fakeSACred","spKey":"fakeSPKey","spCert":"fakeSPCERT","certificate":"fakeCert","privateKey":"fakeKey"}`),
			Expected: fmt.Appendf(nil, `{"credentials":"%s","applicationSecret":"%[1]s","oauthCredential":"%[1]s","serviceAccountCredential":"%[1]s","spKey":"%[1]s","spCert":"%[1]s","certificate":"%[1]s","privateKey":"%[1]s"}`, redacted),
		},

		{
			Name:     "With malformed input",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"key":"value","response":}`),
			Expected: fmt.Appendf(nil, `{"%s":"failed to unmarshal request body: invalid character '}' looking for beginning of value"}`, auditLogErrorKey),
		},

		{
			Name:     "With secret string data with last-applied-configuration annotation",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"apiVersion": "v1", "stringData": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"stringData\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			Expected: fmt.Appendf(nil, `{"apiVersion": "v1", "stringData": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted),
		},

		{
			Name:     "With secret data with last-applied-configuration annotation",
			Uri:      "/secrets",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"apiVersion": "v1", "data": {"secret": "02020202020202020202"}, "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"annotations\":{},\"name\":\"opaque-secret2\",\"namespace\":\"default\"},\"data\":{\"secret\":\"02020202020202020202\"}}"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`),
			Expected: fmt.Appendf(nil, `{"apiVersion": "v1", "data": "%s", "kind": "Secret", "metadata": {"annotations": {"kubectl.kubernetes.io/last-applied-configuration": "%[1]s"}, "name": "opaque-secret2", "namespace": "default"}, "type": "Opaque"}`, redacted),
		},

		{
			Name:     "With configmap data",
			Uri:      "/configmaps",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s", "accessToken" : "%[1]s"}`, redacted),
		},
		{
			Name:     "With config map list data",
			Uri:      "/configmaps",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type": "collection", "data":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}},{"metadata":{"namespace":"default","name":"my_configmap_2"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"type": "collection", "data":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s"},{"metadata":{"namespace":"default","name":"my_configmap_2"},"data":"%[1]s"}]}`, redacted),
		},
		{
			// norman transforms some configmap subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:     "With configmap list data but no data field for array elements",
			Uri:      "/configmaps",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type": "collection", "data":[{"id":"p-12345:testconfigmap","baseType":"configmap","foo":"something","bar":"something","accessToken":"token"}]}`),
			Expected: fmt.Appendf(nil, `{"data":[{"accessToken":"%[1]s","bar":"%[1]s","baseType":"configmap","foo":"%[1]s","id":"p-12345:testconfigmap"}],"type":"collection"}`, redacted),
		},
		{
			Name:     "With secret list data from k8s proxy",
			Uri:      "/k8s/clusters/local/api/v1/configmaps?limit=500",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"kind": "ConfigMapList", "items":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: fmt.Appendf(nil, `{"kind": "ConfigMapList", "items":[{"metadata":{"namespace":"default","name":"my_configmap"},"data":"%s"}]}`, redacted),
		},
		{
			Name:     "With configmap data and wrong URI",
			Uri:      "nont-configmap",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			Expected: fmt.Appendf(nil, `{"metadata":{"namespace":"default","name":"my_configmap"},"data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "%s"}`, redacted),
		},
		{
			Name:     "With enabled data encryption",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"type": "cluster", "rancherKubernetesEngineConfig": {"services": {"kubeApi": {"secretsEncryptionConfig": {"enabled": true, "customConfig": {}}}}}}`),
			Expected: fmt.Appendf(nil, `{"type": "cluster", "rancherKubernetesEngineConfig": {"services": {"kubeApi": {"secretsEncryptionConfig": "%s"}}}}`, redacted),
		},
		{
			Name:     "With create new custom clusters",
			Headers:  http.Header{"Content-Type": {contentTypeJSON}},
			Body:     []byte(`{"normalField": "some data", "manifestUrl": "https://localhost:8443/v3/import/abcd.yaml", "insecureWindowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "insecureCommand": "curl https://localhost:8443/v3/import/abcd.yaml", "command": "curl https://localhost:8443/v3/import/abcd.yaml", "windowsNodeCommand": "curl https://localhost:8443/v3/import/abcd.yaml"}`),
			Expected: fmt.Appendf(nil, `{"normalField": "some data", "manifestUrl": "%s", "insecureWindowsNodeCommand": "%[1]s", "insecureNodeCommand": "%[1]s", "insecureCommand": "%[1]s", "command": "%[1]s", "windowsNodeCommand": "%[1]s"}`, redacted),
		},
	}

	buffer := bytes.NewBuffer(nil)
	writer, err := NewWriter(buffer, WriterOptions{
		DefaultPolicyLevel: auditlogv1.LevelRequestResponse,
	})
	require.NoError(t, err)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			log := &log{
				AuditID:        "0123456789",
				RequestURI:     c.Uri,
				RequestHeader:  c.Headers,
				rawRequestBody: c.Body,
			}

			err = writer.Write(log)
			assert.NoError(t, err)

			err = json.Unmarshal(buffer.Bytes(), log)
			assert.NoError(t, err)

			actual := log.RequestBody

			expected := map[string]any{}
			err = json.Unmarshal(c.Expected, &expected)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)

			buffer.Reset()
		})
	}
}
