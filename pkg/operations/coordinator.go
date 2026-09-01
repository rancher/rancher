package operations

import (
	"encoding/json"
	"fmt"
	"strings"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// DynamicClient is the subset of the dynamic controller the Coordinator needs in order to annotate an
// operation's cluster, whatever kind of cluster that is.
type DynamicClient interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	Update(obj runtime.Object) (runtime.Object, error)
}

// Coordinator implements the parts of the operations contract which span more than one kind of
// operation: cancelling the operations a new operation is permitted to preempt, and recording on the
// cluster that a cancelled operation left work half-done.
//
// None of that behaviour is specific to a kind of operation. Which operations may preempt which, and
// which of them require a restore after being cancelled, is declared by annotations on each operation's
// CustomResourceDefinition, so adding a kind of operation means adding its annotations and registering
// it in kinds - not changing the logic here.
//
// A nil *Coordinator is inert: every method is a no-op. Controllers under test therefore need not
// construct one.
type Coordinator struct {
	crds    generic.NonNamespacedCacheInterface[*apiextv1.CustomResourceDefinition]
	dynamic DynamicClient
	kinds   []operationKind
}

// NewCoordinator returns a Coordinator for every registered kind of operation.
func NewCoordinator(clients *wrangler.CAPIContext, dynamicClient DynamicClient) *Coordinator {
	return &Coordinator{
		crds:    clients.CRD.CustomResourceDefinition().Cache(),
		dynamic: dynamicClient,
		kinds:   kinds(clients),
	}
}

// operationKind is everything the Coordinator needs to work with a kind of operation without knowing
// its Go type.
type operationKind struct {
	kind     string
	resource string

	list  func() ([]*opv1alpha1.Operation, error)
	patch func(namespace, name string, data []byte) error
}

// kinds returns the registered kinds of operation. A new kind of operation is added here, and needs
// nothing else from this package.
func kinds(clients *wrangler.CAPIContext) []operationKind {
	return []operationKind{
		kindFor("ETCDSnapshotSave", "etcdsnapshotsaves", clients.Operation.ETCDSnapshotSave()),
		kindFor("ETCDSnapshotRestore", "etcdsnapshotrestores", clients.Operation.ETCDSnapshotRestore()),
		kindFor("EncryptionKeyRotation", "encryptionkeyrotations", clients.Operation.EncryptionKeyRotation()),
	}
}

// kindFor adapts a generated operation controller to operationKind. Listing goes through the
// controller's cache, and cancellation is a merge patch, so neither needs access to the fields of the
// concrete type.
func kindFor[T generic.RuntimeMetaObject, TL runtime.Object](kind, resource string, controller generic.ControllerInterface[T, TL]) operationKind {
	return operationKind{
		kind:     kind,
		resource: resource,
		list: func() ([]*opv1alpha1.Operation, error) {
			objs, err := controller.Cache().List(allNamespaces, labels.Everything())
			if err != nil {
				return nil, err
			}

			operations := make([]*opv1alpha1.Operation, 0, len(objs))
			for _, obj := range objs {
				operation, err := opv1alpha1.ToOperation(obj)
				if err != nil {
					return nil, err
				}
				if operation.Kind == "" {
					operation.Kind = kind
				}

				operations = append(operations, operation)
			}

			return operations, nil
		},
		patch: func(namespace, name string, data []byte) error {
			_, err := controller.Patch(namespace, name, types.MergePatchType, data)
			return err
		},
	}
}

// allNamespaces is the empty namespace a wrangler cache interprets as "every namespace".
const allNamespaces = ""

// Preempt requests cancellation of every in-flight operation which op is permitted to preempt, and
// returns the identities of those it is not permitted to preempt. An empty result means the beacon is
// or will shortly be free for op to acquire.
func (c *Coordinator) Preempt(op *opv1alpha1.Operation) ([]string, error) {
	if c == nil {
		return nil, nil
	}

	priority, err := c.priority(op.Kind)
	if err != nil {
		return nil, err
	}

	inFlight, err := c.InFlight(op)
	if err != nil {
		return nil, err
	}

	var blocking []string
	for _, other := range inFlight {
		otherPriority, err := c.priority(other.Kind)
		if err != nil {
			return nil, err
		}

		if !opv1alpha1.CanPreempt(priority, otherPriority) {
			blocking = append(blocking, other.Key())
			continue
		}

		if other.Spec.Cancel {
			// Already cancelling; it will release the beacon on its own.
			continue
		}

		logrus.Infof("[operations] %s: preempting %s", op.Key(), other.Key())

		if err := c.cancel(other, op.Key()); err != nil {
			return nil, err
		}
	}

	return blocking, nil
}

// InFlight returns every non-terminal operation which targets the same cluster as op, excluding op
// itself.
func (c *Coordinator) InFlight(op *opv1alpha1.Operation) ([]*opv1alpha1.Operation, error) {
	if c == nil {
		return nil, nil
	}

	var result []*opv1alpha1.Operation
	for _, kind := range c.kinds {
		operations, err := kind.list()
		if err != nil {
			return nil, fmt.Errorf("unable to list %s: %w", kind.resource, err)
		}

		for _, other := range operations {
			if other.Status.Phase.IsTerminal() || op.SameAs(other) || !opv1alpha1.SameCluster(op.Spec.ClusterRef, other.Spec.ClusterRef) {
				continue
			}

			result = append(result, other)
		}
	}

	return result, nil
}

