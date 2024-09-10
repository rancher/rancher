package services

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewServiceTemplate is a constructor that creates the service template for services
func NewServiceTemplate(serviceName, namespaceName string, serviceType corev1.ServiceType, ports []corev1.ServicePort, selector map[string]string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespaceName,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Ports:    ports,
			Selector: selector,
		},
	}
}
