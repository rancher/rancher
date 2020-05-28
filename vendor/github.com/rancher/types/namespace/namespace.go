package namespace

import v1 "k8s.io/api/core/v1"

var projectIDAnnotation = "field.cattle.io/projectId"

func NsByProjectID(obj interface{}) ([]string, error) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return []string{}, nil
	}

	if id, ok := ns.Annotations[projectIDAnnotation]; ok {
		return []string{id}, nil
	}

	return []string{}, nil
}
