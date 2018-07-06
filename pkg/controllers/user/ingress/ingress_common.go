package ingress

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/norman/types/convert"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ingressStateAnnotation = "field.cattle.io/ingressState"
)

func GetStateKey(name, namespace, host string, path string, port string) string {
	ipDomain := settings.IngressIPDomain.Get()
	if ipDomain != "" && strings.HasSuffix(host, ipDomain) {
		host = ipDomain
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s", name, namespace, host, path, port)
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func GetIngressState(obj *v1beta1.Ingress) map[string]string {
	annotations := obj.Annotations
	if annotations == nil {
		return nil
	}
	if v, ok := annotations[ingressStateAnnotation]; ok {
		state := make(map[string]string)
		json.Unmarshal([]byte(convert.ToString(v)), &state)
		return state
	}
	return nil
}

type ingressService struct {
	serviceName string
	servicePort int32
	workloadIDs string
}

func generateIngressService(name string, port int32, workloadIDs string) (ingressService, error) {
	rtn := ingressService{
		serviceName: name,
		servicePort: port,
	}
	if workloadIDs != "" {
		b, err := json.Marshal(strings.Split(workloadIDs, "/"))
		if err != nil {
			logrus.WithError(err).Warnf("marshal workload ids %s string error", workloadIDs)
			return rtn, err
		}
		rtn.workloadIDs = string(b)
	}
	return rtn, nil
}

func (i *ingressService) generateNewService(obj *v1beta1.Ingress, serviceType string) *corev1.Service {
	controller := true
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.serviceName,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					Name:       obj.Name,
					APIVersion: "v1beta1/extensions",
					UID:        obj.UID,
					Kind:       "Ingress",
					Controller: &controller,
				},
			},
			Namespace: obj.Namespace,
			Annotations: map[string]string{
				util.WorkloadAnnotation: i.workloadIDs,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []corev1.ServicePort{
				{
					Port:       i.servicePort,
					TargetPort: intstr.FromInt(int(i.servicePort)),
					Protocol:   "TCP",
				},
			},
		},
	}
}

func IsServiceOwnedByIngress(ingress *v1beta1.Ingress, service *corev1.Service) bool {
	for i, owners := 0, service.GetOwnerReferences(); owners != nil && i < len(owners); i++ {
		if owners[i].UID == ingress.UID && owners[i].Kind == ingress.Kind {
			return true
		}
	}
	return false
}
