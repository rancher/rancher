package cluster

import (
	"testing"

	"github.com/rancher/norman/types"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

// stubURLBuilder is a minimal implementation of types.URLBuilder for testing.
type stubURLBuilder struct{}

func (s *stubURLBuilder) Current() string { return "http://rancher.test" }
func (s *stubURLBuilder) Collection(_ *types.Schema, _ *types.APIVersion) string { return "" }
func (s *stubURLBuilder) CollectionAction(_ *types.Schema, _ *types.APIVersion, _ string) string {
	return ""
}
func (s *stubURLBuilder) SubContextCollection(_ *types.Schema, _ string, _ *types.Schema) string {
	return ""
}
func (s *stubURLBuilder) SchemaLink(_ *types.Schema) string                        { return "" }
func (s *stubURLBuilder) ResourceLink(resource *types.RawResource) string {
	return "http://rancher.test/v3/clusters/" + resource.ID
}
func (s *stubURLBuilder) Link(linkName string, resource *types.RawResource) string {
	return "http://rancher.test/v3/clusters/" + resource.ID + "/" + linkName
}
func (s *stubURLBuilder) RelativeToRoot(path string) string            { return path }
func (s *stubURLBuilder) Version(_ types.APIVersion) string            { return "" }
func (s *stubURLBuilder) Marker(marker string) string                  { return marker }
func (s *stubURLBuilder) ReverseSort(_ types.SortOrder) string         { return "" }
func (s *stubURLBuilder) Sort(_ string) string                         { return "" }
func (s *stubURLBuilder) SetSubContext(_ string)                        {}
func (s *stubURLBuilder) FilterLink(_ *types.Schema, _, _ string) string { return "" }
func (s *stubURLBuilder) Action(action string, resource *types.RawResource) string {
	return "http://rancher.test/v3/clusters/" + resource.ID + "?action=" + action
}
func (s *stubURLBuilder) ResourceLinkByID(_ *types.Schema, _ string) string         { return "" }
func (s *stubURLBuilder) ActionLinkByID(_ *types.Schema, _, _ string) string        { return "" }

func TestFormatter_ShellLink(t *testing.T) {
	tests := []struct {
		name           string
		featureEnabled bool
		wantShellLink  bool
	}{
		{
			name:           "shell link present when ClusterShell feature enabled",
			featureEnabled: true,
			wantShellLink:  true,
		},
		{
			name:           "shell link absent when ClusterShell feature disabled",
			featureEnabled: false,
			wantShellLink:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features.ClusterShell.Set(tt.featureEnabled)
			defer features.ClusterShell.Unset()

			formatter := &Formatter{
				nodeLister: &fakes.NodeListerMock{
					ListFunc: func(_ string, _ labels.Selector) ([]*apimgmtv3.Node, error) {
						return nil, nil
					},
				},
			}

			resource := &types.RawResource{
				ID:      "test-cluster",
				Schema:  &types.Schema{},
				Links:   map[string]string{},
				Actions: map[string]string{},
				Values:  map[string]interface{}{},
			}

			apiCtx := &types.APIContext{
				URLBuilder: &stubURLBuilder{},
			}

			formatter.Formatter(apiCtx, resource)

			if tt.wantShellLink {
				assert.Contains(t, resource.Links, "shell", "expected shell link to be present")
				assert.Contains(t, resource.Links["shell"], "ws://", "expected shell link to use ws scheme")
				assert.Contains(t, resource.Links["shell"], "?shell=true", "expected shell link query param")
			} else {
				assert.NotContains(t, resource.Links, "shell", "expected shell link to be absent")
			}
		})
	}
}