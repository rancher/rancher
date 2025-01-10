package plugin

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	testCases := []struct {
		Name            string
		CachedPlugins   []*v1.UIPlugin
		ExpectedEntries map[string]*UIPlugin
	}{
		{
			Name: "Generate valid index from controllers cached plugins",
			CachedPlugins: []*v1.UIPlugin{
				{
					Spec: v1.UIPluginSpec{
						Plugin: v1.UIPluginEntry{
							Name:     "test-plugin",
							Version:  "0.1.0",
							Endpoint: "https://test.endpoint.svc",
							NoCache:  false,
							Metadata: map[string]string{
								"test": "data",
							},
						},
					},
				},
				{
					Spec: v1.UIPluginSpec{
						Plugin: v1.UIPluginEntry{
							Name:     "test-plugin-2",
							Version:  "0.1.1",
							Endpoint: "https://test-2.endpoint.svc",
							NoCache:  true,
							Metadata: map[string]string{},
						},
					},
				},
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: &v1.UIPluginEntry{

						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
				"test-plugin-2": {
					UIPluginEntry: &v1.UIPluginEntry{
						Name:     "test-plugin-2",
						Version:  "0.1.1",
						Endpoint: "https://test-2.endpoint.svc",
						NoCache:  true,
						Metadata: map[string]string{},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			index := SafeIndex{}
			err := index.Generate(tc.CachedPlugins)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, tc.ExpectedEntries, index.Entries)
		})
	}
}
