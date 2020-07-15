package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/stores/partition"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	passthroughPartitions = []partition.Partition{
		Partition{Passthrough: true},
	}
)

type filterKey struct{}

func AddNamespaceConstraint(req *http.Request, names ...string) *http.Request {
	set := sets.NewString(names...)
	ctx := context.WithValue(req.Context(), filterKey{}, set)
	return req.WithContext(ctx)
}

func getNamespaceConstraint(req *http.Request) (sets.String, bool) {
	set, ok := req.Context().Value(filterKey{}).(sets.String)
	return set, ok
}

type Partition struct {
	Namespace   string
	All         bool
	Passthrough bool
	Names       sets.String
}

func (p Partition) Name() string {
	return p.Namespace
}

type rbacPartitioner struct {
	proxyStore *Store
}

func (p *rbacPartitioner) Lookup(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) (partition.Partition, error) {
	switch verb {
	case "create":
		fallthrough
	case "get":
		fallthrough
	case "update":
		fallthrough
	case "delete":
		return passthroughPartitions[0], nil
	default:
		return nil, fmt.Errorf("partition list: invalid verb %s", verb)
	}
}

func (p *rbacPartitioner) All(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) ([]partition.Partition, error) {
	switch verb {
	case "list":
		fallthrough
	case "watch":
		if id != "" {
			ns, name := kv.RSplit(id, "/")
			return []partition.Partition{
				Partition{
					Namespace:   ns,
					All:         false,
					Passthrough: false,
					Names:       sets.NewString(name),
				},
			}, nil
		}
		partitions, passthrough := isPassthrough(apiOp, schema, verb)
		if passthrough {
			return passthroughPartitions, nil
		}
		sort.Slice(partitions, func(i, j int) bool {
			return partitions[i].(Partition).Namespace < partitions[j].(Partition).Namespace
		})
		return partitions, nil
	default:
		return nil, fmt.Errorf("parition all: invalid verb %s", verb)
	}
}

func (p *rbacPartitioner) Store(apiOp *types.APIRequest, partition partition.Partition) (types.Store, error) {
	return &byNameOrNamespaceStore{
		Store:     p.proxyStore,
		partition: partition.(Partition),
	}, nil
}

type byNameOrNamespaceStore struct {
	*Store
	partition Partition
}

func (b *byNameOrNamespaceStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	if b.partition.Passthrough {
		return b.Store.List(apiOp, schema)
	}

	apiOp.Namespace = b.partition.Namespace
	if b.partition.All {
		return b.Store.List(apiOp, schema)
	}
	return b.Store.ByNames(apiOp, schema, b.partition.Names)
}

func (b *byNameOrNamespaceStore) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	if b.partition.Passthrough {
		return b.Store.Watch(apiOp, schema, wr)
	}

	apiOp.Namespace = b.partition.Namespace
	if b.partition.All {
		return b.Store.Watch(apiOp, schema, wr)
	}
	return b.Store.WatchNames(apiOp, schema, wr, b.partition.Names)
}

func isPassthrough(apiOp *types.APIRequest, schema *types.APISchema, verb string) ([]partition.Partition, bool) {
	partitions, passthrough := isPassthroughUnconstrained(apiOp, schema, verb)
	namespaces, ok := getNamespaceConstraint(apiOp.Request)
	if !ok {
		return partitions, passthrough
	}

	var result []partition.Partition

	if passthrough {
		for namespace := range namespaces {
			result = append(result, Partition{
				Namespace: namespace,
				All:       true,
			})
		}
		return result, false
	}

	for _, partition := range partitions {
		if namespaces.Has(partition.Name()) {
			result = append(result, partition)
		}
	}

	return result, false
}

func isPassthroughUnconstrained(apiOp *types.APIRequest, schema *types.APISchema, verb string) ([]partition.Partition, bool) {
	accessListByVerb, _ := attributes.Access(schema).(accesscontrol.AccessListByVerb)
	if accessListByVerb.All(verb) {
		return nil, true
	}

	resources := accessListByVerb.Granted(verb)
	if apiOp.Namespace != "" {
		if resources[apiOp.Namespace].All {
			return nil, true
		} else {
			return []partition.Partition{
				Partition{
					Namespace: apiOp.Namespace,
					Names:     resources[apiOp.Namespace].Names,
				},
			}, false
		}
	}

	var result []partition.Partition

	if attributes.Namespaced(schema) {
		for k, v := range resources {
			result = append(result, Partition{
				Namespace: k,
				All:       v.All,
				Names:     v.Names,
			})
		}
	} else {
		for _, v := range resources {
			result = append(result, Partition{
				All:   v.All,
				Names: v.Names,
			})
		}
	}

	return result, false
}
