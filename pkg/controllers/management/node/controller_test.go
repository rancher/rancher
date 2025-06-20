package node

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasMaps(t *testing.T) {
	assert := assert.New(t)
	assert.Len(SchemaToDriverFields, len(nodedriver.DriverToSchemaFields), "Alias maps are not equal")
	for driver, fields := range SchemaToDriverFields {
		assert.Contains(nodedriver.DriverToSchemaFields, driver)
		nodeAliases := nodedriver.DriverToSchemaFields[driver]
		for k, v := range fields {
			// check that the value from the first map is the key to the 2nd map
			val, ok := nodeAliases[v]
			require.True(t, ok, fmt.Sprintf("Alias %v not found", v))
			// check that the value from the 2nd map is equal to the key from the first
			assert.Equal(k, val)
		}
	}
}
