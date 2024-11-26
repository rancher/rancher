package features

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var isDefFalse = newFeature("isfalse", "", false, false, true)

// TestApplyArgumentDefaults ensure that applyArgumentsDefault accepts argument
// of the form "features=feature1=bool,feature2=bool" and nothing else
func TestApplyArgumentDefaults(t *testing.T) {
	assert := assert.New(t)

	// each string contained below is expected to cause an error when passed
	// to applyArgumentDefaults
	invalidArguments := []string{
		"asdf",                    // argument must be of the form feature1=bool
		"invalidfeature=true",     // left hand of assignment must be existing feature
		"invalidfeature=notabool", // right-hand of assignment but be bool parsable
		"=asdf",
		"=",
		"invalid=invalid,=",
		",feature=true",
		"invalidfeature=true,invalidfeature2=false",
		"feature = false",
	}

	for _, arg := range invalidArguments {
		err := applyArgumentDefaults(arg)
		assert.NotNil(err)
	}
}

func TestInitializeNil(t *testing.T) {
	assert := assert.New(t)
	assert.False(isDefFalse.Enabled())
	InitializeFeatures(nil, "isfalse=true")
	assert.True(isDefFalse.Enabled())
}

func TestInitializeFeatures(t *testing.T) {
	tests := map[string]struct {
		featureMock func() managementv3.FeatureClient
		features    map[string]*Feature
	}{
		"delete external-rules feature is called": {
			featureMock: func() managementv3.FeatureClient {
				mock := fake.NewMockNonNamespacedControllerInterface[*v3.Feature, *v3.FeatureList](gomock.NewController(t))
				mock.EXPECT().Delete("external-rules", &metav1.DeleteOptions{})

				return mock
			},
			features: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			features = test.features // we can't run parallel tests if we modify this package var here. Code should be refactored to not use a package var if more tests cases are added.
			InitializeFeatures(test.featureMock(), "")
		})
	}
}
