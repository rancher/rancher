package pod

import (
	"fmt"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/sirupsen/logrus"
)

var (
	ownerCache, _ = lru.New(100000)
)

type key struct {
	SubContext string
	Namespace  string
	Kind       string
	Name       string
}

type value struct {
	Kind string
	Name string
}

func getOwnerWithKind(apiContext *types.APIContext, namespace, ownerKind, name string) (string, string, error) {
	subContext := apiContext.SubContext["/v3/schemas/project"]
	if subContext == "" {
		subContext = apiContext.SubContext["/v3/schemas/cluster"]
	}
	if subContext == "" {
		logrus.Warnf("failed to find subcontext to lookup replicaSet owner")
		return "", "", nil
	}

	key := key{
		SubContext: subContext,
		Namespace:  namespace,
		Kind:       strings.ToLower(ownerKind),
		Name:       name,
	}

	val, ok := ownerCache.Get(key)
	if ok {
		value, _ := val.(value)
		return value.Kind, value.Name, nil
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, &schema.Version, ownerKind, ref.FromStrings(namespace, name), &data); err != nil {
		return "", "", err
	}

	kind, name := getOwner(data)

	ownerCache.Add(key, value{
		Kind: kind,
		Name: name,
	})

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

func SaveOwner(apiContext *types.APIContext, kind, name string, data map[string]interface{}) {
	parentKind, parentName := getOwner(data)
	namespace, _ := data["namespaceId"].(string)

	subContext := apiContext.SubContext["/v3/schemas/project"]
	if subContext == "" {
		subContext = apiContext.SubContext["/v3/schemas/cluster"]
	}
	if subContext == "" {
		return
	}

	key := key{
		SubContext: subContext,
		Namespace:  namespace,
		Kind:       strings.ToLower(kind),
		Name:       name,
	}

	ownerCache.Add(key, value{
		Kind: parentKind,
		Name: parentName,
	})
}

func resolveWorkloadID(apiContext *types.APIContext, data map[string]interface{}) string {
	kind, name := getOwner(data)
	if kind == "" || !workload.WorkloadKinds[kind] {
		return ""
	}

	namespace, _ := data["namespaceId"].(string)

	if ownerKind := strings.ToLower(kind); ownerKind == workload.ReplicaSetType || ownerKind == workload.JobType {
		k, n, err := getOwnerWithKind(apiContext, namespace, ownerKind, name)
		if err != nil {
			return ""
		}
		if k != "" {
			kind, name = k, n
		}
	}

	return strings.ToLower(fmt.Sprintf("%s:%s:%s", kind, namespace, name))
}
