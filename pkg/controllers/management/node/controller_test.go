package node

import (
	"fmt"
	"os"
	"strconv"
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

func TestAliasToPath(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	os.Setenv("CATTLE_DEV_MODE", "true")
	defer os.Unsetenv("CATTLE_DEV_MODE")

	for driver, fields := range SchemaToDriverFields {
		testData, _ := createFakeConfig(fields)

		err := aliasToPath(driver, testData, "fake")
		assert.Nil(err)
		for alias := range nodedriver.DriverToSchemaFields[driver] {
			assert.Contains(testData, alias)
		}
		tempdir := os.TempDir()

		for _, v := range testData {
			filePath := v.(string)
			// validate the temp dir is in the path for the field
			assert.Contains(filePath, tempdir)
			// valide the file exists on disk
			_, err = os.Stat(filePath)
			require.Nil(err)

			// assert the file contents starts with our expected string
			b, err := os.ReadFile(filePath)
			require.Nil(err)
			assert.Contains(string(b), "fakecontent")
			os.Remove(filePath)
		}
	}
}

func createFakeConfig(fields map[string]string) (map[string]interface{}, []string) {
	fakeContents := []string{}
	testData := make(map[string]interface{})

	base := "fakecontent"
	i := 0
	for k := range fields {
		content := base + strconv.Itoa(i)
		fakeContents = append(fakeContents, content)
		testData[k] = content
		i++
	}
	return testData, fakeContents

}