// OnCanceled records on the operation's cluster that it needs to be restored, if the operation's kind
// declares that cancelling it may leave the cluster in a state only a restore can recover. It reports
// whether the cluster was marked.
//
// Callers should only invoke this for an operation which had begun executing: an operation canceled
// before it started has not mutated the cluster and so does not require a restore.
func (c *Coordinator) OnCanceled(op *opv1alpha1.Operation) (bool, error) {
	if c == nil {
		return false, nil
	}

	required, err := c.boolAnnotation(op.Kind, opv1alpha1.RestoreRequiredOnCancellationAnnotation)
	if err != nil {
		return false, err
	}

	if !required {
		return false, nil
	}

	logrus.Infof("[operations] %s: marking cluster %s as requiring a restore", op.Key(), op.Spec.ClusterRef.Name)

	if err := c.annotateCluster(op.Spec.ClusterRef, opv1alpha1.RestoreRequiredAnnotation, op.Key()); err != nil {
		return false, err
	}

	return true, nil
}

// OnSucceeded clears the restore-required mark from the operation's cluster, if the operation's kind
// declares that completing it returns the cluster to a known-good state.
func (c *Coordinator) OnSucceeded(op *opv1alpha1.Operation) error {
	if c == nil {
		return nil
	}

	clears, err := c.boolAnnotation(op.Kind, opv1alpha1.ClearsRestoreRequiredAnnotation)
	if err != nil {
		return err
	}

	if !clears {
		return nil
	}

	return c.annotateCluster(op.Spec.ClusterRef, opv1alpha1.RestoreRequiredAnnotation, "")
}

// cancel requests cancellation of an operation on behalf of the operation preempting it.
func (c *Coordinator) cancel(op *opv1alpha1.Operation, canceledBy string) error {
	kind, ok := c.kindFor(op.Kind)
	if !ok {
		return fmt.Errorf("unable to cancel unknown kind of operation %s", op.Kind)
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{opv1alpha1.CanceledByAnnotation: canceledBy},
		},
		"spec": map[string]any{"cancel": true},
	})
	if err != nil {
		return err
	}

	return kind.patch(op.Metadata.Namespace, op.Metadata.Name, patch)
}

// annotateCluster sets an annotation on the object an operation references, or removes it when the value
// is empty.
func (c *Coordinator) annotateCluster(ref *corev1.ObjectReference, key, value string) error {
	if ref == nil {
		return nil
	}

	gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)

	obj, err := c.dynamic.Get(gvk, ref.Namespace, ref.Name)
	if err != nil {
		return fmt.Errorf("unable to get %s %s: %w", ref.Kind, ObjectKey(ref.Namespace, ref.Name), err)
	}

	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}

	cluster := &unstructured.Unstructured{Object: objMap}

	annotations := cluster.GetAnnotations()
	if value == "" {
		if _, ok := annotations[key]; !ok {
			return nil
		}
		delete(annotations, key)
	} else {
		if annotations[key] == value {
			return nil
		}
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[key] = value
	}

	cluster = cluster.DeepCopy()
	cluster.SetAnnotations(annotations)

	_, err = c.dynamic.Update(cluster)
	return err
}

// priority returns the preemption priority declared by the given kind's CustomResourceDefinition.
func (c *Coordinator) priority(kind string) (int, error) {
	crd, err := c.crd(kind)
	if err != nil {
		return 0, err
	}

	return opv1alpha1.PreemptionPriority(crd.Annotations), nil
}

// boolAnnotation reports whether the given kind's CustomResourceDefinition declares the given annotation.
func (c *Coordinator) boolAnnotation(kind, key string) (bool, error) {
	crd, err := c.crd(kind)
	if err != nil {
		return false, err
	}

	return opv1alpha1.BoolAnnotation(crd.Annotations, key), nil
}

// crd returns the CustomResourceDefinition of the given kind of operation.
func (c *Coordinator) crd(kind string) (*apiextv1.CustomResourceDefinition, error) {
	registered, ok := c.kindFor(kind)
	if !ok {
		return nil, fmt.Errorf("unknown kind of operation %s", kind)
	}

	crd, err := c.crds.Get(registered.resource + "." + opv1alpha1.SchemeGroupVersion.Group)
	if err != nil {
		return nil, fmt.Errorf("unable to get CustomResourceDefinition for %s: %w", kind, err)
	}

	return crd, nil
}

func (c *Coordinator) kindFor(kind string) (operationKind, bool) {
	for _, registered := range c.kinds {
		if registered.kind == kind {
			return registered, true
		}
	}

	return operationKind{}, false
}

// WaitingForBeaconMessage returns the message for an operation which is waiting for the beacon, naming
// the operations it is not permitted to preempt when there are any.
func WaitingForBeaconMessage(blocking []string) string {
	if len(blocking) == 0 {
		return "Waiting to acquire the beacon"
	}

	return "Waiting for " + strings.Join(blocking, ", ") + " to finish"
}

// CanceledReason returns the reason to record on a canceled operation, distinguishing an operation
// canceled by a user from one preempted by another operation.
func CanceledReason(op metav1.Object) string {
	if op.GetAnnotations()[opv1alpha1.CanceledByAnnotation] != "" {
		return opv1alpha1.PreemptedReason
	}

	return opv1alpha1.CanceledByUserReason
}

// CanceledMessage returns the message to record on a canceled operation. When the cluster was marked as
// requiring a restore, that is included so the reason the cluster needs attention is visible on the
// operation which caused it.
func CanceledMessage(op metav1.Object, clusterRef *corev1.ObjectReference, restoreRequired bool) string {
	message := "Canceled by user request"
	if by := op.GetAnnotations()[opv1alpha1.CanceledByAnnotation]; by != "" {
		message = "Preempted by " + by
	}

	if restoreRequired {
		message += "; " + opv1alpha1.RestoreRequiredMessage(clusterRef)
	}

	return message
}

// ObjectKey formats a namespace and name for use in messages and conditions.
func ObjectKey(namespace, name string) string {
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
