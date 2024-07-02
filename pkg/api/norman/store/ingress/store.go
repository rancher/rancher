package ingress

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/norman/store/workload"
	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementagent/ingress"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

const (
	ingressStateAnnotation = "field.cattle.io/ingressState"
)

func Wrap(manager *clustermanager.Manager, userContext *config.ScaledContext) func(types.Store) types.Store {
	return func(store types.Store) types.Store {
		return New(store, manager, userContext)
	}
}

type Store struct {
	types.Store
	legacyStore    types.Store
	userContext    *config.ScaledContext
	ClusterManager *clustermanager.Manager
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	name, _ := data["name"].(string)
	namespace, _ := data["namespaceId"].(string)
	id := ref.FromStrings(namespace, name)
	formatData(id, data, false)
	clusterContext, err := p.downstreamClusterContext(apiContext)
	if err != nil {
		return nil, err
	}
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		return p.Store.Create(apiContext, schema, data)
	}
	// TODO: remove when support for k8s 1.18 is dropped
	return p.legacyStore.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	formatData(id, data, false)
	clusterContext, err := p.downstreamClusterContext(apiContext)
	if err != nil {
		return nil, err
	}
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		return p.Store.Update(apiContext, schema, data, id)
	}
	// TODO: remove when support for k8s 1.18 is dropped
	return p.legacyStore.Update(apiContext, schema, data, id)
}

func (p *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	clusterContext, err := p.downstreamClusterContext(apiContext)
	if err != nil {
		return nil, err
	}
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		return p.Store.List(apiContext, schema, opt)
	}
	// TODO: remove when support for k8s 1.18 is dropped
	return p.legacyStore.List(apiContext, schema, opt)
}

func (p *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	clusterContext, err := p.downstreamClusterContext(apiContext)
	if err != nil {
		return nil, err
	}
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		return p.Store.Watch(apiContext, schema, opt)
	}
	// TODO: remove when support for k8s 1.18 is dropped
	return p.legacyStore.Watch(apiContext, schema, opt)
}

func (p *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	clusterContext, err := p.downstreamClusterContext(apiContext)
	if err != nil {
		return nil, err
	}
	if ingresswrapper.ServerSupportsIngressV1(clusterContext.K8sClient) {
		return p.Store.ByID(apiContext, schema, id)
	}
	// TODO: remove when support for k8s 1.18 is dropped
	return p.legacyStore.ByID(apiContext, schema, id)
}

// downstreamClusterContext is a helper for fetching the cluster context from the request.
// TODO: remove when support for k8s 1.18 is dropped
func (p *Store) downstreamClusterContext(apiContext *types.APIContext) (*config.UserContext, error) {
	clusterName := p.ClusterManager.ClusterName(apiContext)
	return p.ClusterManager.UserContext(clusterName)
}

func formatData(id string, data map[string]interface{}, forFrontend bool) {
	oldState := getState(data)
	newState := map[string]string{}

	// transform default backend
	if target, ok := values.GetValue(data, "defaultBackend"); ok && target != nil {
		updateRule(convert.ToMapInterface(target), id, "", "/", forFrontend, data, oldState, newState)
	}

	// transform rules
	if paths, ok, flag := getPaths(data); ok {
		for hostpath, target := range paths {
			updateRule(target, id, hostpath.host, hostpath.path, forFrontend, data, oldState, newState)
		}
		if flag {
			updateDataRules(data)
		}
	}

	updateCerts(data, forFrontend, oldState, newState)
	setState(data, newState)
	workload.SetPublicEndpointsFields(data)
}

func updateDataRules(data map[string]interface{}) {
	v, ok := values.GetValue(data, "rules")
	if !ok {
		return
	}
	var updated []interface{}
	rules := convert.ToInterfaceSlice(v)
	for _, rule := range rules {
		converted := convert.ToMapInterface(rule)
		paths, ok := converted["paths"]
		if ok {
			pathSlice := convert.ToInterfaceSlice(paths)
			for index, target := range pathSlice {
				targetMap := convert.ToMapInterface(target)
				serviceID := convert.ToString(values.GetValueN(targetMap, "serviceId"))
				if serviceID == "" {
					pathSlice = append(pathSlice[:index], pathSlice[index+1:]...)
				}
			}
			converted["paths"] = pathSlice
			if len(pathSlice) != 0 {
				updated = append(updated, rule)
			}
		}
	}
	values.PutValue(data, updated, "rules")
}

