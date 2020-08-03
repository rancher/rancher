package servicemonitor

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	util "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

func (c *MetricsServiceController) createService(key string, w *util.Workload) error {
	for _, o := range w.OwnerReferences {
		if o.Controller != nil && *o.Controller {
			return nil
		}
	}

	if errs := validation.IsDNS1123Subdomain(w.Name); len(errs) != 0 {
		logrus.Debugf("Not creating service for workload [%s]: dns name is invalid", w.Name)
		return nil
	}

	return c.ReconcileServiceMonitor(w)
}

//ReconcileServiceMonitor Workloads to ServiceMonitor
func (c *MetricsServiceController) ReconcileServiceMonitor(w *util.Workload) error {
	expectedServiceMonitor, err := getServiceMonitorFromWorkload(w)
	if err != nil {
		return err
	}

	sm, err := c.getServiceMonitor(w)
	if err != nil {
		return err
	}

	switch {
	case sm != nil && expectedServiceMonitor != nil: //Update scenario
		newSM := sm.DeepCopy()
		if areServiceMonitorEqual(newSM, expectedServiceMonitor) {
			return nil
		}
		newSM.Spec.Endpoints = expectedServiceMonitor.Spec.Endpoints
		value, ok := expectedServiceMonitor.Annotations[util.WorkloadAnnotation]
		if ok {
			newSM.Annotations[util.WorkloadAnnotation] = value
		}
		value, ok = expectedServiceMonitor.Annotations[servicesAnnotation]
		if ok {
			newSM.Annotations[servicesAnnotation] = value
		}
		if _, err := c.smClient.Update(newSM); err != nil {
			return err
		}
	case expectedServiceMonitor != nil: //Create scenario
		if _, err := c.smClient.Create(expectedServiceMonitor); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case sm != nil: //Delete scenario
		return c.smClient.DeleteNamespaced(sm.Namespace, sm.Name, &metav1.DeleteOptions{})
	}
	return nil
}

