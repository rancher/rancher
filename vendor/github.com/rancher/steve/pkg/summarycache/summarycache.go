package summarycache

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schema/converter"
	"github.com/rancher/wrangler/pkg/slice"
	"github.com/rancher/wrangler/pkg/summary"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const (
	relationshipIndex = "relationshipIndex"
)

var (
	cbID = 0
)

type Relationship struct {
	ToID        string `json:"toId,omitempty"`
	ToType      string `json:"toType,omitempty"`
	ToNamespace string `json:"toNamespace,omitempty"`
	FromID      string `json:"fromId,omitempty"`
	FromType    string `json:"fromType,omitempty"`
	Rel         string `json:"rel,omitempty"`
	Selector    string `json:"selector,omitempty"`
}

type SummaryCache struct {
	sync.RWMutex
	cache   cache.ThreadSafeStore
	schemas *schema.Collection
	cbs     map[int]chan *summary.Relationship
}

func New(schemas *schema.Collection) *SummaryCache {
	indexers := cache.Indexers{}
	s := &SummaryCache{
		cache:   cache.NewThreadSafeStore(indexers, cache.Indices{}),
		schemas: schemas,
		cbs:     map[int]chan *summary.Relationship{},
	}
	indexers[relationshipIndex] = s.relationshipIndexer
	return s
}

func (s *SummaryCache) OnInboundRelationshipChange(ctx context.Context, schema *types.APISchema, namespace string) <-chan *summary.Relationship {
	s.Lock()
	defer s.Unlock()

	apiVersion, kind := attributes.GVK(schema).ToAPIVersionAndKind()
	ret := make(chan *summary.Relationship, 100)
	cb := make(chan *summary.Relationship, 100)
	id := cbID
	cbID++
	s.cbs[id] = cb

	go func() {
		defer close(ret)
		for rel := range cb {
			if rel.Kind == kind &&
				rel.APIVersion == apiVersion &&
				rel.Namespace == namespace {
				ret <- rel
			}
		}
	}()

	go func() {
		<-ctx.Done()
		s.Lock()
		defer s.Unlock()
		delete(s.cbs, id)
	}()

	return cb
}

func (s *SummaryCache) SummaryAndRelationship(obj runtime.Object) (*summary.SummarizedObject, []Relationship) {
	s.RLock()
	defer s.RUnlock()

	key := toKey(obj)
	summarized := summary.Summarized(obj)

	relObjs, err := s.cache.ByIndex(relationshipIndex, key)
	if err != nil {
		return summarized, nil
	}

	var (
		rels      []Relationship
		selectors = map[string]bool{}
	)

	for _, rel := range summarized.Relationships {
		if rel.Selector != nil {
			selectors[rel.APIVersion+"/"+rel.Kind] = true
		}
		rels = append(rels, s.toRel(summarized.Namespace, &rel))
	}

	for _, relObj := range relObjs {
		summary := relObj.(*summary.SummarizedObject)
		for _, rel := range summary.Relationships {
			if !s.refersTo(summarized, &rel) {
				continue
			}
			// drop references that an existing selector reference will cover
			if rel.Inbound && len(selectors) > 0 && selectors[rel.APIVersion+"/"+rel.Kind] {
				continue
			}
			rels = append(rels, s.reverseRel(summary, rel))
		}
	}

	return summarized, rels
}

func (s *SummaryCache) reverseRel(summarized *summary.SummarizedObject, rel summary.Relationship) Relationship {
	return s.toRel(summarized.Namespace, &summary.Relationship{
		Name:       summarized.Name,
		Namespace:  summarized.Namespace,
		Kind:       summarized.Kind,
		APIVersion: summarized.APIVersion,
		Inbound:    !rel.Inbound,
		Type:       rel.Type,
	})
}

func toSelector(sel *metav1.LabelSelector) string {
	if sel == nil {
		return ""
	}
	result, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return ""
	}
	return result.String()
}

func (s *SummaryCache) toRel(ns string, rel *summary.Relationship) Relationship {
	ns = s.resolveNamespace(ns, rel.Namespace, runtimeschema.FromAPIVersionAndKind(rel.APIVersion, rel.Kind))

	id := rel.Name
	if id != "" && ns != "" {
		id = ns + "/" + rel.Name
	}

	if rel.Inbound {
		return Relationship{
			FromID:   id,
			FromType: converter.GVKToSchemaID(runtimeschema.FromAPIVersionAndKind(rel.APIVersion, rel.Kind)),
			Rel:      rel.Type,
		}
	}

	toNS := ""
	if rel.Selector != nil {
		toNS = ns
	}

	return Relationship{
		ToID:        id,
		ToType:      converter.GVKToSchemaID(runtimeschema.FromAPIVersionAndKind(rel.APIVersion, rel.Kind)),
		Rel:         rel.Type,
		ToNamespace: toNS,
		Selector:    toSelector(rel.Selector),
	}
}

func (s *SummaryCache) Add(obj runtime.Object) {
	summary, rels := s.process(obj)
	key := toKey(summary)

	s.cache.Add(key, summary)
	for _, rel := range rels {
		s.notify(rel)
	}
}

func (s *SummaryCache) notify(rel *summary.Relationship) {
	go func() {
		s.Lock()
		defer s.Unlock()
		for _, cb := range s.cbs {
			cb <- rel
		}
	}()
}

