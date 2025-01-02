package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPolicies(t *testing.T) {
	writer, err := NewWriter(io.Discard, WriterOptions{
		DefaultPolicyLevel: auditlogv1.LevelRequest,
	})
	require.NoError(t, err)

	machineDataInput, machineDataWant := []byte("{"), []byte("{")
	for _, v := range management.DriverData {
		for key, value := range v {
			if strings.HasPrefix(key, "optional") {
				continue
			}
			public := strings.HasPrefix(key, "public")
			for _, item := range value {
				machineDataInput = append(machineDataInput, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				if public {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				} else {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \""+redacted+"\",")...)
				}
			}
		}
	}

	machineDataInput[len(machineDataInput)-1] = byte('}')
	machineDataWant[len(machineDataWant)-1] = byte('}')

	type testCase struct {
		Name     string
		Uri      string
		Body     []byte
		Expected []byte
		Skip     bool
	}

	cases := []testCase{
		{
			Name:     "password entry",
			Body:     []byte(`{"password":"fake_password","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "Password entry",
			Body:     []byte(`{"Password":"fake_password","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "password entry no space",
			Body:     []byte(`{"password":"whatever you want","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "Password entry no space",
			Body:     []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "currentPassword entry",
			Body:     []byte(`{"currentPassword":"something super secret","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"currentPassword":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "newPassword entry",
			Body:     []byte(`{"newPassword":"don't share this","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"newPassword":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "Multiple password entries",
			Body:     []byte(`{"currentPassword":"fake_password","newPassword":"new_fake_password","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"currentPassword":"%s","newPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "No password entries",
			Body:     []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
			Expected: []byte(`{"user":"fake_user","user_info":"some information about the user","request_info":"some info about the request"}`),
		},
		{
			Name:     "Strategic password examples",
			Body:     []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "Token entry",
			Body:     []byte(`{"accessToken":"fake_access_token","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"accessToken":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "Token entry in slice",
			Body:     []byte(`{"data":[{"accessToken":"fake_access_token","user":"fake_user"}]}`),
			Expected: []byte(fmt.Sprintf(`{"data":[{"accessToken":"%s","user":"fake_user"}]}`, redacted)),
		},
		{
			Name:     "Token entry in args slice",
			Body:     []byte(`{"data":{"commands":["--user","user","--token","sometoken"]}}`),
			Expected: []byte(fmt.Sprintf(`{"data":{"commands":["--user","user","--token","%s"]}}`, redacted)),
		},
		{
			Name:     "Token entry in args slice but is last element of slice",
			Body:     []byte(`{"data":{"commands":["--user","user","--token"]}}`),
			Expected: []byte(`{"data":{"commands":["--user","user","--token"]}}`),
		},
		{
			Name:     "With public fields",
			Body:     []byte(`{"accessKey":"fake_access_key","secretKey":"fake_secret_key","user":"fake_user"}`),
			Expected: []byte(fmt.Sprintf(`{"accessKey":"fake_access_key","secretKey":"%s","user":"fake_user"}`, redacted)),
		},
		{
			Name:     "With secret data",
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s","accessToken" :"%[1]s"}`, redacted)),
			Uri:      "/secrets",
		},
		{
			Name:     "With secret list data",
			Body:     []byte(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: []byte(fmt.Sprintf(`{"type":"collection","data":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"},{"type":"Opaque","metadata":{"namespace":"default","name":"my secret2"},"_type":"Opaque","data":"%[1]s"}]}`, redacted)),
			Uri:      "/v1/secrets",
		},
		{
			// norman transforms some secret subtypes to where their data fields cannot be distinguished from non-sensitive fields.
			// In this case, all fields aside from id, created, and baseType should be redacted.
			Name:     "With secret list data but no data field for array elements",
			Body:     []byte(`{"type":"collection","data":[{"id":"p-12345:testsecret","baseType":"secret","type":"Opaque","_type":"Opaque","foo":"something","bar":"something","accessToken":"token"}]}`),
			Expected: []byte(fmt.Sprintf(`{"data":[{"_type":"%s","accessToken":"%[1]s","bar":"%[1]s","baseType":"secret","foo":"%[1]s","id":"p-12345:testsecret","type":"%[1]s"}],"type":"collection"}`, redacted)),
			Uri:      "/secrets",
		},
		{
			Name:     "With secret list data from k8s proxy",
			Body:     []byte(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"}}]}`),
			Expected: []byte(fmt.Sprintf(`{"kind":"SecretList","items":[{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":"%s"}]}`, redacted)),
			Uri:      "/k8s/clusters/local/api/v1/secrets?limit=500",
		},
		{
			Name:     "With secret data and wrong URI",
			Body:     []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"fake_access_token"}`),
			Expected: []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\","bar":"U3VwZXIgU2VjcmV0IERhdGEK"},"accessToken" :"%s"}`, redacted)),
			Uri:      "/not-secret",
		},
		{
			Name:     "With nested sensitive information",
			Body:     []byte(`{"sensitiveData": {"accessToken":"fake_access_token","user":"fake_user"}}`),
			Expected: []byte(fmt.Sprintf(`{"sensitiveData": {"accessToken":"%s","user":"fake_user"}}`, redacted)),
		},
		{
			Name:     "With all machine driver fields",
			Body:     machineDataInput,
			Expected: machineDataWant,
		},
		{
			Name:     "With no secret uri but secret base type slice",
			Body:     []byte(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":{"testfield":"somesecretencodeddata"},"id":"cattle-system:test","kind":"Opaque"}]}`),
			Expected: []byte(fmt.Sprintf(`{"type":"collection","data":[{"baseType":"namespacedSecret","creatorId":null,"data":"%s","id":"cattle-system:test","kind":"Opaque"}]}`, redacted)),
			Uri:      `/v3/project/local:p-12345/namespacedcertificates?limit=-1&sort=name`,
			Skip:     true,
		},
		{
			Name:     "With kubeconfig from generateKubeconfig action",
			Body:     []byte(`{"baseType":"generateKubeConfigOutput","config":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","type":"generateKubeConfigOutput"}`),
			Expected: []byte(fmt.Sprintf(`{"baseType":"generateKubeConfigOutput","config":"%s","type":"generateKubeConfigOutput"}`, redacted)),
			Uri:      `/v3/clusters/c-xxxxx?action=generateKubeconfig`,
		},
		{
			Name:     "With kubeconfig from connect agent",
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
			Uri:      `/v3/connect/agent`,
		},
		{
			Name:     "With kubeconfig from random request",
			Body:     []byte(`{"kubeConfig":"apiVersion: v1\nkind: Config\nclusters:\n- name: \"somecluster-rke\"\n  cluster:\n    server: \"https://rancherurl.com/k8s/clusters/c-xxxxx\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  cluster:\n    server: \"https://34.211.205.110:6443\"\n    certificate-authority-data: \"somecadata\"\n\nusers:\n- name: \"somecluster-rke\"\n  user:\n    token: \"kubeconfig-user-12345:sometoken\"\n\n\ncontexts:\n- name: \"somecluster-rke\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke\"\n- name: \"somecluster-rke-somecluster-rke1\"\n  context:\n    user: \"somecluster-rke\"\n    cluster: \"somecluster-rke-somecluster-rke1\"\n\ncurrent-context: \"somecluster-rke\"\n","namespace":"testns","secretName":"secret-name"}`),
			Expected: []byte(fmt.Sprintf(`{"kubeConfig":"%s","namespace":"testns","secretName":"secret-name"}`, redacted)),
			Uri:      `asdf`,
		},
		{
			Name:     "With items from sensitiveBodyFields",
			Body:     []byte(`{"credentials":"{'fakeCredName': 'fakeCred'}","applicationSecret":"fakeAppSecret","oauthCredential":"fakeOauth","serviceAccountCredential":"fakeSACred","spKey":"fakeSPKey","spCert":"fakeSPCERT","certificate":"fakeCert","privateKey":"fakeKey"}`),
			Expected: []byte(fmt.Sprintf(`{"credentials":"%s","applicationSecret":"%[1]s","oauthCredential":"%[1]s","serviceAccountCredential":"%[1]s","spKey":"%[1]s","spCert":"%[1]s","certificate":"%[1]s","privateKey":"%[1]s"}`, redacted)),
		},
		{
			Name:     "With malformed input",
			Body:     []byte(`{"key":"value","response":}`),
			Expected: []byte(fmt.Sprintf(`{"%s":"failed to unmarshal request body: invalid character '}' looking for beginning of value"}`, auditLogErrorKey)),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.Skip {
				t.Skipf("skipping test '%s'", c.Name)
			}

			log := &log{
				RequestURI:  c.Uri,
				RequestBody: c.Body,
			}

			err := writer.Write(log)
			assert.NoError(t, err)

			actual := map[string]any{}
			err = json.Unmarshal(log.RequestBody, &actual)
			assert.NoError(t, err)

			expected := map[string]any{}
			err = json.Unmarshal(c.Expected, &expected)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
		})
	}
}
