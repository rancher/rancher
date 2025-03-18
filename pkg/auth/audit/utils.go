package audit

import (
	"fmt"
	"regexp"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit/jsonpath"
)

func mergeLogVerbosities(lhs auditlogv1.LogVerbosity, rhs auditlogv1.LogVerbosity) auditlogv1.LogVerbosity {
	return auditlogv1.LogVerbosity{
		Request: auditlogv1.Verbosity{
			Headers: lhs.Request.Headers || rhs.Request.Headers,
			Body:    lhs.Request.Body || rhs.Request.Body,
		},
		Response: auditlogv1.Verbosity{
			Headers: lhs.Response.Headers || rhs.Response.Headers,
			Body:    lhs.Response.Body || rhs.Response.Body},
	}
}

func verbosityForLevel(level auditlogv1.Level) auditlogv1.LogVerbosity {
	switch level {
	case auditlogv1.LevelMetadata:
		return auditlogv1.LogVerbosity{
			Request: auditlogv1.Verbosity{
				Headers: true,
			},
			Response: auditlogv1.Verbosity{
				Headers: true,
			},
		}
	case auditlogv1.LevelRequest:
		return auditlogv1.LogVerbosity{
			Request: auditlogv1.Verbosity{
				Headers: true,
				Body:    true,
			},
			Response: auditlogv1.Verbosity{
				Headers: true,
			},
		}
	case auditlogv1.LevelRequestResponse:
		return auditlogv1.LogVerbosity{
			Level: level,
			Request: auditlogv1.Verbosity{
				Headers: true,
				Body:    true,
			},
			Response: auditlogv1.Verbosity{
				Headers: true,
				Body:    true,
			},
		}
	default:
		return auditlogv1.LogVerbosity{}
	}
}

func compileRegexes(s []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, len(s))
	for i, v := range s {
		re, err := regexp.Compile(v)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex: %w", err)
		}
		compiled[i] = re
	}

	return compiled, nil
}

func matchesAny(s string, regexes []*regexp.Regexp) bool {
	for _, re := range regexes {
		if re.MatchString(s) {
			return true
		}
	}

	return false
}

func parsePaths(paths []string) ([]*jsonpath.JSONPath, error) {
	compiled := make([]*jsonpath.JSONPath, len(paths))
	for i, v := range paths {
		jp, err := jsonpath.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse jsonpath: %w", err)
		}
		compiled[i] = jp
	}

	return compiled, nil
}

func pairMatches(v any, f func(string, any) bool) bool {
	switch v := v.(type) {
	case map[string]any:
		for k, v := range v {
			if f(k, v) {
				return true
			}

			if pairMatches(v, f) {
				return true
			}
		}
	case []any:
		for _, v := range v {
			if pairMatches(v, f) {
				return true
			}
		}
	default:
		return false
	}

	return false
}
