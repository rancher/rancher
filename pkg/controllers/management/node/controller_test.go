package node

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/data/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasToPath(t *testing.T) {
	assertT := assert.New(t)
	requireT := require.New(t)
	err := os.Setenv("CATTLE_DEV_MODE", "true")
	assertT.NoError(err)
	defer os.Unsetenv("CATTLE_DEV_MODE")

	for _, fields := range management.DriverData {
		testConfig, annotations := createFakeConfigAnnotations(fields.FileToFieldAliases)
		err = aliasToPath(annotations, testConfig, "fakeNamespace")
		assertT.NoError(err)

		driverToSchemaFields := reverseAnnotations(annotations["fileToFieldAliases"])
		for alias := range driverToSchemaFields {
			assertT.Contains(testConfig, alias)
		}

		tempDir := os.TempDir()

		for _, v := range testConfig {
			filePath := v.(string)
			// validate that the temp directory in the path for the field
			assertT.Contains(filePath, tempDir)
			// validate that the file exists on disk
			_, err = os.Stat(filePath)
			requireT.NoError(err)

			// validate that the fileContents start with expected string
			b, err := os.ReadFile(filePath)
			requireT.NoError(err)
			assertT.Contains(string(b), "fakecontent")
			os.Remove(filePath)
		}
	}
}

func createFakeConfigAnnotations(fields map[string]string) (map[string]interface{}, map[string]string) {
	fakeContents := []string{}
	testData := make(map[string]interface{})
	annotations := map[string]string{}

	base := "fakecontent"
	i := 0
	for k, v := range fields {
		content := base + strconv.Itoa(i)
		fakeContents = append(fakeContents, fmt.Sprintf("%s:%s", k, v))
		testData[k] = content
		i++
	}

	if len(fakeContents) > 0 {
		annotations = map[string]string{
			"fileToFieldAliases": strings.Join(fakeContents, ","),
		}
	}

	return testData, annotations
}

// reverseAnnotations reverse the key-value pairing and splits the string of key-value pair into
func reverseAnnotations(annotations string) map[string]string {
	result := map[string]string{}
	pairs := strings.Split(annotations, ",")
	for _, pair := range pairs {
		keyVal := strings.SplitN(pair, ":", 2)
		if len(keyVal) == 2 {
			result[strings.TrimSpace(keyVal[1])] = strings.TrimSpace(keyVal[0])
		}
	}
	return result
}
