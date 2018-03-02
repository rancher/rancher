package ingress

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/sirupsen/logrus"
)

func Wrap(store types.Store) types.Store {
	modify := &Store{
		store,
	}
	return New(modify)
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	formatData(data, false)
	data, err := p.Store.Create(apiContext, schema, data)
	return data, err
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	formatData(data, false)
	data, err := p.Store.Update(apiContext, schema, data, id)
	return data, err
}

func formatData(data map[string]interface{}, forFrontend bool) {
	oldState := getState(data)
	newState := map[string]string{}

	// transform default backend
	if target, ok := values.GetValue(data, "defaultBackend"); ok {
		updateRule(convert.ToMapInterface(target), "/", forFrontend, data, oldState, newState)
	}

	// transform rules
	if paths, ok := getPaths(data); ok {
		for hostpath, target := range paths {
			updateRule(target, hostpath, forFrontend, data, oldState, newState)
		}
	}

	updateCerts(data, forFrontend, oldState, newState)
	setState(data, newState)

}

func updateRule(target map[string]interface{}, hostpath string, forFrontend bool, data map[string]interface{}, oldState map[string]string, newState map[string]string) {
	targetData := convert.ToMapInterface(target)
	port, _ := targetData["targetPort"]
	serviceID, _ := targetData["serviceId"].(string)
	stateKey := getStateKey(hostpath, convert.ToString(port))
	if forFrontend {
		isService := true
		if serviceValue, ok := oldState[stateKey]; ok && !convert.IsEmpty(serviceValue) {
			targetData["workloadIds"] = strings.Split(serviceValue, "/")
			isService = false
		}

		if isService {
			targetData["serviceId"] = fmt.Sprintf("%s/%s", data["namespaceId"].(string), serviceID)
		} else {
			delete(targetData, "serviceId")
		}
	} else {
		workloadIDs := convert.ToStringSlice(targetData["workloadIds"])
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

func getStateKey(hostpath string, port string) string {
	key := fmt.Sprintf("%s/%s", hostpath, port)
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func getCertKey(key string) string {
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func getPaths(data map[string]interface{}) (map[string]map[string]interface{}, bool) {
	v, ok := values.GetValue(data, "rules")
	if !ok {
		return nil, false
	}

	result := make(map[string]map[string]interface{})
	for _, rule := range convert.ToMapSlice(v) {
		converted := convert.ToMapInterface(rule)
		paths, ok := converted["paths"]
		if ok {
			for path, target := range convert.ToMapInterface(paths) {
				result[fmt.Sprintf("%s/%s", convert.ToString(converted["host"]), path)] = convert.ToMapInterface(target)
			}
		}
	}

	return result, true
}

func setState(data map[string]interface{}, stateMap map[string]string) {
	content, err := json.Marshal(stateMap)
	if err != nil {
		logrus.Errorf("failed to save state on ingress: %v", data["id"])
		return
	}

	values.PutValue(data, string(content), "annotations", "ingress.cattle.io/state")
}

func getState(data map[string]interface{}) map[string]string {
	state := make(map[string]string)

	v, ok := values.GetValue(data, "annotations", "ingress.cattle.io/state")
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
				cert["certificateId"] = oldState[certKey]
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

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			formatData(data, true)
			return data, nil
		},
	}
}
