package audit

import (
	"fmt"
	"regexp"
	"strings"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
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

func isLoginRequest(uri string) bool {
	return strings.Contains(uri, "?action=login")
}
