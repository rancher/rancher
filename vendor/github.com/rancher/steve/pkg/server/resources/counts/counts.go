package counts

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/schemaserver/store/empty"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/summary"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ignore = map[string]bool{
		"count":   true,
		"schema":  true,
		"apiRoot": true,
	}
)

func Register(schemas *types.APISchemas, ccache clustercache.ClusterCache) {
	schemas.MustImportAndCustomize(Count{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet}
		schema.Attributes["access"] = accesscontrol.AccessListByVerb{
			"watch": accesscontrol.AccessList{
				{
					Namespace:    "*",
					ResourceName: "*",
				},
			},
		}
		schema.Store = &Store{
			ccache: ccache,
		}
	})
}

type Count struct {
	ID     string               `json:"id,omitempty"`
	Counts map[string]ItemCount `json:"counts"`
}

type Summary struct {
	Count         int            `json:"count,omitempty"`
	States        map[string]int `json:"states,omitempty"`
	Error         int            `json:"errors,omitempty"`
	Transitioning int            `json:"transitioning,omitempty"`
}

type ItemCount struct {
	Summary    Summary            `json:"summary,omitempty"`
	Namespaces map[string]Summary `json:"namespaces,omitempty"`
	Revision   int                `json:"revision,omitempty"`
}

type Store struct {
	empty.Store
	ccache clustercache.ClusterCache
}

func toAPIObject(c Count) types.APIObject {
	return types.APIObject{
		Type:   "count",
		ID:     c.ID,
		Object: c,
	}
}

func (s *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	c := s.getCount(apiOp)
	return toAPIObject(c), nil
}

func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	c := s.getCount(apiOp)
	return types.APIObjectList{
		Objects: []types.APIObject{
			toAPIObject(c),
		},
	}, nil
}

func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	var (
		result      = make(chan types.APIEvent, 100)
		counts      map[string]ItemCount
		gvrToSchema = map[schema2.GroupVersionResource]*types.APISchema{}
		countLock   sync.Mutex
	)

	counts = s.getCount(apiOp).Counts
	for id := range counts {
		schema := apiOp.Schemas.LookupSchema(id)
		if schema == nil {
			continue
		}

		gvrToSchema[attributes.GVR(schema)] = schema
	}

	go func() {
		<-apiOp.Context().Done()
		countLock.Lock()
		close(result)
		result = nil
		countLock.Unlock()
	}()

	onChange := func(add bool, gvr schema2.GroupVersionResource, _ string, obj, oldObj runtime.Object) error {
		countLock.Lock()
		defer countLock.Unlock()

		if result == nil {
			return nil
		}

		schema := gvrToSchema[gvr]
		if schema == nil {
			return nil
		}

		_, namespace, revision, summary, ok := getInfo(obj)
		if !ok {
			return nil
		}

		itemCount := counts[schema.ID]
		if revision <= itemCount.Revision {
			return nil
		}

		if oldObj != nil {
			if _, _, _, oldSummary, ok := getInfo(oldObj); ok {
				if oldSummary.Transitioning == summary.Transitioning &&
					oldSummary.Error == summary.Error &&
					oldSummary.State == summary.State {
					return nil
				}
				itemCount = removeCounts(itemCount, namespace, oldSummary)
			} else {
				return nil
			}
		} else if add {
			itemCount = addCounts(itemCount, namespace, summary)
		} else {
			itemCount = removeCounts(itemCount, namespace, summary)
		}

		counts[schema.ID] = itemCount
		countsCopy := map[string]ItemCount{}
		for k, v := range counts {
			ns := map[string]Summary{}
			for i, j := range v.Namespaces {
				ns[i] = j
			}
			countsCopy[k] = ItemCount{
				Summary:    v.Summary,
				Revision:   v.Revision,
				Namespaces: ns,
			}
		}

		result <- types.APIEvent{
			Name:         "resource.change",
			ResourceType: "counts",
			Object: toAPIObject(Count{
				ID:     "count",
				Counts: countsCopy,
			}),
		}

		return nil
	}

	s.ccache.OnAdd(apiOp.Context(), func(gvr schema2.GroupVersionResource, key string, obj runtime.Object) error {
		return onChange(true, gvr, key, obj, nil)
	})
	s.ccache.OnChange(apiOp.Context(), func(gvr schema2.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
		return onChange(true, gvr, key, obj, oldObj)
	})
	s.ccache.OnRemove(apiOp.Context(), func(gvr schema2.GroupVersionResource, key string, obj runtime.Object) error {
		return onChange(false, gvr, key, obj, nil)
	})

	return result, nil
}

