package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

const (
	redacted         = "[redacted]"
	auditLogErrorKey = "auditLogError"
)

type Redactor interface {
	Redact(*log) error
}

// todo: we can probably combine the regex slices into a single regex for better performance
type redactor struct {
	headers []*regexp.Regexp
	paths   []string
	keys    []*regexp.Regexp
}

func NewRedactor(redaction auditlogv1.Redaction) (*redactor, error) {
	headers, err := compileRegexes(redaction.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to compile headers regexes: %w", err)
	}

	keys, err := compileRegexes(redaction.Keys)
	if err != nil {
		return nil, fmt.Errorf("failed to compile keys regexes: %w", err)
	}

	return &redactor{
		headers: headers,
		paths:   redaction.Paths,
		keys:    keys,
	}, nil
}

func (r *redactor) redactHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	for key := range headers {
		if matchesAny(key, r.headers) {
			headers[key] = []string{redacted}
		}
	}
}

func (r *redactor) redactSlice(path string, slice []any) {
	for i, value := range slice {
		switch value := value.(type) {
		case map[string]any:
			r.redactMap(path, value)
		case string:
			// best effort attempt to redact sensitive information in slices containing command arguments.
			// For example, ["command", "login", "--token", "sensitive_info"] should be redacted to ["command", "login", "--token", "[redacted]"
			if strings.HasPrefix(value, "--") && matchesAny(value, r.keys) {
				if len(slice) > i+1 {
					slice[i+1] = redacted
				}
			}
		}
	}
}

func (r *redactor) redactMap(path string, body map[string]any) {
	for key, value := range body {
		nextPath := strings.Join([]string{path, key}, ".")
		if slices.Contains(r.paths, nextPath) {
			body[key] = redacted
			continue
		}

		if matchesAny(key, r.keys) {
			body[key] = redacted
			continue
		}

		switch value := value.(type) {
		case map[string]any:
			r.redactMap(nextPath, value)
		case []any:
			r.redactSlice(nextPath+"[]", value)
		}
	}
}

// todo: we should unmarshal the body in to a map[string]any
// todo: consider adding a [Request|Response]BodyUnmarshaled
func (r *redactor) redactBody(data []byte) ([]byte, error) {
	if data == nil {
		return data, nil
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		body := map[string]string{
			auditLogErrorKey: fmt.Sprintf("failed to unmarshal log body: %s", err),
		}

		if data, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log body: %w", err)
		}

		return data, nil
	}

	r.redactMap("", body)

	redacted, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log body: %w", err)
	}

	return redacted, nil
}

// Redact redacts fields and headers which match the
func (r *redactor) Redact(log *log) error {
	r.redactHeaders(log.RequestHeader)
	r.redactHeaders(log.ResponseHeader)

	var err error

	log.RequestBody, err = r.redactBody(log.RequestBody)
	if err != nil {
		return err
	}

	log.ResponseBody, err = r.redactBody(log.ResponseBody)
	if err != nil {
		return err
	}

	return nil
}

// RedactFunc is a function that redacts a log entry in place.
type RedactFunc func(*log) error

func (rf RedactFunc) Redact(log *log) error {
	return rf(log)
}

var (
	secretBaseType = regexp.MustCompile(".\"baseType\":\"([A-Za-z]*[S|s]ecret)\".")
)

func redactSingleSecret(secret map[string]any) {
	var isRedacted bool

	if secret["data"] != nil {
		secret["data"] = redacted
		isRedacted = true
	}

	if secret["stringData"] != nil {
		secret["stringData"] = redacted
		isRedacted = true
	}

	if isRedacted {
		return
	}

	for key := range secret {
		switch key {
		case "id", "created", "baseType":
			// censorAll is used when the secret is formatted in such a way where its
			// data fields cannot be distinguished from its other fields. In this case
			// most of the data is redacted apart from "id", "baseType", "key"
			continue
		}

		secret[key] = redacted
	}
}

func redactSecretsFromBody(log *log, data []byte) ([]byte, error) {
	var err error

	if data == nil {
		return data, nil
	}

	body := map[string]any{}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("failed to unmarshal log body: %w", err)
	}

	isK8sProxyList := strings.HasPrefix(log.RequestURI, "/k8s/") && (body["kind"] != nil && body["kind"] == "SecretList")
	isRegularList := body["type"] != nil && body["type"] == "collection"

	if !(isK8sProxyList || isRegularList) {
		redactSingleSecret(body)

		if data, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("failed to marshal redacted secret: %w", err)
		}

		return data, nil
	}

	secretListItemsKey := "data"
	if isK8sProxyList {
		secretListItemsKey = "items"
	}

	if _, ok := body[secretListItemsKey]; !ok {
		logrus.Debugf("auditLog: skipping data redaction of secret bodies in secret list: no key [%s] present, no data to redact", secretListItemsKey)
		return data, nil
	}

	list, ok := body[secretListItemsKey].([]any)
	if !ok {
		logrus.Debugf("auditlog: redacting entire value for key [%s] in resopnse to URI [%s], unable to assert body is of type []any", secretListItemsKey, log.RequestURI)
		body[secretListItemsKey] = redacted
		return data, nil
	}

	for i, s := range list {
		secret, ok := s.(map[string]any)
		if !ok {
			logrus.Debugf("auditlog: redacting entire value for key [%s] in response to URI [%s], unable to assert body is of type map[string]any", secretListItemsKey, log.RequestURI)
			list[i] = redacted
			continue
		}

		redactSingleSecret(secret)
		list[i] = secret
	}

	if data, err = json.Marshal(body); err != nil {
		return nil, fmt.Errorf("failed to marshal redacted secret list: %w", err)
	}

	return data, err
}

func redactSecret(log *log) error {
	var err error

	if strings.Contains(log.RequestURI, "secrets") || secretBaseType.Match(log.RequestBody) {
		if log.RequestBody, err = redactSecretsFromBody(log, log.RequestBody); err != nil {
			return err
		}
	}

	if strings.Contains(log.RequestURI, "secrets") || secretBaseType.Match(log.RequestBody) {
		if log.ResponseBody, err = redactSecretsFromBody(log, log.ResponseBody); err != nil {
			return err
		}
	}

	return nil
}
