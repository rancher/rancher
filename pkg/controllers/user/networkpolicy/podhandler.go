package networkpolicy

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	// PodNameFieldLabel is used to specify the podName for pods
	// with hostPort specified
	PodNameFieldLabel = "field.cattle.io/podName"
)

type podHandler struct {
	npmgr *netpolMgr
	pods  v1.PodInterface
}

func (ph *podHandler) Sync(key string, pod *corev1.Pod) error {
	if pod == nil || pod.DeletionTimestamp != nil {
		return nil
	}
	logrus.Debugf("podHandler: Sync: %+v", *pod)

	if err := ph.addLabelIfHostPortsPresent(pod); err != nil {
		return err
	}
	return ph.npmgr.hostPortsUpdateHandler(pod)
}

// k8s native network policy can select pods only using labels,
// hence need to add a label which can be used to select this pod
// which has hostPorts
func (ph *podHandler) addLabelIfHostPortsPresent(pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	hasHostPorts := false
Loop:
	for _, c := range pod.Spec.Containers {
		for _, port := range c.Ports {
			if port.HostPort != 0 {
				hasHostPorts = true
				break Loop
			}
		}
	}
	if hasHostPorts {
		logrus.Debugf("podHandler: addLabelIfHostPortsPresent: pod=%+v has HostPort", *pod)
		if _, ok := pod.Labels[PodNameFieldLabel]; !ok {
			podCopy := pod.DeepCopy()
			if podCopy.Labels == nil {
				podCopy.Labels = map[string]string{}
			}
			podCopy.Labels[PodNameFieldLabel] = podCopy.Name
			_, err := ph.pods.Update(podCopy)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
