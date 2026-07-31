package builder

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
)

func TestEmptyStringWithDefault(t *testing.T) {
	schema := &types.Schema{
		ResourceFields: map[string]types.Field{
			"foo": {
				Default: "foo",
				Type:    "string",
				Create:  true,
			},
		},
	}
	schemas := types.NewSchemas()
	schemas.AddSchema(*schema)

	builder := NewBuilder(&types.APIContext{})

	// Test if no field we set to "foo"
	result, err := builder.Construct(schema, nil, Create)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result["foo"]
	assert.True(t, ok)
	assert.Equal(t, "foo", value)

	// Test if field is "" we set to "foo"
	result, err = builder.Construct(schema, map[string]interface{}{
		"foo": "",
	}, Create)
	if err != nil {
		t.Fatal(err)
	}
	value, ok = result["foo"]
	assert.True(t, ok)
	assert.Equal(t, "foo", value)
}