func updateRule(target map[string]interface{}, id, host, path string, forFrontend bool, data map[string]interface{}, oldState map[string]string, newState map[string]string) {
	targetData := convert.ToMapInterface(target)
	port, _ := targetData["targetPort"]
	serviceID, _ := targetData["serviceId"].(string)
	namespace, name := ref.Parse(id)
	stateKey := ingress.GetStateKey(name, namespace, host, path, convert.ToString(port))
	if forFrontend {
		isService := true
		if serviceValue, ok := oldState[stateKey]; ok && !convert.IsAPIObjectEmpty(serviceValue) {
			targetData["workloadIds"] = strings.Split(serviceValue, "/")
			isService = false
		}

		if isService {
			targetData["serviceId"] = fmt.Sprintf("%s:%s", data["namespaceId"].(string), serviceID)
		} else {
			delete(targetData, "serviceId")
		}
	} else {
		workloadIDs := convert.ToStringSlice(targetData["workloadIds"])
		sort.Strings(workloadIDs)
		if serviceID != "" {
			splitted := strings.Split(serviceID, ":")
			if len(splitted) > 1 {
				serviceID = splitted[1]
			}
		} else {
			serviceID = getServiceID(stateKey)
		}
		newState[stateKey] = strings.Join(workloadIDs, "/")
		targetData["serviceId"] = serviceID
		values.RemoveValue(targetData, "workloadIds")
	}
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

func getCertKey(key string) string {
	return base64.URLEncoding.EncodeToString([]byte(key))
}

type hostPath struct {
	host string
	path string
}

func getPaths(data map[string]interface{}) (map[hostPath]map[string]interface{}, bool, bool) {
	v, ok := values.GetValue(data, "rules")
	if !ok {
		return nil, false, false
	}
	flag := false
	result := make(map[hostPath]map[string]interface{})
	for _, rule := range convert.ToMapSlice(v) {
		converted := convert.ToMapInterface(rule)
		paths, ok := converted["paths"]
		if ok {
			for _, target := range convert.ToMapSlice(paths) {
				targetMap := convert.ToMapInterface(target)
				path := convert.ToString(targetMap["path"])
				key := hostPath{host: convert.ToString(converted["host"]), path: path}
				if existing, ok := result[key]; ok {
					flag = true
					targetWorkloadIDs := convert.ToStringSlice(values.GetValueN(targetMap, "workloadIds"))
					updated := convert.ToStringSlice(values.GetValueN(convert.ToMapInterface(existing), "workloadIds"))
					targetWorkloadIDs = append(targetWorkloadIDs, updated...)
					values.PutValue(targetMap, targetWorkloadIDs, "workloadIds")
				}
				result[key] = targetMap
			}
		}
	}

	return result, true, flag
}

func setState(data map[string]interface{}, stateMap map[string]string) {
	content, err := json.Marshal(stateMap)
	if err != nil {
		logrus.Errorf("failed to save state on ingress: %v", data["id"])
		return
	}

	values.PutValue(data, string(content), "annotations", ingressStateAnnotation)
}

func getState(data map[string]interface{}) map[string]string {
	state := map[string]string{}

	v, ok := values.GetValue(data, "annotations", ingressStateAnnotation)
	if ok {
		json.Unmarshal([]byte(convert.ToString(v)), &state)
	}

	return state
}

func updateCerts(data map[string]interface{}, forFrontend bool, oldState map[string]string, newState map[string]string) {
	if forFrontend {
		if certs, _ := values.GetSlice(data, "tls"); len(certs) > 0 {
			for _, cert := range certs {
				certName := convert.ToString(cert["certificateId"])
				certKey := getCertKey(certName)
				id := oldState[certKey]
				if id == "" {
					cert["certificateId"] = fmt.Sprintf("%s:%s", convert.ToString(data["namespaceId"]), certName)
				} else {
					cert["certificateId"] = id
				}
			}
		}
	} else {
		if certs, _ := values.GetSlice(data, "tls"); len(certs) > 0 {
			for _, cert := range certs {
				certificateID := convert.ToString(cert["certificateId"])
				id := strings.Split(certificateID, ":")
				if len(id) == 2 {
					certName := id[1]
					certKey := getCertKey(certName)
					newState[certKey] = certificateID
					cert["certificateId"] = certName
				}
			}
		}
	}
}

func New(store types.Store, manager *clustermanager.Manager, userContext *config.ScaledContext) types.Store {
	// legacyStore is a norman proxy store object for the
	// extensions/v1beta1/Ingress API that can be used to sidestep the registered
	// networking.k8s.io/v1/Ingress API if the server doesn't support it.
	// TODO: remove when support for k8s 1.18 is dropped
	legacyStore := proxy.NewProxyStore(
		context.Background(),
		userContext.ClientGetter,
		config.UserStorageContext,
		scheme.Scheme,
		[]string{"apis"},
		"extensions",
		"v1beta1",
		"Ingress",
		"ingresses",
	)
	s := &Store{
		store,
		legacyStore,
		userContext,
		manager,
	}
	return &transform.Store{
		Store: s,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			id, _ := data["id"].(string)
			formatData(id, data, true)
			setIngressState(data)
			return data, nil
		},
	}
}

func setIngressState(data map[string]interface{}) {
	lbStatus, ok := values.GetSlice(data, "status", "loadBalancer", "ingress")
	if !ok || len(lbStatus) == 0 {
		data["state"] = "initializing"
	}
}
