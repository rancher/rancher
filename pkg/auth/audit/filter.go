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

func (m *Filter) Allowed(log *Log) bool {
	if m.uri.MatchString(log.RequestURI) {
		return m.action == auditlogv1.FilterActionAllow
	}

	return false
}
