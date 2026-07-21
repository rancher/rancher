package httpproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// applyInjectionSpec injects credential values into req according to the server-defined spec.
// secretData is a map of secret field name → value, as returned by getCredential.
func applyInjectionSpec(req *http.Request, spec *v3.CredentialInjectionSpec, secretData map[string]string) error {
	switch spec.Mode {
	case "bearer":
		return applyBearer(req, spec, secretData)
	case "basic":
		return applyBasic(req, spec, secretData)
	case "headerinject":
		return applyHeaderInject(req, spec, secretData)
	case "bodyinject":
		return applyBodyInject(req, spec, secretData)
	default:
		return fmt.Errorf("unknown credential injection mode %q", spec.Mode)
	}
}

func applyBearer(req *http.Request, spec *v3.CredentialInjectionSpec, secretData map[string]string) error {
	if spec.TokenField == "" {
		return fmt.Errorf("bearer mode requires tokenField")
	}
	val, ok := secretData[spec.TokenField]
	if !ok {
		return fmt.Errorf("bearer: field %q not found in credential", spec.TokenField)
	}
	req.Header.Set(AuthHeader, "Bearer "+val)
	return nil
}

func applyBasic(req *http.Request, spec *v3.CredentialInjectionSpec, secretData map[string]string) error {
	if spec.UsernameField == "" || spec.PasswordField == "" {
		return fmt.Errorf("basic mode requires usernameField and passwordField")
	}
	username, ok := secretData[spec.UsernameField]
	if !ok {
		return fmt.Errorf("basic: field %q not found in credential", spec.UsernameField)
	}
	password, ok := secretData[spec.PasswordField]
	if !ok {
		return fmt.Errorf("basic: field %q not found in credential", spec.PasswordField)
	}
	encoded := base64.URLEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set(AuthHeader, "Basic "+encoded)
	return nil
}

func applyHeaderInject(req *http.Request, spec *v3.CredentialInjectionSpec, secretData map[string]string) error {
	if len(spec.Fields) == 0 {
		return fmt.Errorf("headerinject mode requires at least one field mapping")
	}
	for _, f := range spec.Fields {
		val, ok := secretData[f.SecretField]
		if !ok {
			return fmt.Errorf("headerinject: field %q not found in credential", f.SecretField)
		}
		req.Header.Set(f.Key, val)
	}
	return nil
}

func applyBodyInject(req *http.Request, spec *v3.CredentialInjectionSpec, secretData map[string]string) error {
	if len(spec.Fields) == 0 {
		return fmt.Errorf("bodyinject mode requires at least one field mapping")
	}

	var body map[string]interface{}
	if req.Body != nil && req.Body != http.NoBody {
		raw, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return fmt.Errorf("bodyinject: failed to read request body: %w", err)
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				return fmt.Errorf("bodyinject: request body is not valid JSON: %w", err)
			}
		}
	}
	if body == nil {
		body = map[string]interface{}{}
	}

	for _, f := range spec.Fields {
		val, ok := secretData[f.SecretField]
		if !ok {
			return fmt.Errorf("bodyinject: field %q not found in credential", f.SecretField)
		}
		body[f.Key] = val
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("bodyinject: failed to encode modified body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(encoded))
	req.ContentLength = int64(len(encoded))
	req.Header.Set("Content-Type", "application/json")
	return nil
}
