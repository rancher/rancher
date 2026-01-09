package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedValues(t *testing.T) {
	v := orderedValues{}
	v.Add("key1", "value1")
	v.Add("key2", "value2")
	v.Add("key3", "value3")

	assert.Equal(t, "key1=value1&key2=value2&key3=value3", v.Encode())
}
