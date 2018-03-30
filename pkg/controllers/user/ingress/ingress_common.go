package ingress

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/rancher/norman/types/convert"
	"k8s.io/api/extensions/v1beta1"
)

func GetStateKey(host string, path string, port string) string {
	key := fmt.Sprintf("%s/%s/%s", host, path, port)
	return base64.URLEncoding.EncodeToString([]byte(key))
}

func GetIngressState(obj *v1beta1.Ingress) map[string]string {
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
