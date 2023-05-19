package ref

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func FromStrings(namespace, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
}

func Ref(obj runtime.Object) string {
	objMeta, _ := meta.Accessor(obj)
	if objMeta.GetNamespace() == "" {
		return objMeta.GetName()
	}
	return FromStrings(objMeta.GetNamespace(), objMeta.GetName())
}

func Parse(ref string) (namespace string, name string) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
