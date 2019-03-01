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
	assert.Len(aliases, len(nodedriver.Aliases), "Alias maps are not equal")
	for driver, fields := range aliases {
		assert.Contains(nodedriver.Aliases, driver)
		nodeAliases := nodedriver.Aliases[driver]
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
	os.Setenv("CATTLE_DEV_MODE", "true")
	defer os.Unsetenv("CATTLE_DEV_MODE")

	for driver, fields := range aliases {
		testData, fakeContents := createFakeConfig(fields)

		pathed := aliasToPath(driver, testData, "fake")
		for alias := range nodedriver.Aliases[driver] {
			assert.Contains(testData, alias)
		}

		tempdir := os.TempDir()

		for k, v := range pathed {
			assert.Contains(k, tempdir)
			assert.Contains(fakeContents, v)
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