func (s *Store) schemasToWatch(apiOp *types.APIRequest) (result []*types.APISchema) {
	for _, schema := range apiOp.Schemas.Schemas {
		if ignore[schema.ID] {
			continue
		}

		if schema.Store == nil {
			continue
		}

		if apiOp.AccessControl.CanList(apiOp, schema) != nil {
			continue
		}

		if apiOp.AccessControl.CanWatch(apiOp, schema) != nil {
			continue
		}

		result = append(result, schema)
	}

	return
}

func getInfo(obj interface{}) (name string, namespace string, revision int, summaryResult summary.Summary, ok bool) {
	r, ok := obj.(runtime.Object)
	if !ok {
		return "", "", 0, summaryResult, false
	}

	meta, err := meta.Accessor(r)
	if err != nil {
		return "", "", 0, summaryResult, false
	}

	revision, err = strconv.Atoi(meta.GetResourceVersion())
	if err != nil {
		return "", "", 0, summaryResult, false
	}

	if unstr, ok := obj.(*unstructured.Unstructured); ok {
		summaryResult = summary.Summarize(unstr)
	}

	return meta.GetName(), meta.GetNamespace(), revision, summaryResult, true
}

func removeCounts(itemCount ItemCount, ns string, summary summary.Summary) ItemCount {
	itemCount.Summary = removeSummary(itemCount.Summary, summary)
	if ns == "" {
		itemCount.Namespaces[ns] = removeSummary(itemCount.Namespaces[ns], summary)
	}
	return itemCount
}

func addCounts(itemCount ItemCount, ns string, summary summary.Summary) ItemCount {
	itemCount.Summary = addSummary(itemCount.Summary, summary)
	if ns == "" {
		itemCount.Namespaces[ns] = addSummary(itemCount.Namespaces[ns], summary)
	}
	return itemCount
}

func removeSummary(counts Summary, summary summary.Summary) Summary {
	counts.Count--
	if summary.Transitioning {
		counts.Transitioning--
	}
	if summary.Error {
		counts.Error--
	}
	if summary.State != "" {
		if counts.States == nil {
			counts.States = map[string]int{}
		}
		counts.States[summary.State] -= 1
	}
	return counts
}

func addSummary(counts Summary, summary summary.Summary) Summary {
	counts.Count++
	if summary.Transitioning {
		counts.Transitioning++
	}
	if summary.Error {
		counts.Error++
	}
	if summary.State != "" {
		if counts.States == nil {
			counts.States = map[string]int{}
		}
		counts.States[summary.State] += 1
	}
	return counts
}

func (s *Store) getCount(apiOp *types.APIRequest) Count {
	counts := map[string]ItemCount{}

	for _, schema := range s.schemasToWatch(apiOp) {
		gvr := attributes.GVR(schema)
		access, _ := attributes.Access(schema).(accesscontrol.AccessListByVerb)

		rev := 0
		itemCount := ItemCount{
			Namespaces: map[string]Summary{},
		}

		all := access.Grants("list", "*", "*")

		for _, obj := range s.ccache.List(gvr) {
			name, ns, revision, summary, ok := getInfo(obj)
			if !ok {
				continue
			}

			if !all && !access.Grants("list", ns, name) && !access.Grants("get", ns, name) {
				continue
			}

			if revision > rev {
				rev = revision
			}

			itemCount = addCounts(itemCount, ns, summary)
		}

		itemCount.Revision = rev
		counts[schema.ID] = itemCount
	}

	return Count{
		ID:     "count",
		Counts: counts,
	}
}
