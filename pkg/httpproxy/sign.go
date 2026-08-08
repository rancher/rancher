package httpproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
)

type SecretGetter func(namespace, name string) (*v1.Secret, error)

type Signer interface {
	sign(*http.Request, SecretGetter, string) error
}

type arbitrary struct{}

type awsv4 struct{}

type basic struct{}

type bearer struct{}

type bodyinject struct{}

type digest struct{}

type headerinject struct{}

func newSigner(auth string) Signer {
	splitAuth := strings.Split(auth, " ")
	switch strings.ToLower(splitAuth[0]) {
	case "arbitrary":
		return arbitrary{}
	case "awsv4":
		return awsv4{}
	case "basic":
		return basic{}
	case "bearer":
		return bearer{}
	case "bodyinject":
		return bodyinject{}
	case "digest":
		return digest{}
	case "headerinject":
		return headerinject{}
	}
	return nil
}

func (a arbitrary) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, _, err := getAuthData(auth, secrets, []string{})
	if err != nil {
		return err
	}
	fields := []string{"headers"}
	if !requiredFieldsExist(data, fields) {
		return fmt.Errorf("required fields %s not set", fields)
	}
	splitHeaders := strings.Split(data["headers"], ",")
	for _, header := range splitHeaders {
		val := strings.SplitN(header, "=", 2)
		if len(val) != 2 || val[0] == "" || val[1] == "" {
			return fmt.Errorf("arbitrary: malformed header pair %q: expected Name=Value", header)
		}
		req.Header.Set(val[0], val[1])
	}
	return nil
}

func (b basic) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"usernameField", "passwordField", "credID"})
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s:%s", secret[data["usernameField"]], secret[data["passwordField"]])
	encoded := base64.URLEncoding.EncodeToString([]byte(key))
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Basic", encoded))
	return nil
}

func (br bearer) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"passwordField", "credID"})
	if err != nil {
		return err
	}
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Bearer", secret[data["passwordField"]]))
	return nil
}

// bodyinject merges credential values into the JSON body of the proxied request.
// The fields parameter is a semicolon-separated list of jsonKey=secretField pairs.
// Each pair causes the value of secretField in the credential secret to be set at
// jsonKey in the top-level JSON object of the request body. Existing keys are overwritten.
// Example: bodyinject credID=cattle-global-data/my-cred fields=password=passwordField;username=usernameField
func (bi bodyinject) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"fields", "credID"})
	if err != nil {
		return err
	}

	// Read and parse the existing request body.
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

	// Inject each credential field into the body.
	pairs := strings.Split(data["fields"], ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return fmt.Errorf("bodyinject: malformed field pair %q: expected jsonKey=secretField", pair)
		}
		jsonKey, secretField := kv[0], kv[1]
		val, ok := secret[secretField]
		if !ok {
			return fmt.Errorf("bodyinject: field %q not found in credential", secretField)
		}
		body[jsonKey] = val
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

// headerinject injects credential values as HTTP headers on the proxied request.
// The headers parameter is a semicolon-separated list of Name=field pairs, where
// field is the key of the value to read from the referenced credential secret.
// Example: headerinject credID=cattle-global-data/my-cred headers=X-Token=tokenField;X-User=usernameField
func (h headerinject) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"headers", "credID"})
	if err != nil {
		return err
	}
	pairs := strings.Split(data["headers"], ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return fmt.Errorf("headerinject: malformed header pair %q: expected Name=field", pair)
		}
		headerName, fieldName := kv[0], kv[1]
		val, ok := secret[fieldName]
		if !ok {
			return fmt.Errorf("headerinject: field %q not found in credential", fieldName)
		}
		req.Header.Set(headerName, val)
	}
	return nil
}
