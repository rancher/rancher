package httpproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRequest returns a minimal POST request with an optional JSON body.
func newTestRequest(t *testing.T, body map[string]interface{}) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://example.com/api", nil)
	require.NoError(t, err)
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		req.Body = io.NopCloser(bytes.NewReader(encoded))
		req.ContentLength = int64(len(encoded))
	}
	return req
}

// readJSONBody parses the request body as a JSON object.
func readJSONBody(t *testing.T, req *http.Request) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &result))
	return result
}

// --- applyInjectionSpec dispatch ---

func TestApplyInjectionSpec_UnknownMode(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "nonsense"}
	err := applyInjectionSpec(req, spec, map[string]string{})
	assert.ErrorContains(t, err, `unknown credential injection mode "nonsense"`)
}

// --- bearer ---

func TestApplyBearer_SetsAuthorizationHeader(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "bearer", TokenField: "token"}
	err := applyInjectionSpec(req, spec, map[string]string{"token": "my-secret-token"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-secret-token", req.Header.Get("Authorization"))
}

func TestApplyBearer_MissingTokenField(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "bearer"}
	err := applyInjectionSpec(req, spec, map[string]string{"token": "val"})
	assert.ErrorContains(t, err, "tokenField")
}

func TestApplyBearer_FieldNotInSecret(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "bearer", TokenField: "token"}
	err := applyInjectionSpec(req, spec, map[string]string{"other": "val"})
	assert.ErrorContains(t, err, `"token" not found in credential`)
}

func TestApplyBearer_EmptyTokenValue(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "bearer", TokenField: "token"}
	err := applyInjectionSpec(req, spec, map[string]string{"token": ""})
	require.NoError(t, err)
	assert.Equal(t, "Bearer ", req.Header.Get("Authorization"))
}

// --- basic ---

func TestApplyBasic_SetsBase64EncodedAuthorizationHeader(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:          "basic",
		UsernameField: "user",
		PasswordField: "pass",
	}
	err := applyInjectionSpec(req, spec, map[string]string{"user": "alice", "pass": "s3cr3t"})
	require.NoError(t, err)

	authHeader := req.Header.Get("Authorization")
	require.True(t, len(authHeader) > 6)
	assert.Equal(t, "Basic ", authHeader[:6])
	decoded, err := base64.URLEncoding.DecodeString(authHeader[6:])
	require.NoError(t, err)
	assert.Equal(t, "alice:s3cr3t", string(decoded))
}

func TestApplyBasic_MissingUsernameField(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "basic", PasswordField: "pass"}
	err := applyInjectionSpec(req, spec, map[string]string{"pass": "p"})
	assert.ErrorContains(t, err, "usernameField")
}

func TestApplyBasic_MissingPasswordField(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "basic", UsernameField: "user"}
	err := applyInjectionSpec(req, spec, map[string]string{"user": "u"})
	assert.ErrorContains(t, err, "passwordField")
}

func TestApplyBasic_UsernameFieldNotInSecret(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "basic", UsernameField: "user", PasswordField: "pass"}
	err := applyInjectionSpec(req, spec, map[string]string{"pass": "p"})
	assert.ErrorContains(t, err, `"user" not found in credential`)
}

func TestApplyBasic_PasswordFieldNotInSecret(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "basic", UsernameField: "user", PasswordField: "pass"}
	err := applyInjectionSpec(req, spec, map[string]string{"user": "u"})
	assert.ErrorContains(t, err, `"pass" not found in credential`)
}

// --- headerinject ---

func TestApplyHeaderInject_SetsSingleHeader(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:   "headerinject",
		Fields: []v3.InjectionFieldMapping{{Key: "X-API-Key", SecretField: "apiKey"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"apiKey": "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "abc123", req.Header.Get("X-API-Key"))
}

func TestApplyHeaderInject_SetsMultipleHeaders(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode: "headerinject",
		Fields: []v3.InjectionFieldMapping{
			{Key: "X-Token", SecretField: "token"},
			{Key: "X-User", SecretField: "user"},
		},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"token": "tok", "user": "bob"})
	require.NoError(t, err)
	assert.Equal(t, "tok", req.Header.Get("X-Token"))
	assert.Equal(t, "bob", req.Header.Get("X-User"))
}

func TestApplyHeaderInject_NoFieldMappings(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "headerinject"}
	err := applyInjectionSpec(req, spec, map[string]string{})
	assert.ErrorContains(t, err, "at least one field mapping")
}

