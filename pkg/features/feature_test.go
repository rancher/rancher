package features

import (
	"fmt"
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

func TestRequireRestarts(t *testing.T) {
	trueVal := true
	falseVal := false

	boolStr := func(b *bool) string {
		if b == nil {
			return "nil"
		}
		return fmt.Sprintf("%v", *b)
	}

	tests := []struct {
		def             bool
		val             *bool
		specVal         *bool
		lockedVal       *bool
		expectedRestart bool
	}{
		// Default false, .spec.value
		// Unset case
		{def: false, val: nil},
		{def: false, val: &trueVal, expectedRestart: true},
		{def: false, val: &falseVal},
		// Set to true case
		{def: false, val: nil, specVal: &trueVal, expectedRestart: true},
		{def: false, val: &trueVal, specVal: &trueVal},
		{def: false, val: &falseVal, specVal: &trueVal, expectedRestart: true},
		// Set to false case
		{def: false, val: nil, specVal: &falseVal},
		{def: false, val: &falseVal, specVal: &falseVal},
		{def: false, val: &trueVal, specVal: &falseVal, expectedRestart: true},

		// Default true, .spec.value
		// Unset case
		{def: true, val: nil},
		{def: true, val: &trueVal},
		{def: true, val: &falseVal, expectedRestart: true},
		// Set to true case
		{def: true, val: nil, specVal: &trueVal},
		{def: true, val: &trueVal, specVal: &trueVal},
		{def: true, val: &falseVal, specVal: &trueVal, expectedRestart: true},
		// Set to false case
		{def: true, val: nil, specVal: &falseVal, expectedRestart: true},
		{def: true, val: &falseVal, specVal: &falseVal},
		{def: true, val: &trueVal, specVal: &falseVal, expectedRestart: true},

		// Default false, .status.lockedValue
		// Unset case
		{def: false, val: nil},
		{def: false, val: &trueVal, expectedRestart: true},
		{def: false, val: &falseVal},
		// Set to true case
		{def: false, val: nil, lockedVal: &trueVal, expectedRestart: true},
		{def: false, val: &trueVal, lockedVal: &trueVal},
		{def: false, val: &falseVal, lockedVal: &trueVal, expectedRestart: true},
		// Set to false case
		{def: false, val: nil, lockedVal: &falseVal},
		{def: false, val: &falseVal, lockedVal: &falseVal},
		{def: false, val: &trueVal, lockedVal: &falseVal, expectedRestart: true},

		// Default true, .status.lockedValue
		// Unset case
		{def: true, val: nil},
		{def: true, val: &trueVal},
		{def: true, val: &falseVal, expectedRestart: true},
		// Set to true case
		{def: true, val: nil, lockedVal: &trueVal},
		{def: true, val: &trueVal, lockedVal: &trueVal},
		{def: true, val: &falseVal, lockedVal: &trueVal, expectedRestart: true},
		// Set to false case
		{def: true, val: nil, lockedVal: &falseVal, expectedRestart: true},
		{def: true, val: &falseVal, lockedVal: &falseVal},
		{def: true, val: &trueVal, lockedVal: &falseVal, expectedRestart: true},
	}
	for _, test := range tests {
		name := fmt.Sprintf("non-dynamic,def=%v,val=%s,specVal=%s,lockedVal=%s", test.def, boolStr(test.val), boolStr(test.specVal), boolStr(test.lockedVal))
		t.Run(name, func(t *testing.T) {
			feat := &Feature{
				dynamic: false,
				def:     test.def,
				val:     test.val,
			}
			featureObj := &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v3.FeatureSpec{
					Value: test.specVal,
				},
				Status: v3.FeatureStatus{
					LockedValue: test.lockedVal,
				},
			}
			needsRestart := RequireRestarts(feat, featureObj)
			assert.Equal(t, test.expectedRestart, needsRestart)
		})
	}
	for _, test := range tests {
		name := fmt.Sprintf("dynamic,def=%v,val=%s,specVal=%s,lockedVal=%s", test.def, boolStr(test.val), boolStr(test.specVal), boolStr(test.lockedVal))
		t.Run(name, func(t *testing.T) {
			feat := &Feature{
				dynamic: true,
				def:     test.def,
				val:     test.val,
			}
			featureObj := &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v3.FeatureSpec{
					Value: test.specVal,
				},
				Status: v3.FeatureStatus{
					LockedValue: test.lockedVal,
				},
			}
			needsRestart := RequireRestarts(feat, featureObj)
			assert.False(t, needsRestart)
		})
	}
}
