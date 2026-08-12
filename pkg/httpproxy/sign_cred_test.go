package httpproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// makeSecretGetter returns a SecretGetter that always returns a secret with the given fields.
func makeSecretGetter(fields map[string]string) SecretGetter {
	return func(namespace, name string) (*corev1.Secret, error) {
		data := make(map[string][]byte, len(fields))
		for k, v := range fields {
			data[k] = []byte(v)
		}
		return &corev1.Secret{Data: data}, nil
	}
}

// errSecretGetter returns a SecretGetter that always errors.
func errSecretGetter(msg string) SecretGetter {
	return func(namespace, name string) (*corev1.Secret, error) {
		return nil, fmt.Errorf("%s", msg)
	}
}

// makeRequest builds a minimal *http.Request with an optional JSON body.
func makeRequest(body map[string]interface{}) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "http://example.com/api", nil)
	if body != nil {
		encoded, _ := json.Marshal(body)
		req.Body = io.NopCloser(bytes.NewReader(encoded))
		req.ContentLength = int64(len(encoded))
	}
	return req
}

// --- arbitrary signer ---

func TestArbitrarySign_WithCredential(t *testing.T) {
	// Specifying credID means we need an actual secret-getter
	sg := makeSecretGetter(nil)
	req := makeRequest(nil)
	auth := "arbitrary credID=cattle-global-data/my-cred headers=X-Token=secret-token-value,X-User=alice"
	err := arbitrary{}.sign(req, sg, auth)
	require.NoError(t, err)
	assert.Equal(t, "secret-token-value", req.Header.Get("X-Token"))
	assert.Equal(t, "alice", req.Header.Get("X-User"))
}

func TestArbitrarySign_WithoutCredential_LiteralValues(t *testing.T) {
	// No credID — values should be treated as literals (backward compat).
	req := makeRequest(nil)
	auth := "arbitrary headers=X-Static=literal-value"
	err := arbitrary{}.sign(req, nil, auth)
	require.NoError(t, err)
	assert.Equal(t, "literal-value", req.Header.Get("X-Static"))
}

func TestArbitrarySign_MissingHeadersParam(t *testing.T) {
	req := makeRequest(nil)
	auth := "arbitrary credID=cattle-global-data/my-cred"
	err := arbitrary{}.sign(req, makeSecretGetter(nil), auth)
	assert.ErrorContains(t, err, "required fields")
}

func TestArbitrarySign_MalformedPair(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"tokenField": "val"})
	req := makeRequest(nil)
	// "X-Token" has no "=" separator
	auth := "arbitrary credID=cattle-global-data/my-cred headers=X-Token"
	err := arbitrary{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, "malformed header pair")
}

func TestArbitrarySign_FieldNotInSecret(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"otherField": "val"})
	req := makeRequest(nil)
	auth := "arbitrary credID=cattle-global-data/my-cred headers=X-Token=tokenField"
	err := arbitrary{}.sign(req, sg, auth)
	require.NoError(t, err)
	assert.Equal(t, "tokenField", req.Header.Get("X-Token"))
}

func TestArbitrarySign_SecretGetterError(t *testing.T) {
	req := makeRequest(nil)
	auth := "arbitrary credID=cattle-global-data/my-cred headers=X-Token=tokenField"
	err := arbitrary{}.sign(req, errSecretGetter("secret not found"), auth)
	assert.ErrorContains(t, err, "secret not found")
}

func TestArbitrarySign_MultipleHeaders(t *testing.T) {
	sg := makeSecretGetter(nil)
	req := makeRequest(nil)
	auth := "arbitrary credID=cattle-global-data/my-cred headers=H1=v1,H2=v2,H3=v3"
	err := arbitrary{}.sign(req, sg, auth)
	require.NoError(t, err)
	assert.Equal(t, "v1", req.Header.Get("H1"))
	assert.Equal(t, "v2", req.Header.Get("H2"))
	assert.Equal(t, "v3", req.Header.Get("H3"))
}

// --- headerinject signer ---

func TestHeaderInject_WithCredential(t *testing.T) {
	sg := makeSecretGetter(map[string]string{
		"tokenField":    "secret-token-value",
		"usernameField": "alice",
	})
	req := makeRequest(nil)
	auth := "headerinject credID=cattle-global-data/my-cred headers=X-Token=tokenField;X-User=usernameField"
	err := headerinject{}.sign(req, sg, auth)
	require.NoError(t, err)
	assert.Equal(t, "secret-token-value", req.Header.Get("X-Token"))
	assert.Equal(t, "alice", req.Header.Get("X-User"))
}

func TestHeaderInject_MalformedPair(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"tokenField": "val"})
	req := makeRequest(nil)
	auth := "headerinject credID=cattle-global-data/my-cred headers=X-Token"
	err := headerinject{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, "malformed header pair")
}

func TestHeaderInject_FieldNotInSecret(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"otherField": "val"})
	req := makeRequest(nil)
	auth := "headerinject credID=cattle-global-data/my-cred headers=X-Token=tokenField"
	err := headerinject{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, `field "tokenField" not found in credential`)
}

