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

// X-API-CattleAuth-Header: arbitrary headers=X-Token=my-value,X-User=alice

// Injects literal header values — no secret lookup.
// Multi-header delimiter: comma (',')
// Values are set verbatim; credID is accepted but ignored
type arbitrary struct{}

// X-API-CattleAuth-Header: awsv4 credID=cattle-global-data/my-aws-cred

// Signs the request with AWS SigV4 using accessKey / secretKey from a cloud credential secret.
// Service and region are auto-detected from the target URL hostname
// Secret must have fields accessKey and secretKey
type awsv4 struct{}

// X-API-CattleAuth-Header: basic credID=cattle-global-data/my-cred usernameField=username passwordField=password

// Sets Authorization: Basic base64(username:password).
// usernameField and passwordField name the keys to read from the secret
type basic struct{}

// X-API-CattleAuth-Header: bearer credID=cattle-global-data/my-cred passwordField=token

// Sets Authorization: Bearer <token>.
// passwordField names the key to read from the secret (historical naming; it holds the token)
type bearer struct{}

// X-API-CattleAuth-Header: bodyinject credID=cattle-global-data/my-cred fields=password=passwordField;username=usernameField

// Merges credential values into the top-level JSON request body.
// Multi-field delimiter: semicolon (;)
// Format: <jsonBodyKey>=<secretFieldName>

type bodyinject struct{}

// X-API-CattleAuth-Header: digest credID=cattle-global-data/my-cred usernameField=username passwordField=password

// Performs HTTP Digest auth — makes a pre-flight request to the target to get the WWW-Authenticate challenge, then signs.

// Same secret-field naming convention as basic
// Target endpoint must return 401 with a WWW-Authenticate: Digest ... header on the pre-flight
type digest struct{}

// X-API-CattleAuth-Header: headerinject credID=cattle-global-data/my-cred headers=X-Token=tokenField;X-User=usernameField

// Injects credential values as named HTTP headers by looking them up from a secret.
// Multi-header delimiter: semicolon (;)
// Format: <HeaderName>=<secretFieldName> — right-hand side is the secret key, not a literal value
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
