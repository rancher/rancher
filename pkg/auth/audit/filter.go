package audit

import (
	"fmt"
	"regexp"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
)

type Filter struct {
	action auditlogv1.FilterAction
	uri    *regexp.Regexp
}

func NewFilter(filter auditlogv1.Filter) (*Filter, error) {
	compiled, err := regexp.Compile(filter.RequestURI)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex '%s': %w", filter.RequestURI, err)
	}

	return &Filter{
		action: filter.Action,
		uri:    compiled,
	}, nil
}

// Allowed returns the filter action if the regex matches, otherwise Unknown.
// Unknown means "this filter does not apply to the request URI".
func (m *Filter) Allowed(requestURI string) auditlogv1.FilterAction {
	if m.uri.MatchString(requestURI) {
		return m.action
	}

	return auditlogv1.FilterActionUnknown
}

func (m *Filter) LogAllowed(log *logEntry) auditlogv1.FilterAction {
	return m.Allowed(log.RequestURI)
}
