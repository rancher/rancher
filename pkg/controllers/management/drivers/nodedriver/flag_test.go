package nodedriver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeFieldsAndToV3Field(t *testing.T) {
	jsonPayload := []byte(`[
		{
			"Type": "string",
			"Name": "amazonec2-region",
			"Description": "AWS Region",
			"Default": "us-east-1"
		},
		{
			"Type": "integer",
			"Name": "amazonec2-ssh-port",
			"Description": "SSH Port",
			"Default": 22
		},
		{
			"Type": "boolean",
			"Name": "amazonec2-monitoring",
			"Description": "Enable monitoring",
			"Default": false
		},
		{
			"Type": "stringSlice",
			"Name": "amazonec2-open-port",
			"Description": "Open ports",
			"Default": ["80", "443"]
		}
	]`)

	flags, err := decodeFields(jsonPayload)
	require.NoError(t, err)
	require.Len(t, flags, 4)

	// String flag
	name0, err := fieldNameFromFlag(flags[0])
	require.NoError(t, err)
	assert.Equal(t, "region", name0)
	field0 := flags[0].toV3Field()
	assert.Equal(t, "string", field0.Type)
	assert.Equal(t, "us-east-1", field0.Default.StringValue)
	assert.Equal(t, "AWS Region", field0.Description)

	// Integer flag
	name1, err := fieldNameFromFlag(flags[1])
	require.NoError(t, err)
	assert.Equal(t, "sshPort", name1)
	field1 := flags[1].toV3Field()
	assert.Equal(t, "string", field1.Type)
	assert.Equal(t, "22", field1.Default.StringValue)

	// Boolean flag
	name2, err := fieldNameFromFlag(flags[2])
	require.NoError(t, err)
	assert.Equal(t, "monitoring", name2)
	field2 := flags[2].toV3Field()
	assert.Equal(t, "boolean", field2.Type)

	// StringSlice flag
	name3, err := fieldNameFromFlag(flags[3])
	require.NoError(t, err)
	assert.Equal(t, "openPort", name3)
	field3 := flags[3].toV3Field()
	assert.Equal(t, "array[string]", field3.Type)
	assert.True(t, field3.Nullable)
	assert.Equal(t, []string{"80", "443"}, field3.Default.StringSliceValue)
}