func TestApplyHeaderInject_FieldNotInSecret(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:   "headerinject",
		Fields: []v3.InjectionFieldMapping{{Key: "X-Token", SecretField: "token"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"other": "val"})
	assert.ErrorContains(t, err, `"token" not found in credential`)
}

func TestApplyHeaderInject_OverwritesExistingHeader(t *testing.T) {
	req := newTestRequest(t, nil)
	req.Header.Set("X-Token", "old-value")
	spec := &v3.CredentialInjectionSpec{
		Mode:   "headerinject",
		Fields: []v3.InjectionFieldMapping{{Key: "X-Token", SecretField: "token"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"token": "new-value"})
	require.NoError(t, err)
	assert.Equal(t, "new-value", req.Header.Get("X-Token"))
}

// --- bodyinject ---

func TestApplyBodyInject_IntoExistingBody(t *testing.T) {
	req := newTestRequest(t, map[string]interface{}{"existing": "value"})
	spec := &v3.CredentialInjectionSpec{
		Mode: "bodyinject",
		Fields: []v3.InjectionFieldMapping{
			{Key: "password", SecretField: "pass"},
			{Key: "username", SecretField: "user"},
		},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"pass": "s3cr3t", "user": "alice"})
	require.NoError(t, err)

	result := readJSONBody(t, req)
	assert.Equal(t, "s3cr3t", result["password"])
	assert.Equal(t, "alice", result["username"])
	assert.Equal(t, "value", result["existing"], "pre-existing keys must be preserved")
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestApplyBodyInject_EmptyBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/api", nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:   "bodyinject",
		Fields: []v3.InjectionFieldMapping{{Key: "token", SecretField: "tok"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"tok": "abc"})
	require.NoError(t, err)

	result := readJSONBody(t, req)
	assert.Equal(t, "abc", result["token"])
}

func TestApplyBodyInject_OverwritesExistingKey(t *testing.T) {
	req := newTestRequest(t, map[string]interface{}{"password": "old"})
	spec := &v3.CredentialInjectionSpec{
		Mode:   "bodyinject",
		Fields: []v3.InjectionFieldMapping{{Key: "password", SecretField: "pass"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"pass": "new"})
	require.NoError(t, err)

	result := readJSONBody(t, req)
	assert.Equal(t, "new", result["password"])
}

func TestApplyBodyInject_SetsContentLengthAndContentType(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:   "bodyinject",
		Fields: []v3.InjectionFieldMapping{{Key: "k", SecretField: "f"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"f": "v"})
	require.NoError(t, err)

	body, _ := io.ReadAll(req.Body)
	assert.Equal(t, int64(len(body)), req.ContentLength)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestApplyBodyInject_NoFieldMappings(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{Mode: "bodyinject"}
	err := applyInjectionSpec(req, spec, map[string]string{})
	assert.ErrorContains(t, err, "at least one field mapping")
}

func TestApplyBodyInject_FieldNotInSecret(t *testing.T) {
	req := newTestRequest(t, nil)
	spec := &v3.CredentialInjectionSpec{
		Mode:   "bodyinject",
		Fields: []v3.InjectionFieldMapping{{Key: "password", SecretField: "pass"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"other": "val"})
	assert.ErrorContains(t, err, `"pass" not found in credential`)
}

func TestApplyBodyInject_InvalidJSONBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/api", bytes.NewBufferString("not-json"))
	spec := &v3.CredentialInjectionSpec{
		Mode:   "bodyinject",
		Fields: []v3.InjectionFieldMapping{{Key: "password", SecretField: "pass"}},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"pass": "p"})
	assert.ErrorContains(t, err, "not valid JSON")
}

func TestApplyBodyInject_MultipleFieldsMergedCorrectly(t *testing.T) {
	req := newTestRequest(t, map[string]interface{}{"keep": true})
	spec := &v3.CredentialInjectionSpec{
		Mode: "bodyinject",
		Fields: []v3.InjectionFieldMapping{
			{Key: "a", SecretField: "fa"},
			{Key: "b", SecretField: "fb"},
			{Key: "c", SecretField: "fc"},
		},
	}
	err := applyInjectionSpec(req, spec, map[string]string{"fa": "1", "fb": "2", "fc": "3"})
	require.NoError(t, err)

	result := readJSONBody(t, req)
	assert.Equal(t, "1", result["a"])
	assert.Equal(t, "2", result["b"])
	assert.Equal(t, "3", result["c"])
	assert.Equal(t, true, result["keep"])
}
