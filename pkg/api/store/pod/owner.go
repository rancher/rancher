package pod

import (
	"fmt"
	"time"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/cache"
)

var (
	replicaSetOwnerCache = cache.NewLRUExpireCache(1000)
)

type key struct {
	SubContext     string
	Namespace      string
	ReplicaSetName string
}

type value struct {
	Kind string
	Name string
}

func getReplicaSetOwner(apiContext *types.APIContext, namespace, name string) (string, string, error) {
	subContext := apiContext.SubContext["/v3/schemas/project"]
	if subContext == "" {
		subContext = apiContext.SubContext["/v3/schemas/cluster"]
	}
	if subContext == "" {
		logrus.Warnf("failed to find subcontext to lookup replicaSet owner")
		return "", "", nil
	}

	key := key{
		SubContext:     subContext,
		Namespace:      namespace,
		ReplicaSetName: name,
	}

	val, ok := replicaSetOwnerCache.Get(key)
	if ok {
		value, _ := val.(value)
		return value.Kind, value.Name, nil
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, &schema.Version, workload.ReplicaSetType, ref.FromStrings(namespace, name), &data); err != nil {
		return "", "", err
	}

	kind, name := getOwner(data)

	replicaSetOwnerCache.Add(key, value{
		Kind: kind,
		Name: name,
	}, time.Hour)

	return kind, name, nil
}

func getOwner(data map[string]interface{}) (string, string) {
	ownerReferences, ok := values.GetSlice(data, "ownerReferences")
	if !ok {
		return "", ""
	}

	for _, ownerReference := range ownerReferences {
		controller, _ := ownerReference["controller"].(bool)
		if !controller {
			continue
		}

		kind, _ := ownerReference["kind"].(string)
		name, _ := ownerReference["name"].(string)
		return kind, name
	}

	return "", ""
}

func resolveWorkloadID(apiContext *types.APIContext, data map[string]interface{}) string {
	kind, name := getOwner(data)
	if kind == "" {
		return ""
	}

	namespace, _ := data["namespaceId"].(string)

	if kind == "ReplicaSet" {
		k, n, err := getReplicaSetOwner(apiContext, namespace, name)
		if err != nil {
			return ""
		}
		if k != "" {
			kind, name = k, n
		}
	}

	return strings.ToLower(fmt.Sprintf("%s:%s:%s", kind, namespace, name))
}
