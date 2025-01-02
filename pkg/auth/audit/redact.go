package audit

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

const (
	redacted = "[redacted]"
)

type Redactor interface {
	// Redact replaces sensitive information within a log with "[redacted]". Expects the log to have been prepared.
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
			fmt.Printf("=== [redactor redactMap] 000 '%v' has '%s' ===\n", r.paths, nextPath)
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

// Redact redacts fields and headers which match the
func (r *redactor) Redact(log *log) error {
	r.redactHeaders(log.RequestHeader)
	r.redactHeaders(log.ResponseHeader)

	r.redactMap("", log.unmarshalledRequestBody)
	r.redactMap("", log.unmarshalledResponseBody)

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

func redactSecretsFromBody(log *log, body map[string]any) error {
	var err error

	if body == nil {
		return nil
	}

	isK8sProxyList := strings.HasPrefix(log.RequestURI, "/k8s/") && (body["kind"] != nil && body["kind"] == "SecretList")
	isRegularList := body["type"] != nil && body["type"] == "collection"

	if !(isK8sProxyList || isRegularList) {
		redactSingleSecret(body)

		return nil
	}

	secretListItemsKey := "data"
	if isK8sProxyList {
		secretListItemsKey = "items"
	}

	if _, ok := body[secretListItemsKey]; !ok {
		logrus.Debugf("auditLog: skipping data redaction of secret bodies in secret list: no key [%s] present, no data to redact", secretListItemsKey)
		return nil
	}

	list, ok := body[secretListItemsKey].([]any)
	if !ok {
		logrus.Debugf("auditlog: redacting entire value for key [%s] in resopnse to URI [%s], unable to assert body is of type []any", secretListItemsKey, log.RequestURI)
		body[secretListItemsKey] = redacted
		return nil
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

	return err
}

func redactSecret(log *log) error {
	var err error

	if strings.Contains(log.RequestURI, "secrets") || secretBaseType.Match(log.RequestBody) {
		if err = redactSecretsFromBody(log, log.unmarshalledRequestBody); err != nil {
			return err
		}
	}

	if strings.Contains(log.RequestURI, "secrets") || secretBaseType.Match(log.RequestBody) {
		if err = redactSecretsFromBody(log, log.unmarshalledResponseBody); err != nil {
			return err
		}
	}

	return nil
}