func TestSigner_HeaderDelimiterContract(t *testing.T) {
	type testCase struct {
		description  string
		signer       Signer
		auth         string
		secretFields map[string]string
		expected     map[string]string
		expectedErr  string
	}

	tests := []testCase{
		{
			description: "arbitrary uses comma delimiter for multiple headers",
			signer:      arbitrary{},
			auth:        "arbitrary headers=H1=v1,H2=v2",
			expected:    map[string]string{"H1": "v1", "H2": "v2"},
		},
		{
			description: "arbitrary treats semicolon as part of literal value (no multi-header split)",
			signer:      arbitrary{},
			auth:        "arbitrary headers=H1=v1;H2=v2",
			expected:    map[string]string{"H1": "v1;H2=v2"},
		},
		{
			description: "headerinject uses semicolon delimiter for multiple headers",
			signer:      headerinject{},
			auth:        "headerinject credID=cattle-global-data/my-cred headers=H1=f1;H2=f2",
			secretFields: map[string]string{
				"f1": "v1",
				"f2": "v2",
			},
			expected: map[string]string{"H1": "v1", "H2": "v2"},
		},
		{
			description: "headerinject does not accept comma-delimited multi-header syntax",
			signer:      headerinject{},
			auth:        "headerinject credID=cattle-global-data/my-cred headers=H1=f1,H2=f2",
			secretFields: map[string]string{
				"f1": "v1",
				"f2": "v2",
			},
			expectedErr: `field "f1,H2=f2" not found in credential`,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := makeRequest(nil)
			err := test.signer.sign(req, makeSecretGetter(test.secretFields), test.auth)

			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)
			for header, val := range test.expected {
				assert.Equal(t, val, req.Header.Get(header))
			}
		})
	}
}

// --- bodyinject signer ---

func TestBodyInject_IntoExistingBody(t *testing.T) {
	sg := makeSecretGetter(map[string]string{
		"passwordField": "s3cr3t",
		"usernameField": "bob",
	})
	req := makeRequest(map[string]interface{}{"existing": "value"})
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password=passwordField;username=usernameField"
	err := bodyinject{}.sign(req, sg, auth)
	require.NoError(t, err)

	var result map[string]interface{}
	body, _ := io.ReadAll(req.Body)
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "s3cr3t", result["password"])
	assert.Equal(t, "bob", result["username"])
	assert.Equal(t, "value", result["existing"], "pre-existing keys should be preserved")
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, int64(len(body)), req.ContentLength)
}

func TestBodyInject_EmptyBody(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"tokenField": "tok"})
	req, _ := http.NewRequest(http.MethodPost, "http://example.com/api", nil)
	auth := "bodyinject credID=cattle-global-data/my-cred fields=token=tokenField"
	err := bodyinject{}.sign(req, sg, auth)
	require.NoError(t, err)

	var result map[string]interface{}
	body, _ := io.ReadAll(req.Body)
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "tok", result["token"])
}

func TestBodyInject_OverwritesExistingKey(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"passwordField": "new-pass"})
	req := makeRequest(map[string]interface{}{"password": "old-pass"})
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password=passwordField"
	err := bodyinject{}.sign(req, sg, auth)
	require.NoError(t, err)

	var result map[string]interface{}
	body, _ := io.ReadAll(req.Body)
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "new-pass", result["password"])
}

func TestBodyInject_MissingCredID(t *testing.T) {
	req := makeRequest(nil)
	auth := "bodyinject fields=password=passwordField"
	err := bodyinject{}.sign(req, makeSecretGetter(nil), auth)
	assert.ErrorContains(t, err, "required fields")
}

func TestBodyInject_MissingFieldsParam(t *testing.T) {
	req := makeRequest(nil)
	auth := "bodyinject credID=cattle-global-data/my-cred"
	err := bodyinject{}.sign(req, makeSecretGetter(nil), auth)
	assert.ErrorContains(t, err, "required fields")
}

func TestBodyInject_MalformedFieldPair(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"passwordField": "pass"})
	req := makeRequest(nil)
	// "password" has no "=" separator
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password"
	err := bodyinject{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, "malformed field pair")
}

func TestBodyInject_FieldNotInSecret(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"otherField": "val"})
	req := makeRequest(nil)
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password=passwordField"
	err := bodyinject{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, `field "passwordField" not found in credential`)
}

func TestBodyInject_InvalidJSONBody(t *testing.T) {
	sg := makeSecretGetter(map[string]string{"passwordField": "pass"})
	req, _ := http.NewRequest(http.MethodPost, "http://example.com/api", bytes.NewBufferString("not-json"))
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password=passwordField"
	err := bodyinject{}.sign(req, sg, auth)
	assert.ErrorContains(t, err, "not valid JSON")
}

func TestBodyInject_SecretGetterError(t *testing.T) {
	req := makeRequest(nil)
	auth := "bodyinject credID=cattle-global-data/my-cred fields=password=passwordField"
	err := bodyinject{}.sign(req, errSecretGetter("secret unavailable"), auth)
	assert.ErrorContains(t, err, "secret unavailable")
}

func TestNewSigner_BodyInject(t *testing.T) {
	assert.IsType(t, bodyinject{}, newSigner("bodyinject credID=x fields=y=z"))
}

func TestNewSigner_Arbitrary(t *testing.T) {
	assert.IsType(t, arbitrary{}, newSigner("arbitrary headers=X-Foo=bar"))
}

func TestNewSigner_HeaderInject(t *testing.T) {
	assert.IsType(t, headerinject{}, newSigner("headerinject credID=x headers=X-Foo=bar"))
}
