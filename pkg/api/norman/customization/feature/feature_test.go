package feature

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
)

// the effective value of a feature should be value if one is assigned, otherwise
// it should reflect the default value
func TestFormatter(t *testing.T) {
	assert := assert.New(t)

	// ensure value is set to default when it has not been set by user
	testResource := &types.RawResource{
		Values: map[string]interface{}{
			"status": map[string]interface{}{
				"default": true,
			},
		},
	}

	assert.Equal(true, getEffectiveValue(testResource))

	// ensure value is not set to default when it has been set by user
	testResource.Values["value"] = false

	assert.Equal(false, getEffectiveValue(testResource))
}
