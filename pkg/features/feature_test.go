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
	trueVal := true
	falseVal := false

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
		"new feature with lockedOnInstall=true sets LockedValue": {
			featureMock: func() managementv3.FeatureClient {
				mock := fake.NewMockNonNamespacedControllerInterface[*v3.Feature, *v3.FeatureList](gomock.NewController(t))
				mock.EXPECT().Delete("external-rules", &metav1.DeleteOptions{})
				mock.EXPECT().Get("locked-feature", gomock.Any()).Return(nil, fmt.Errorf("not found"))
				mock.EXPECT().Create(gomock.Any()).DoAndReturn(func(feature *v3.Feature) (*v3.Feature, error) {
					assert.Equal(t, "locked-feature", feature.Name)
					assert.NotNil(t, feature.Status.LockedValue)
					assert.True(t, *feature.Status.LockedValue)
					assert.True(t, feature.Status.Default)
					return feature, nil
				})

				return mock
			},
			features: map[string]*Feature{
				"locked-feature": {
					name:            "locked-feature",
					description:     "A locked feature",
					def:             true,
					dynamic:         false,
					install:         true,
					lockedOnInstall: true,
				},
			},
		},
		"new feature with lockedOnInstall=false and def=false sets LockedValue to nil": {
			featureMock: func() managementv3.FeatureClient {
				mock := fake.NewMockNonNamespacedControllerInterface[*v3.Feature, *v3.FeatureList](gomock.NewController(t))
				mock.EXPECT().Delete("external-rules", &metav1.DeleteOptions{})
				mock.EXPECT().Get("unlocked-feature", gomock.Any()).Return(nil, fmt.Errorf("not found"))
				mock.EXPECT().Create(gomock.Any()).DoAndReturn(func(feature *v3.Feature) (*v3.Feature, error) {
					assert.Equal(t, "unlocked-feature", feature.Name)
					assert.Nil(t, feature.Status.LockedValue)
					assert.False(t, feature.Status.Default)
					return feature, nil
				})

				return mock
			},
			features: map[string]*Feature{
				"unlocked-feature": {
					name:            "unlocked-feature",
					description:     "An unlocked feature",
					def:             false,
					dynamic:         true,
					install:         true,
					lockedOnInstall: false,
				},
			},
		},
		"existing feature with lockedOnInstall=false removes LockedValue": {
			featureMock: func() managementv3.FeatureClient {
				mock := fake.NewMockNonNamespacedControllerInterface[*v3.Feature, *v3.FeatureList](gomock.NewController(t))
				mock.EXPECT().Delete("external-rules", &metav1.DeleteOptions{})

				existingFeature := &v3.Feature{
					ObjectMeta: metav1.ObjectMeta{
						Name: "unlocked-feature",
					},
					Spec: v3.FeatureSpec{
						Value: nil,
					},
					Status: v3.FeatureStatus{
						Default:     false,
						Dynamic:     true,
						Description: "An unlocked feature",
						LockedValue: &trueVal, // Has a locked value that should be removed
					},
				}

				mock.EXPECT().Get("unlocked-feature", gomock.Any()).Return(existingFeature, nil)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(feature *v3.Feature) (*v3.Feature, error) {
					assert.Equal(t, "unlocked-feature", feature.Name)
					assert.Nil(t, feature.Status.LockedValue, "LockedValue should be removed when lockedOnInstall=false")
					return feature, nil
				})

				return mock
			},
			features: map[string]*Feature{
				"unlocked-feature": {
					name:            "unlocked-feature",
					description:     "An unlocked feature",
					def:             false,
					dynamic:         true,
					install:         true,
					lockedOnInstall: false, // Not locked, so existing LockedValue should be removed
				},
			},
		},
		"existing feature with lockedOnInstall=true preserves LockedValue": {
			featureMock: func() managementv3.FeatureClient {
				mock := fake.NewMockNonNamespacedControllerInterface[*v3.Feature, *v3.FeatureList](gomock.NewController(t))
				mock.EXPECT().Delete("external-rules", &metav1.DeleteOptions{})

				existingFeature := &v3.Feature{
					ObjectMeta: metav1.ObjectMeta{
						Name: "locked-feature",
					},
					Spec: v3.FeatureSpec{
						Value: &falseVal,
					},
					Status: v3.FeatureStatus{
						Default:     true,
						Dynamic:     false,
						Description: "A locked feature",
						LockedValue: &trueVal,
					},
				}

				mock.EXPECT().Get("locked-feature", gomock.Any()).Return(existingFeature, nil)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(feature *v3.Feature) (*v3.Feature, error) {
					assert.Equal(t, "locked-feature", feature.Name)
					assert.NotNil(t, feature.Status.LockedValue, "LockedValue should be preserved when lockedOnInstall=true")
					assert.True(t, *feature.Status.LockedValue)
					return feature, nil
				})

				return mock
			},
			features: map[string]*Feature{
				"locked-feature": {
					name:            "locked-feature",
					description:     "A locked feature",
					def:             true,
					dynamic:         false,
					install:         true,
					lockedOnInstall: true,
				},
			},
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

func TestPrimeFeatureEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name        string
		primeEnvVal string
		val         *bool
		def         bool
		expected    bool
	}{
		{
			name:        "prime feature on prime build with no value set defaults to enabled",
			primeEnvVal: "prime",
			val:         nil,
			def:         true,
			expected:    true,
		},
		{
			name:        "prime feature on prime build respects explicit false value",
			primeEnvVal: "prime",
			val:         &falseVal,
			def:         true,
			expected:    false,
		},
		{
			name:        "prime feature on prime build respects explicit true value",
			primeEnvVal: "prime",
			val:         &trueVal,
			def:         false,
			expected:    true,
		},
		{
			name:        "prime feature on non-prime build returns false even when val is true",
			primeEnvVal: "",
			val:         &trueVal,
			def:         true,
			expected:    false,
		},
		{
			name:        "prime feature on non-prime build returns false regardless of default",
			primeEnvVal: "",
			val:         nil,
			def:         true,
			expected:    false,
		},
		{
			name:        "non-prime feature on non-prime build is unaffected",
			primeEnvVal: "",
			val:         &trueVal,
			def:         false,
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(primeEnv, test.primeEnvVal)
			feat := &Feature{
				prime: test.name != "non-prime feature on non-prime build is unaffected",
				val:   test.val,
				def:   test.def,
			}
			assert.Equal(t, test.expected, feat.Enabled())
		})
	}
}

func TestIsEnabledPrimeFeature(t *testing.T) {
	trueVal := true

	tests := []struct {
		name        string
		primeEnvVal string
		feature     *v3.Feature
		expected    bool
	}{
		{
			name:        "prime feature CR on non-prime build always returns false",
			primeEnvVal: "",
			feature: &v3.Feature{
				Spec:   v3.FeatureSpec{Value: &trueVal},
				Status: v3.FeatureStatus{Prime: true, Default: true},
			},
			expected: false,
		},
		{
			name:        "prime feature CR on prime build returns spec value",
			primeEnvVal: "prime",
			feature: &v3.Feature{
				Spec:   v3.FeatureSpec{Value: &trueVal},
				Status: v3.FeatureStatus{Prime: true, Default: false},
			},
			expected: true,
		},
		{
			name:        "prime feature CR on non-prime build returns false even with LockedValue true",
			primeEnvVal: "",
			feature: &v3.Feature{
				Spec:   v3.FeatureSpec{Value: &trueVal},
				Status: v3.FeatureStatus{Prime: true, Default: true, LockedValue: &trueVal},
			},
			expected: false,
		},
		{
			name:        "prime feature CR on prime build falls through to default when value and lockedValue are nil",
			primeEnvVal: "prime",
			feature: &v3.Feature{
				Spec:   v3.FeatureSpec{Value: nil},
				Status: v3.FeatureStatus{Prime: true, Default: true},
			},
			expected: true,
		},
		{
			name:        "non-prime feature CR on non-prime build is unaffected",
			primeEnvVal: "",
			feature: &v3.Feature{
				Spec:   v3.FeatureSpec{Value: &trueVal},
				Status: v3.FeatureStatus{Prime: false, Default: false},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(primeEnv, test.primeEnvVal)
			assert.Equal(t, test.expected, IsEnabled(test.feature))
		})
	}
}

func TestRequireRestartsNonPrimeBuild(t *testing.T) {
	trueVal := true
	t.Setenv(primeEnv, "")

	feat := &Feature{
		prime:   true,
		dynamic: false,
		def:     true,
		val:     nil,
	}
	featureObj := &v3.Feature{
		Spec:   v3.FeatureSpec{Value: &trueVal},
		Status: v3.FeatureStatus{Prime: true},
	}
	// Toggling a prime feature on a non-prime build never requires restart
	// because the feature is unconditionally disabled.
	assert.False(t, RequireRestarts(feat, featureObj))
}
