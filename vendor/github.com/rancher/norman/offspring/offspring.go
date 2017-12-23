package offspring

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"encoding/json"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types/values"
	"github.com/sirupsen/logrus"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type ParentLookup func(obj runtime.Object) *ParentReference
type Generator func(obj runtime.Object) (ObjectSet, error)

type Enqueue func(namespace, name string)

type ObjectReference struct {
	Kind       string
	Namespace  string
	Name       string
	APIVersion string
}

type ParentReference struct {
	Namespace string
	Name      string
}

type ObjectSet struct {
	Parent   runtime.Object
	Children []runtime.Object
	Complete bool
}

type KnownObjectSet struct {
	Children map[ObjectReference]runtime.Object
}

func (k KnownObjectSet) clone() KnownObjectSet {
	newMap := map[ObjectReference]runtime.Object{}
	for k, v := range k.Children {
		newMap[k] = v
	}
	return KnownObjectSet{
		Children: newMap,
	}
}

type ChildWatch struct {
	ObjectClient clientbase.ObjectClient
	Informer     cache.SharedInformer
}

type change struct {
	parent   ParentReference
	childRef ObjectReference
	child    runtime.Object
	delete   bool
}

type Reconciliation struct {
	sync.Mutex
	Generator    Generator
	Enqueue      Enqueue
	ObjectClient *clientbase.ObjectClient
	Children     []ChildWatcher

	running       bool
	changes       chan change
	children      map[ParentReference]KnownObjectSet
	childWatchers map[schema.GroupVersionKind]*ChildWatcher
	keys          map[string]bool
}

type ChildWatcher struct {
	ObjectClient *clientbase.ObjectClient
	Informer     cache.SharedInformer
	Scheme       runtime.Scheme
	// optional
	CompareKeys []string
	// optional
	ParentLookup ParentLookup

	watcher *Reconciliation
	keys    map[string]bool
}

func NewReconciliation(ctx context.Context, generator Generator, enqueue Enqueue, client *clientbase.ObjectClient, children ...ChildWatcher) *Reconciliation {
	r := &Reconciliation{
		Generator:     generator,
		Enqueue:       enqueue,
		ObjectClient:  client,
		running:       true,
		changes:       make(chan change, 10),
		children:      map[ParentReference]KnownObjectSet{},
		childWatchers: map[schema.GroupVersionKind]*ChildWatcher{},
		keys:          getKeys(client.Factory.Object(), nil),
	}

	for _, child := range children {
		if child.ParentLookup == nil {
			child.ParentLookup = OwnerReferenceLookup(r.ObjectClient.GroupVersionKind())
		}
		child.watcher = r
		if len(child.CompareKeys) == 0 {
			child.keys = getKeys(child.ObjectClient.Factory.Object(), map[string]bool{"Status": true})
		} else {
			child.keys = map[string]bool{}
			for _, key := range child.CompareKeys {
				child.keys[key] = true
			}
		}

		childCopy := child
		child.Informer.AddEventHandler(&childCopy)
		r.childWatchers[child.ObjectClient.GroupVersionKind()] = &childCopy
	}

	go r.run()
	go func() {
		<-ctx.Done()
		r.Lock()
		r.running = false
		close(r.changes)
		r.Unlock()
	}()

	return r
}

func OwnerReferenceLookup(gvk schema.GroupVersionKind) ParentLookup {
	return func(obj runtime.Object) *ParentReference {
		meta, err := apimeta.Accessor(obj)
		if err != nil {
			logrus.Errorf("Failed to look up parent for %v", obj)
			return nil
		}

		var ownerRef *metav1.OwnerReference
		for i, owner := range meta.GetOwnerReferences() {
			if owner.Controller != nil && *owner.Controller {
				ownerRef = &meta.GetOwnerReferences()[i]
				break
			}
		}

		if ownerRef == nil {
			return nil
		}

		apiVersion, kind := gvk.ToAPIVersionAndKind()
		if ownerRef.APIVersion != apiVersion || ownerRef.Kind != kind {
			return nil
		}

		return &ParentReference{
			Name:      ownerRef.Name,
			Namespace: meta.GetNamespace(),
		}
	}
}

func getKeys(obj interface{}, ignore map[string]bool) map[string]bool {
	keys := map[string]bool{}

	keys["metadata"] = true
	value := reflect.ValueOf(obj)
	t := value.Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	numFields := t.NumField()
	for i := 0; i < numFields; i++ {
		field := t.Field(i)
		if field.Name != "" && !field.Anonymous && !ignore[field.Name] {
			keys[field.Name] = true
		}
	}

	return keys
}