func (s *SummaryCache) Remove(obj runtime.Object) {
	summary, rels := s.process(obj)
	key := toKey(summary)

	s.cache.Delete(key)
	for _, rel := range rels {
		s.notify(rel)
	}
}

func (s *SummaryCache) Change(newObj, oldObj runtime.Object) {
	_, oldRels := s.process(oldObj)
	summary, rels := s.process(newObj)
	key := toKey(summary)

	if len(rels) == len(oldRels) {
		for i, rel := range rels {
			if !relEquals(oldRels[i], rel) {
				s.notify(rel)
			}
		}
	}
	s.cache.Update(key, summary)
}

func (s *SummaryCache) process(obj runtime.Object) (*summary.SummarizedObject, []*summary.Relationship) {
	var (
		rels    []*summary.Relationship
		summary = summary.Summarized(obj)
	)

	for _, rel := range summary.Relationships {
		gvk := runtimeschema.FromAPIVersionAndKind(rel.APIVersion, rel.Kind)
		schemaID := converter.GVKToSchemaID(gvk)
		schema := s.schemas.Schema(schemaID)
		if schema == nil {
			continue
		}
		copy := rel
		if copy.Namespace == "" && attributes.Namespaced(schema) {
			copy.Namespace = summary.Namespace
		}
		rels = append(rels, &copy)
	}

	return summary, rels
}

func (s *SummaryCache) relationshipIndexer(obj interface{}) (result []string, err error) {
	var (
		summary = obj.(*summary.SummarizedObject)
	)

	for _, rel := range summary.Relationships {
		gvk := runtimeschema.FromAPIVersionAndKind(rel.APIVersion, rel.Kind)
		result = append(result, toKeyFrom(s.resolveNamespace(summary.Namespace, rel.Namespace, gvk), rel.Name, gvk))
	}

	return
}

func (s *SummaryCache) resolveNamespace(sourceNamespace, toNamespace string, gvk runtimeschema.GroupVersionKind) string {
	if toNamespace != "" {
		return toNamespace
	}
	schema := s.schemas.Schema(converter.GVKToSchemaID(gvk))
	if schema == nil || !attributes.Namespaced(schema) {
		return toNamespace
	}
	return sourceNamespace
}

func (s *SummaryCache) refersTo(summarized *summary.SummarizedObject, rel *summary.Relationship) bool {
	if summarized.APIVersion != rel.APIVersion ||
		summarized.Kind != rel.Kind ||
		summarized.Name != rel.Name {
		return false
	}
	if summarized.Namespace == "" && rel.Namespace == "" {
		return true
	}
	ns := s.resolveNamespace(summarized.Namespace, rel.Namespace, summarized.GroupVersionKind())
	return summarized.Namespace == ns
}

func (s *SummaryCache) OnAdd(gvr runtimeschema.GroupVersionResource, key string, obj runtime.Object) error {
	s.Add(obj)
	return nil
}

func (s *SummaryCache) OnRemove(gvr runtimeschema.GroupVersionResource, key string, obj runtime.Object) error {
	s.Remove(obj)
	return nil
}

func (s *SummaryCache) OnChange(gvr runtimeschema.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
	s.Change(obj, oldObj)
	return nil
}

func toKeyFrom(namespace, name string, gvk runtimeschema.GroupVersionKind, other ...string) string {
	parts := []string{
		gvk.Group,
		gvk.Version,
		gvk.Kind,
		namespace,
		name,
	}
	parts = append(parts, other...)
	return strings.Join(parts, ",")
}

func toKey(obj runtime.Object) string {
	var (
		name, namespace = "", ""
		gvk             = obj.GetObjectKind().GroupVersionKind()
	)

	m, err := meta.Accessor(obj)
	if err == nil {
		name = m.GetName()
		namespace = m.GetNamespace()
	}

	return toKeyFrom(namespace, name, gvk)
}

func toRelKey(key string, index int) string {
	return fmt.Sprintf("%s:%d", key, index)
}

func relEquals(left, right *summary.Relationship) bool {
	if left == nil && right == nil {
		return true
	} else if left == nil || right == nil {
		return false
	}

	return left.Name == right.Name &&
		left.Namespace == right.Namespace &&
		left.ControlledBy == right.ControlledBy &&
		left.Kind == right.Kind &&
		left.APIVersion == right.APIVersion &&
		left.Inbound == right.Inbound &&
		left.Type == right.Type &&
		selEquals(left.Selector, right.Selector)
}

func selEquals(left, right *metav1.LabelSelector) bool {
	if left == nil && right == nil {
		return true
	} else if left == nil || right == nil {
		return false
	}

	return reqEquals(left.MatchExpressions, right.MatchExpressions) &&
		mapEquals(left.MatchLabels, right.MatchLabels)
}

func reqEquals(left, right []metav1.LabelSelectorRequirement) bool {
	if len(left) != len(right) {
		return false
	}
	for i, right := range right {
		left := left[i]
		if left.Key != right.Key ||
			left.Operator != right.Operator ||
			!slice.StringsEqual(left.Values, right.Values) {
			return false
		}
	}
	return true
}

func mapEquals(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for k, v := range right {
		if left[k] != v {
			return false
		}
	}
	return true
}
