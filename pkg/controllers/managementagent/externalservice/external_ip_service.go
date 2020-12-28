package externalservice

import (
	"context"

	"reflect"

	"encoding/json"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// this controller monitors services with "field.cattle.io/ipAddresses" annotation
// and creates an endpoint for it

const (
	ExternalIPsAnnotation = "field.cattle.io/ipAddresses"
)

type Controller struct {
	endpointsLister v1.EndpointsLister
	endpoints       v1.EndpointsInterface
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		endpoints:       workload.Core.Endpoints(""),
		endpointsLister: workload.Core.Endpoints("").Controller().Lister(),
	}
	workload.Core.Services("").AddHandler(ctx, "externalIpServiceController", c.sync)
}

func (c *Controller) sync(key string, obj *corev1.Service) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if obj.Annotations == nil {
		return nil, nil
	}

	var newSubsets []corev1.EndpointSubset
	if val, ok := obj.Annotations[ExternalIPsAnnotation]; !ok || val == "" {
		return nil, nil
	}
	var ipsStr []string
	err := json.Unmarshal([]byte(obj.Annotations[ExternalIPsAnnotation]), &ipsStr)
	if err != nil {
		logrus.Debugf("Failed to unmarshal ipAddresses, error: %v", err)
		return nil, nil
	}
	if ipsStr == nil {
		return nil, nil
	}

	var addresses []corev1.EndpointAddress
	for _, ipStr := range ipsStr {
		addresses = append(addresses, corev1.EndpointAddress{IP: ipStr})
	}
	var ports []corev1.EndpointPort
	if len(addresses) > 0 {
		for _, p := range obj.Spec.Ports {
			epPort := corev1.EndpointPort{Name: p.Name, Protocol: p.Protocol, Port: p.TargetPort.IntVal}
			ports = append(ports, epPort)
		}
		if len(ports) == 0 {
			epPort := corev1.EndpointPort{Name: "default", Protocol: corev1.ProtocolTCP, Port: 42}
			ports = append(ports, epPort)
		}
		newSubsets = append(newSubsets, corev1.EndpointSubset{
			Addresses: addresses,
			Ports:     ports,
		})
	}

	existing, err := c.endpointsLister.Get(obj.Namespace, obj.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if existing == nil {
		controller := true
		ownerRef := metav1.OwnerReference{
			Name:       obj.Name,
			APIVersion: "v1",
			UID:        obj.UID,
			Kind:       "Service",
			Controller: &controller,
		}

		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:            obj.Name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Namespace:       obj.Namespace,
			},
			Subsets: newSubsets,
		}
		logrus.Infof("Creating endpoints for external ip service [%s]: %v", key, ep.Subsets)
		if _, err := c.endpoints.Create(ep); err != nil {
			return nil, err
		}
	} else if subsetsChanged(newSubsets, existing.Subsets) {
		toUpdate := existing.DeepCopy()
		toUpdate.Subsets = newSubsets
		logrus.Infof("Updating endpoints for external ip service [%s]: %v", key, toUpdate.Subsets)
		if _, err := c.endpoints.Update(toUpdate); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func subsetsChanged(new []corev1.EndpointSubset, old []corev1.EndpointSubset) bool {
	if len(new) != len(old) {
		return true
	}
	for _, newEP := range new {
		found := false
		for _, oldEP := range old {
			if reflect.DeepEqual(newEP, oldEP) {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}