func (w *ChildWatcher) OnAdd(obj interface{}) {
	w.changed(obj, false)
}

func (w *ChildWatcher) OnUpdate(oldObj, newObj interface{}) {
	w.changed(newObj, false)
}

func (w *ChildWatcher) OnDelete(obj interface{}) {
	w.changed(obj, true)
}

func (w *ChildWatcher) changed(obj interface{}, deleted bool) {
	ro, ok := obj.(runtime.Object)
	if !ok {
		logrus.Errorf("Failed to cast %s to runtime.Object", reflect.TypeOf(obj))
		return
	}

	parent := w.ParentLookup(ro)
	if parent == nil {
		return
	}

	meta, err := apimeta.Accessor(ro)
	if err != nil {
		logrus.Errorf("Failed to access metadata of runtime.Object: %v", err)
		return
	}

	w.watcher.Lock()
	if w.watcher.running {
		gvk := w.ObjectClient.GroupVersionKind()
		apiVersion, kind := gvk.ToAPIVersionAndKind()
		w.watcher.changes <- change{
			parent: *parent,
			childRef: ObjectReference{
				Namespace:  meta.GetNamespace(),
				Name:       meta.GetName(),
				Kind:       kind,
				APIVersion: apiVersion,
			},
			child:  ro,
			delete: deleted,
		}
	}
	w.watcher.Unlock()
}

func (w *Reconciliation) Changed(key string, obj runtime.Object) (runtime.Object, error) {
	var (
		err       error
		objectSet ObjectSet
	)

	if obj == nil {
		objectSet.Complete = true
	} else {
		objectSet, err = w.Generator(obj)
		if err != nil {
			return obj, err
		}
	}

	parentRef := keyToParentReference(key)
	existingSet := w.children[parentRef]

	if objectSet.Parent != nil {
		newObj, err := w.updateParent(parentRef, obj, objectSet.Parent)
		if err != nil {
			return obj, err
		}
		obj = newObj
		objectSet.Parent = obj
	}

	var lastErr error
	newChildRefs := map[ObjectReference]bool{}

	for _, child := range objectSet.Children {
		childRef, err := createRef(child)
		if err != nil {
			return obj, err
		}
		newChildRefs[childRef] = true
		existingChild, ok := existingSet.Children[childRef]
		if ok {
			if _, err := w.updateChild(childRef, existingChild, child); err != nil {
				lastErr = err
			}
		} else {
			if _, err := w.createChild(obj, childRef, child); err != nil {
				lastErr = err
			}
		}
	}

	if objectSet.Complete {
		for childRef, child := range existingSet.Children {
			if !newChildRefs[childRef] {
				if err := w.deleteChild(childRef, child); err != nil {
					lastErr = err
				}
			}
		}
	}

	return obj, lastErr
}

func createRef(obj runtime.Object) (ObjectReference, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	ref := ObjectReference{}
	ref.APIVersion, ref.Kind = gvk.ToAPIVersionAndKind()

	meta, err := apimeta.Accessor(obj)
	if err != nil {
		return ref, err
	}

	ref.Name = meta.GetName()
	ref.Namespace = meta.GetNamespace()

	if ref.Name == "" || ref.Kind == "" || ref.APIVersion == "" {
		return ref, fmt.Errorf("name, kind, or apiVersion is blank %v", ref)
	}

	return ref, nil
}

func (w *Reconciliation) createChild(parent runtime.Object, reference ObjectReference, object runtime.Object) (runtime.Object, error) {
	childWatcher, err := w.getChildWatcher(reference)
	if err != nil {
		return object, err
	}

	parentMeta, err := apimeta.Accessor(parent)
	if err != nil {
		return object, err
	}

	meta, err := apimeta.Accessor(object)
	if err != nil {
		return object, err
	}

	if meta.GetNamespace() == parentMeta.GetNamespace() {
		trueValue := true
		ownerRef := metav1.OwnerReference{
			Name:               parentMeta.GetName(),
			UID:                parentMeta.GetUID(),
			BlockOwnerDeletion: &trueValue,
			Controller:         &trueValue,
		}
		gvk := parent.GetObjectKind().GroupVersionKind()
		ownerRef.APIVersion, ownerRef.Kind = gvk.ToAPIVersionAndKind()
		meta.SetOwnerReferences(append(meta.GetOwnerReferences(), ownerRef))
	}

	return childWatcher.ObjectClient.Create(object)
}

