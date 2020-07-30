package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	fooProviderInstance = `
apiVersion: project.cattle.io/v3
kind: SourceCodeProviderConfig
metadata:
  creationTimestamp: "2000-01-01T00:00:00Z"
  generation: 1
  labels:
    foo: bar
  managedFields:
  - apiVersion: project.cattle.io/v3
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:labels:
          .: {}
          f:cattle.io/creator: {}
      f:projectName: {}
      f:type: {}
    manager: Go-http-client
    operation: Update
    time: "2000-01-01T00:00:00Z"
  name: foo
  namespace: p-test
  resourceVersion: "1234"
  selfLink: /apis/project.cattle.io/v3/namespaces/p-test/sourcecodeproviderconfigs/foo
  uid: 693145a1-22e4-471d-8410-d7555458702f
projectName: local:p-abcde
type: fooConfig`
)

func TestObjectMetaFromUnstructureContent(t *testing.T) {
	foo := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(fooProviderInstance), &foo.Object)
	assert.Nil(t, err)
	objectMeta, err := ObjectMetaFromUnstructureContent(foo.UnstructuredContent())
	assert.Nil(t, err)
	assert.Equal(t, "foo", objectMeta.Name)
	assert.Equal(t, "p-test", objectMeta.Namespace)
	assert.Equal(t, map[string]string{"foo": "bar"}, objectMeta.Labels)
}
