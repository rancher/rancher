package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	assert.False(IsDefFalse.Enabled())
	InitializeFeatures(nil, "isfalse=true")
	assert.True(IsDefFalse.Enabled())
}
