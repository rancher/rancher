package ingress

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/workload/converttypes"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	serviceSchema = schema.Schemas.Schema(&schema.Version, client.ServiceType)
)

type Controller struct {
	serviceClient v1.ServiceInterface
	namespaces    v1.NamespaceLister
	services      v1.ServiceLister
}

func NewIngressWorkloadController(workload *config.WorkloadContext) *Controller {
	return &Controller{
		serviceClient: workload.Core.Services(""),
		namespaces:    workload.Core.Namespaces("").Controller().Lister(),
		services:      workload.Core.Services("").Controller().Lister(),
	}
}

func (c *Controller) Reconcile(data map[string]interface{}, frontend bool) ([]corev1.Service, error) {
	oldState := getState(data)
	paths, ok := getPaths(data)
	if !ok || len(paths) == 0 {
		return nil, nil
	}

	ingressName, _ := data["name"].(string)
	uid, _ := data["uuid"].(string)

	newState := map[string]bool{}
	for _, target := range paths {
		targetData := convert.ToMapInterface(target)
		port, _ := targetData["targetPort"]
		serviceID, _ := targetData["serviceId"].(string)
		workloadIDs := convert.ToStringSlice(targetData["workloadIds"])

		if len(workloadIDs) == 0 || convert.IsEmpty(port) {
			if !frontend {
				for oldServiceKey := range oldState {
					if getServiceID(oldServiceKey) == serviceID {
						newState[oldServiceKey] = true
					}
				}
			}
			continue
		}

		stateKey := getStateKey(convert.ToString(port), workloadIDs)
		newState[stateKey] = true
		targetData["serviceId"] = getServiceID(stateKey)
	}

	setState(data, newState)
	toCreate, toDelete, same := set.Diff(newState, oldState)
	toCreate = append(toCreate, same...)

	var lastErr error

	if frontend {
		for _, deleteServiceKey := range toDelete {
			service, err := toService(data, deleteServiceKey)
			if err != nil {
				lastErr = err
				continue
			}
			if err := c.remove(service); err != nil {
				lastErr = err
			}
		}
	}

	var result []corev1.Service
	for _, serviceKey := range toCreate {
		service, err := toService(data, serviceKey)
		if err != nil {
			lastErr = err
			continue
		}
		result = append(result, service)
		if frontend {
			if err := c.create(ingressName, uid, service); err != nil {
				lastErr = err
			}
		}
	}

	return result, lastErr
}

func (c *Controller) remove(service corev1.Service) error {
	prop := metav1.DeletePropagationForeground
	return c.serviceClient.DeleteNamespaced(service.Namespace, service.Name, &metav1.DeleteOptions{
		PropagationPolicy: &prop,
	})
}

func (c *Controller) create(ingressParent, uid string, service corev1.Service) error {
	_, err := c.services.Get(service.Namespace, service.Name)
	if !errors.IsNotFound(err) {
		return err
	}

	t := true
	service.OwnerReferences = append(service.OwnerReferences, metav1.OwnerReference{
		Name:               ingressParent,
		APIVersion:         "v1beta1/extensions",
		UID:                types.UID(uid),
		Kind:               "Ingress",
		BlockOwnerDeletion: &t,
	})

	_, err = c.serviceClient.Create(&service)
	return err
}

func toService(data map[string]interface{}, serviceKey string) (corev1.Service, error) {
	var result corev1.Service

	name := getServiceID(serviceKey)
	namespace, _ := data["namespaceId"].(string)

	bytes, err := base64.URLEncoding.DecodeString(serviceKey)
	if err != nil {
		return result, err
	}

	parts := strings.Split(string(bytes), "/")
	if len(parts) == 1 {
		return result, fmt.Errorf("invalid service key: %v", serviceKey)
	}

	workloadIDs := parts[:len(parts)-1]
	portString := parts[len(parts)-1]
	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil {
		return result, fmt.Errorf("invalid port number: %v", portString)
	}

	service := client.Service{
		Name:              name,
		NamespaceId:       namespace,
		TargetWorkloadIDs: workloadIDs,
		ClusterIp:         "",
		Kind:              "ClusterIP",
		Ports: []client.ServicePort{
			{
				Port:       &port,
				TargetPort: intstr.Parse(portString),
				Protocol:   "TCP",
			},
		},
	}

	return result, converttypes.ToInternal(service, serviceSchema, &result)
}

func getServiceID(stateKey string) string {
	bytes, err := base64.URLEncoding.DecodeString(stateKey)
	if err != nil {
		return ""
	}

	sum := md5.Sum(bytes)
	hex := "ingress-" + hex.EncodeToString(sum[:])

	return hex
}

func getPaths(data map[string]interface{}) ([]map[string]interface{}, bool) {
	v, ok := values.GetValue(data, "rules")
	if !ok {
		return nil, false
	}

	var result []map[string]interface{}
	for _, rule := range convert.ToMapSlice(v) {
		paths, ok := convert.ToMapInterface(rule)["paths"]
		if ok {
			for _, target := range convert.ToMapInterface(paths) {
				result = append(result, convert.ToMapInterface(target))
			}
		}
	}

	return result, true
}

func getStateKey(port string, workloadIDs []string) string {
	key := fmt.Sprintf("%s/%s", strings.Join(workloadIDs, "/"), port)
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func setState(data map[string]interface{}, stateMap map[string]bool) {
	var state []string

	for key := range stateMap {
		state = append(state, key)
	}

	content, err := json.Marshal(state)
	if err != nil {
		logrus.Errorf("failed to save state on ingress: %v", data["id"])
		return
	}

	values.PutValue(data, string(content), "annotations", "ingress.cattle.io/state")
}

func getState(data map[string]interface{}) map[string]bool {
	var state []string

	v, ok := values.GetValue(data, "annotations", "ingress.cattle.io/state")
	if ok {
		json.Unmarshal([]byte(convert.ToString(v)), &state)
	}

	result := map[string]bool{}
	for _, v := range state {
		result[v] = true
	}

	return result
}
