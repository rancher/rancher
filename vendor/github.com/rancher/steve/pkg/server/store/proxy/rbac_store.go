package proxy

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"
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

type RBACStore struct {
	*Store
}

type Partition struct {
	Namespace string
	All       bool
	Names     sets.String
}

func isPassthrough(apiOp *types.APIRequest, schema *types.APISchema, verb string) ([]Partition, bool) {
	partitions, passthrough := isPassthroughUnconstrained(apiOp, schema, verb)
	namespaces, ok := getNamespaceConstraint(apiOp.Request)
	if !ok {
		return partitions, passthrough
	}

	var result []Partition

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
		if namespaces.Has(partition.Namespace) {
			result = append(result, partition)
		}
	}

	return result, false
}

func isPassthroughUnconstrained(apiOp *types.APIRequest, schema *types.APISchema, verb string) ([]Partition, bool) {
	accessListByVerb, _ := attributes.Access(schema).(accesscontrol.AccessListByVerb)
	if accessListByVerb.All(verb) {
		return nil, true
	}

	resources := accessListByVerb.Granted(verb)
	if apiOp.Namespace != "" {
		if resources[apiOp.Namespace].All {
			return nil, true
		} else {
			return []Partition{
				{
					Namespace: apiOp.Namespace,
					Names:     resources[apiOp.Namespace].Names,
				},
			}, false
		}
	}

	var result []Partition

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

func (r *RBACStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	partitions, passthrough := isPassthrough(apiOp, schema, "list")
	if passthrough {
		return r.Store.List(apiOp, schema)
	}

	resume := apiOp.Request.URL.Query().Get("continue")
	limit := getLimit(apiOp.Request)

	sort.Slice(partitions, func(i, j int) bool {
		return partitions[i].Namespace < partitions[j].Namespace
	})

	lister := &ParallelPartitionLister{
		Lister: func(ctx context.Context, partition Partition, cont string, revision string, limit int) (types.APIObjectList, error) {
			return r.list(apiOp, schema, partition, cont, revision, limit)
		},
		Concurrency: 3,
		Partitions:  partitions,
	}

	result := types.APIObjectList{}
	items, err := lister.List(apiOp.Context(), limit, resume)
	if err != nil {
		return result, err
	}

	for item := range items {
		result.Objects = append(result.Objects, item...)
	}

	result.Continue = lister.Continue()
	result.Revision = lister.Revision()
	return result, lister.Err()
}

func getLimit(req *http.Request) int {
	limitString := req.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitString)
	if err != nil {
		limit = 0
	}
	if limit <= 0 {
		limit = 100000
	}
	return limit
}

func (r *RBACStore) list(apiOp *types.APIRequest, schema *types.APISchema, partition Partition, cont, revision string, limit int) (types.APIObjectList, error) {
	req := *apiOp
	req.Namespace = partition.Namespace
	req.Request = req.Request.Clone(apiOp.Context())

	values := req.Request.URL.Query()
	values.Set("continue", cont)
	values.Set("revision", revision)
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	} else {
		values.Del("limit")
	}
	req.Request.URL.RawQuery = values.Encode()

	if partition.All {
		return r.Store.List(&req, schema)
	}
	return r.Store.ByNames(&req, schema, partition.Names)
}

func (r *RBACStore) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	partitions, passthrough := isPassthrough(apiOp, schema, "watch")
	if passthrough {
		return r.Store.Watch(apiOp, schema, w)
	}

	ctx, cancel := context.WithCancel(apiOp.Context())
	apiOp = apiOp.WithContext(ctx)

	eg := errgroup.Group{}
	response := make(chan types.APIEvent)
	for _, partition := range partitions {
		partition := partition
		eg.Go(func() error {
			defer cancel()

			var (
				c   chan types.APIEvent
				err error
			)

			req := *apiOp
			req.Namespace = partition.Namespace
			if partition.All {
				c, err = r.Store.Watch(&req, schema, w)
			} else {
				c, err = r.Store.WatchNames(&req, schema, w, partition.Names)
			}
			if err != nil {
				return err
			}
			for i := range c {
				response <- i
			}
			return nil
		})
	}

	go func() {
		defer close(response)
		<-ctx.Done()
		eg.Wait()
	}()

	return response, nil
}
