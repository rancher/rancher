package audit

import (
	"regexp"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	type testCase struct {
		Name    string
		Filter  Filter
		log     Log
		Allowed bool
	}

	cases := []testCase{
		{
			Name: "Allow All",
			Filter: Filter{
				action: auditlogv1.FilterActionAllow,
				uri:    regexp.MustCompile(".*"),
			},
			log: Log{
				RequestURI: "/api/v1/namespaces/default/pods",
			},
			Allowed: true,
		},
		{
			Name: "Deny All",
			Filter: Filter{
				action: auditlogv1.FilterActionDeny,
				uri:    regexp.MustCompile(".*"),
			},
			log: Log{
				RequestURI: "/api/v1/namespaces/default/pods",
			},
			Allowed: false,
		},

		{
			Name: "Block Secret Operations",
			Filter: Filter{
				action: auditlogv1.FilterActionDeny,
				uri:    regexp.MustCompile("/api/v1/namespaces/.*/secrets.*"),
			},
			log: Log{
				RequestURI: "/api/v1/namespaces/default/secrets/my-secret",
			},
			Allowed: false,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual := c.Filter.Allowed(&c.log)
			assert.Equal(t, c.Allowed, actual)
		})
	}
}
