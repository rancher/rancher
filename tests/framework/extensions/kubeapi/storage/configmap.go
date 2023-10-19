package storage

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfigmapTemplate is a constructor that creates a configmap template
func NewConfigmapTemplate(configmapName string, namespace string, annotations map[string]string, labels map[string]string, data map[string]string) corev1.ConfigMap {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if data == nil {
		data = make(map[string]string)
	}

	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: 	 	 configmapName,
			Namespace: 	 namespace,
			Annotations: annotations,
			Labels: 	 labels,
		},
		Data: data,
	}
}