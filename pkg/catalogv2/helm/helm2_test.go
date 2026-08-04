package helm

import (
	"os"
	"testing"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func getHelmV2SampleConfigMap(t *testing.T) runtime.Object {
	t.Helper()
	f, err := os.Open("testdata/helmv2-release-cm.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var m map[string]interface{}
	if err := yaml.NewDecoder(f).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{m}
}

func Test_fromHelm2Data(t *testing.T) {
	res, err := ToRelease(getHelmV2SampleConfigMap(t), func(_ schema.GroupVersionKind) bool {
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, res.HelmMajorVersion)
	assert.Equal(t, "mariadb", res.Name)
	assert.Equal(t, "10.3.22", res.Chart.Metadata.AppVersion)
	assert.Equal(t, catalogv1.StatusDeployed, res.Info.Status)
}
