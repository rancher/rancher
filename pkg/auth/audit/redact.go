package audit

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit/jsonpath"
	"github.com/sirupsen/logrus"
)

const (
	redacted = "[redacted]"
)

type Redactor interface {
	// Redact replaces sensitive information within a log with "[redacted]". Expects the log to have been prepared.
	Redact(*log) error
}

type policyRedactor struct {
	headers []*regexp.Regexp
	paths   []*jsonpath.JSONPath
}

func NewRedactor(redaction auditlogv1.Redaction) (*policyRedactor, error) {
	headers, err := compileRegexes(redaction.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to compile headers regexes: %w", err)
	}

	paths, err := parsePaths(redaction.Paths)
	if err != nil {
		return nil, fmt.Errorf("failed to parse paths: %w", err)
	}

	return &policyRedactor{
		headers: headers,
		paths:   paths,
	}, nil
}

func (r *policyRedactor) redactHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	for key := range headers {
		if matchesAny(key, r.headers) {
			headers[key] = []string{redacted}
		}
	}
}

// Redact redacts fields and headers which match the
func (r *policyRedactor) Redact(log *log) error {
	r.redactHeaders(log.RequestHeader)
	r.redactHeaders(log.ResponseHeader)

	for _, path := range r.paths {
		path.Set(log.RequestBody, redacted)
		path.Set(log.ResponseBody, redacted)
	}

	return nil
}

// RedactFunc is a function that redacts a log entry in place.
type RedactFunc func(*log) error

func (rf RedactFunc) Redact(log *log) error {
	return rf(log)
}

var (
	secretBaseType    = regexp.MustCompile("^[A-Za-z]*[S|s]ecret$")
	configmapBaseType = regexp.MustCompile("^[A-Za-z]*[C|c]onfig[M|m]ap$")
)

func redactData(secret map[string]any) bool {
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
		return isRedacted
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

	return isRedacted
}

func redactSecretsFromBody(log *log, body map[string]any) error {
	var err error

	if body == nil {
		return nil
	}

	isK8sProxyList := strings.HasPrefix(log.RequestURI, "/k8s/") && (body["kind"] != nil && body["kind"] == "SecretList")
	isRegularList := body["type"] != nil && body["type"] == "collection"

	if !(isK8sProxyList || isRegularList) {
		redactData(body)

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

		redactData(secret)
		list[i] = secret
	}

	return err
}

func redactDataFromBody(log *log, body map[string]any, listKind string) bool {
	var changed bool

	isK8sProxyList := strings.HasPrefix(log.RequestURI, "/k8s") && (body["kind"] != nil && body["kind"] == listKind)
	isRegularList := body["type"] != nil && body["type"] == "collection"

	if !(isK8sProxyList || isRegularList) {
		redactData(body)
		return false
	}

	var itemsKey string
	if isRegularList {
		itemsKey = "data"
	} else {
		itemsKey = "items"
	}

	itemsList, ok := body[itemsKey].([]any)
	if !ok {
		body[itemsKey] = redacted
		return true
	}

	for i, item := range itemsList {
		m, ok := item.(map[string]any)
		if !ok {
			itemsList[i] = redacted
		}

		changed = redactData(m) || changed
		itemsList[i] = m
	}

	if changed {
		body[itemsKey] = itemsList
	}

	return changed
}

func checkForBasetype(pattern *regexp.Regexp) func(string, any) bool {
	return func(k string, v any) bool {
		s, ok := v.(string)
		if !ok {
			return false
		}

		return k == "baseType" && pattern.MatchString(s)
	}
}

func redactSecret(log *log) error {
	if strings.Contains(log.RequestURI, "secrets") || pairMatches(log.RequestBody, checkForBasetype(secretBaseType)) {
		redactDataFromBody(log, log.RequestBody, "SecretList")
	}

	if strings.Contains(log.RequestURI, "secrets") || pairMatches(log.ResponseBody, checkForBasetype(secretBaseType)) {
		redactDataFromBody(log, log.RequestBody, "SecretList")
	}

	return nil
}

func redactConfigMap(log *log) error {
	if strings.Contains(log.RequestURI, "configmaps") || pairMatches(log.RequestBody, checkForBasetype(configmapBaseType)) {
		redactDataFromBody(log, log.RequestBody, "ConfigMapList")
	}

	if strings.Contains(log.RequestURI, "configmaps") || pairMatches(log.ResponseBody, checkForBasetype(configmapBaseType)) {
		redactDataFromBody(log, log.ResponseBody, "ConfigMapList")
	}

	return nil
}

func redactSlice(patterns []*regexp.Regexp, s []any) {
	for i, v := range s {
		switch v := v.(type) {
		case map[string]any:
			redactMap(patterns, v)
		case string:
			// best effort attempt to redact sensitive information in slices containing command arguments.
			// For example, ["command", "login", "--token", "sensitive_info"] should be redacted to ["command", "login", "--token", "[redacted]"
			if strings.HasPrefix(v, "--") && matchesAny(v, patterns) {
				if len(s) > i+1 {
					s[i+1] = redacted
				}
			}
		}
	}
}

func redactMap(patterns []*regexp.Regexp, m map[string]any) {
	for k, v := range m {
		if matchesAny(k, patterns) {
			m[k] = redacted
		}

		switch v := v.(type) {
		case map[string]any:
			redactMap(patterns, v)
		case []any:
			redactSlice(patterns, v)
		}
	}
}

func regexRedactor(patterns []string) (Redactor, error) {
	regexes, err := compileRegexes(patterns)
	if err != nil {
		return nil, err
	}

	return RedactFunc(func(log *log) error {
		redactMap(regexes, log.RequestBody)
		redactMap(regexes, log.ResponseBody)
		return nil
	}), nil
}

func redactImportUrl(l *log) error {
	if strings.HasPrefix(l.RequestURI, "/v3/import") {
		l.RequestURI = "/v3/import/" + redacted
	}

	return nil
}
