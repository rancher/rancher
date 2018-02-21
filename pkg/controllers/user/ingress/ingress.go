package ingress

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"strconv"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/util"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//TODO fix the workload services cleanup

type Controller struct {
	serviceLister v1.ServiceLister
	services      v1.ServiceInterface
}

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	c := &Controller{
		services:      workload.Core.Services(""),
		serviceLister: workload.Core.Services("").Controller().Lister(),
	}
	workload.Extensions.Ingresses("").AddHandler("ingressWorkloadController", c.sync)
}

func (c *Controller) sync(key string, obj *v1beta1.Ingress) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}
	state := getIngressState(obj)
	if state == nil {
		return nil
	}
	serviceToPort := make(map[string]string)
	serviceToKey := make(map[string]string)
	for _, r := range obj.Spec.Rules {
		host := r.Host
		for _, b := range r.HTTP.Paths {
			path := b.Path
			port := b.Backend.ServicePort.IntVal
			key := getStateKey(host, path, convert.ToString(port))
			if _, ok := state[key]; ok {
				serviceToKey[b.Backend.ServiceName] = key
				serviceToPort[b.Backend.ServiceName] = convert.ToString(port)
			}
		}
	}
	if obj.Spec.Backend != nil {
		serviceName := obj.Spec.Backend.ServiceName
		portStr := convert.ToString(obj.Spec.Backend.ServicePort.IntVal)
		key := getStateKey("", "", portStr)
		if _, ok := state[key]; ok {
			serviceToKey[serviceName] = key
			serviceToPort[serviceName] = portStr
		}
	}

	for serviceName, portStr := range serviceToPort {
		workloadIDs := state[serviceToKey[serviceName]]
		existing, err := c.serviceLister.Get(obj.Namespace, serviceName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if existing == nil {
			controller := true
			ownerRef := metav1.OwnerReference{
				Name:       serviceName,
				APIVersion: "v1beta1/extensions",
				UID:        obj.UID,
				Kind:       "Ingress",
				Controller: &controller,
			}
			port, err := strconv.ParseInt(portStr, 10, 64)
			if err != nil {
				return err
			}
			servicePorts := []corev1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.Parse(portStr),
					Protocol:   "TCP",
				},
			}
			annotations := make(map[string]string)
			annotations[util.WorkloadAnnotation] = workloadIDs
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            serviceName,
					OwnerReferences: []metav1.OwnerReference{ownerRef},
					Namespace:       obj.Namespace,
					Annotations:     annotations,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "None",
					Type:      "ClusterIP",
					Ports:     servicePorts,
				},
			}
			logrus.Infof("Creating headless service %s for ingress %s, port %s", serviceName, key, portStr)
			if _, err := c.services.Create(service); err != nil {
				return err
			}
		} else {
			// TODO - fix so the update is done as needed
			toUpdate := existing.DeepCopy()
			toUpdate.Annotations[util.WorkloadAnnotation] = workloadIDs
			if _, err := c.services.Update(toUpdate); err != nil {
				return err
			}
		}
	}

	return nil
}

func getStateKey(host string, path string, port string) string {
	key := fmt.Sprintf("%s/%s/%s", host, path, port)
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func getIngressState(obj *v1beta1.Ingress) map[string]string {
	annotations := obj.Annotations
	if annotations == nil {
		return nil
	}
	if v, ok := annotations["ingress.cattle.io/state"]; ok {
		state := make(map[string]string)
		json.Unmarshal([]byte(convert.ToString(v)), &state)
		return state
	}
	return nil
}