func (w *Reconciliation) deleteChild(reference ObjectReference, object runtime.Object) error {
	childWatcher, err := w.getChildWatcher(reference)
	if err != nil {
		return err
	}

	policy := metav1.DeletePropagationForeground
	return childWatcher.ObjectClient.DeleteNamespace(reference.Name, reference.Namespace, &metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

func (w *Reconciliation) getChildWatcher(reference ObjectReference) (*ChildWatcher, error) {
	gvk := schema.FromAPIVersionAndKind(reference.APIVersion, reference.Kind)
	childWatcher, ok := w.childWatchers[gvk]
	if !ok {
		return nil, fmt.Errorf("failed to find childWatcher for %s", gvk)
	}
	return childWatcher, nil
}

func keyToParentReference(key string) ParentReference {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 1 {
		return ParentReference{
			Name: parts[0],
		}
	}
	return ParentReference{
		Namespace: parts[0],
		Name:      parts[1],
	}
}

func (w *Reconciliation) run() {
	for change := range w.changes {
		w.Lock()
		children := w.children[change.parent]
		w.Unlock()

		children = children.clone()
		if change.delete {
			delete(children.Children, change.childRef)
		} else {
			children.Children[change.childRef] = change.child
		}

		w.Lock()
		if len(children.Children) == 0 {
			delete(w.children, change.parent)
		} else {
			w.children[change.parent] = children
		}
		w.Unlock()

		w.Enqueue(change.parent.Namespace, change.parent.Name)
	}
}

func (w *Reconciliation) updateParent(parentRef ParentReference, oldObj runtime.Object, newObj runtime.Object) (runtime.Object, error) {
	reference := ObjectReference{
		Name:      parentRef.Name,
		Namespace: parentRef.Namespace,
	}

	gvk := w.ObjectClient.GroupVersionKind()
	reference.APIVersion, reference.Kind = gvk.ToAPIVersionAndKind()
	return w.updateObject(reference, oldObj, newObj, w.ObjectClient, w.keys)
}

func (w *Reconciliation) updateChild(reference ObjectReference, oldObj runtime.Object, newObj runtime.Object) (runtime.Object, error) {
	childWatcher, err := w.getChildWatcher(reference)
	if err != nil {
		return nil, err
	}

	return w.updateObject(reference, oldObj, newObj, childWatcher.ObjectClient, childWatcher.keys)
}

func (w *Reconciliation) updateObject(reference ObjectReference, oldObj runtime.Object, newObj runtime.Object, client *clientbase.ObjectClient, keys map[string]bool) (runtime.Object, error) {
	changes := map[string]interface{}{}
	oldObj = oldObj.DeepCopyObject()
	oldValue := reflect.ValueOf(oldObj).Elem()
	newValue := reflect.ValueOf(newObj).Elem()

	for key := range keys {
		if key == "metadata" {
			oldMeta, err := apimeta.Accessor(oldObj)
			if err != nil {
				return nil, err
			}

			newMeta, err := apimeta.Accessor(newObj)
			if err != nil {
				return nil, err
			}

			if data, changed := compareMaps(oldMeta.GetLabels(), newMeta.GetLabels()); changed {
				values.PutValue(changes, data, "metadata", "labels")
			}
			if data, changed := compareMaps(oldMeta.GetAnnotations(), newMeta.GetAnnotations()); changed {
				values.PutValue(changes, data, "metadata", "annotations")
			}
		} else {
			oldField := oldValue.FieldByName(key)
			oldIValue := oldField.Interface()
			newField := newValue.FieldByName(key)
			newIValue := newField.Interface()
			if !reflect.DeepEqual(oldIValue, newIValue) {
				oldField.Set(newField)
				changeName := jsonName(newValue, key)
				if changeName != "-" {
					changes[changeName] = newIValue
				}
			}
		}
	}

	if len(changes) > 0 {
		meta, err := apimeta.Accessor(oldObj)
		if err == nil {
			values.PutValue(changes, meta.GetResourceVersion(), "metadata", "resourceVersion")
		}

		data, err := json.Marshal(changes)
		if err != nil {
			return newObj, err
		}
		return client.Patch(reference.Name, oldObj, data)
	}

	return newObj, nil
}

func jsonName(value reflect.Value, fieldName string) string {
	field, _ := value.Type().FieldByName(fieldName)
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "" {
		return fieldName
	}
	return name
}

func compareMaps(oldValues, newValues map[string]string) (map[string]string, bool) {
	changed := false
	mergedValues := map[string]string{}

	for k, v := range oldValues {
		mergedValues[k] = v
	}

	for k, v := range newValues {
		oldV, ok := oldValues[k]
		if !ok || v != oldV {
			changed = true
		}
		mergedValues[k] = v
	}

	return mergedValues, changed
}