func (c *MetricsServiceController) syncServiceMonitor(key string, svm *monitoringv1.ServiceMonitor) (runtime.Object, error) {
	if svm == nil || svm.DeletionTimestamp != nil {
		return nil, nil
	}

	data := md5.Sum([]byte(key))
	base64Key := hex.EncodeToString(data[:])
	var servicePorts []corev1.ServicePort
	serviceAnnotations := map[string]string{}
	serviceLabels := map[string]string{
		metricsServiceLabel: base64Key,
	}
	controller := true
	owner := metav1.OwnerReference{
		APIVersion: svm.APIVersion,
		Kind:       svm.Kind,
		Name:       svm.Name,
		UID:        svm.UID,
		Controller: &controller,
	}

	wExistings, sExistings, err := c.getMetricsServices(svm)
	if err != nil {
		return nil, err
	}

	//handling workloads
	var toDelete []*corev1.Service
	toCreate := map[string]*corev1.Service{}
	toUpdate := map[string]*corev1.Service{}

	workloadIDs := getStringSliceFromAnnotation(svm.ObjectMeta, util.WorkloadAnnotation)
	if len(workloadIDs) != 0 {
		servicePorts = GetServicePortsFromEndpoint(svm.Spec.Endpoints)
	}
	for _, workloadID := range workloadIDs {
		w, err := c.workloadLister.GetByWorkloadID(workloadID)
		if err != nil {
			logrus.WithError(err).Warnf("workload %s is not existing anymore", workloadID)
			continue
		}
		workloadTarget, err := util.IDAnnotationToString(workloadID)
		if err != nil {
			return svm, err
		}
		svc, ok := wExistings[workloadID]
		if !ok {
			toCreate[workloadID] = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            w.Name + metricsServiceSuffix,
					Namespace:       svm.Namespace,
					Annotations:     serviceAnnotations,
					Labels:          serviceLabels,
					OwnerReferences: []metav1.OwnerReference{owner},
				},
				Spec: corev1.ServiceSpec{
					Ports:     servicePorts,
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: corev1.ClusterIPNone,
					Selector:  w.SelectorLabels,
				},
			}
			toCreate[workloadID].Annotations[util.WorkloadAnnotation] = workloadTarget
		} else {
			if !util.ArePortsEqual(svc.Spec.Ports, servicePorts) {
				newSvc := svc.DeepCopy()
				newSvc.Spec.Ports = servicePorts
				newSvc.Labels = serviceLabels
				newSvc.Annotations[util.WorkloadAnnotation] = workloadTarget
				toUpdate[workloadID] = newSvc
			}
		}
	}

	for k, v := range wExistings {
		_, ok1 := toCreate[k]
		_, ok2 := toUpdate[k]
		if !ok1 && !ok2 {
			toDelete = append(toDelete, v)
		}
	}

	for _, create := range toCreate {
		if _, err := c.services.Create(create); err != nil && !apierrors.IsAlreadyExists(err) {
			return svm, err
		}
	}

	for _, update := range toUpdate {
		if _, err := c.services.Update(update); err != nil {
			return svm, err
		}
	}

	for _, delete := range toDelete {
		if err := c.services.DeleteNamespaced(delete.Namespace, delete.Name, &metav1.DeleteOptions{}); err != nil {
			return svm, err
		}
	}

	//handling services
	serviceIDs := getStringSliceFromAnnotation(svm.ObjectMeta, servicesAnnotation)

	for _, serviceID := range serviceIDs {
		_, ok := sExistings[serviceID]
		if !ok {
			svc, err := c.serviceLister.Get(svm.Namespace, svm.Name)
			if err != nil {
				return svm, err
			}
			newSvc := svc.DeepCopy()
			newSvc.Labels[metricsServiceLabel] = base64Key
			if _, err := c.services.Update(newSvc); err != nil {
				return svm, err
			}
		}
	}

	for _, existing := range sExistings {
		found := false
		for _, serviceID := range serviceIDs {
			if serviceID == existing.Name {
				found = true
			}
		}
		if !found {
			newSvc := existing.DeepCopy()
			delete(newSvc.Labels, metricsServiceLabel)
			if _, err := c.services.Update(newSvc); err != nil {
				return svm, err
			}
		}
	}

	if (len(workloadIDs) != 0 || len(serviceIDs) != 0) &&
		(svm.Spec.Selector.MatchLabels == nil || svm.Spec.Selector.MatchLabels[metricsServiceLabel] != base64Key) {
		newSvm := svm.DeepCopy()
		if newSvm.Spec.Selector.MatchLabels == nil {
			newSvm.Spec.Selector.MatchLabels = map[string]string{}
		}
		newSvm.Spec.Selector.MatchLabels[metricsServiceLabel] = base64Key
		_, err := c.smClient.Update(newSvm)
		if err != nil {
			return svm, err
		}
		return newSvm, nil
	}

	return svm, nil
}

func (c *MetricsServiceController) getMetricsServices(svm *monitoringv1.ServiceMonitor) (map[string]*corev1.Service, map[string]*corev1.Service, error) {
	data := md5.Sum([]byte(fmt.Sprintf("%s/%s", svm.Namespace, svm.Name)))
	hashKey := hex.EncodeToString(data[:])
	svcs, err := c.serviceLister.List(
		svm.Namespace,
		labels.SelectorFromSet(map[string]string{metricsServiceLabel: hashKey}),
	)
	if err != nil {
		return nil, nil, err
	}
	workloads := map[string]*corev1.Service{}
	services := map[string]*corev1.Service{}
	for _, svc := range svcs {
		workloadIDs := getStringSliceFromAnnotation(svc.ObjectMeta, util.WorkloadAnnotation)
		switch len(workloadIDs) {
		case 0:
			services[svc.Name] = svc
		default:
			for _, owner := range svc.OwnerReferences {
				if owner.UID == svm.UID {
					workloads[workloadIDs[0]] = svc
					break
				}
			}
		}
	}
	return workloads, services, nil
}

func (c *MetricsServiceController) getServiceMonitor(w *util.Workload) (*monitoringv1.ServiceMonitor, error) {
	sms, err := c.smLister.List(w.Namespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for _, sm := range sms {
		for _, owner := range sm.OwnerReferences {
			if owner.UID == w.UUID {
				return sm, nil
			}
		}
	}
	return nil, nil
}
