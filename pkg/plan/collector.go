package plan

import (
	"errors"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	labelClusterName = "rke.cattle.io/cluster-name"
	labelInitNode    = "rke.cattle.io/init-node"
	labelEtcd        = "rke.cattle.io/etcd-role"
	labelControl     = "rke.cattle.io/controlplane-role"
	labelWorker      = "rke.cattle.io/worker-role"
	labelOS          = "cattle.io/os"

	osWindows = "windows"

	SecretTypeMachinePlan = "rke.cattle.io/machine-plan"
)

// SecretClient defines the contract for fetching secrets.
type SecretClient interface {
	List(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error)
}

// FilterFunc decides whether to keep a single secret in a result set. It runs after the cache
// fetch, so it can inspect fields a label selector cannot (annotations, data, etc.).
// Returning true keeps the secret; returning false drops it.
type FilterFunc func(s *corev1.Secret) bool

// SorterFunc orders a result set. It is invoked once on the deduplicated+filtered slice and
// returns the slice in the desired order. Sorters should be stable for deterministic outputs.
type SorterFunc func(s []*corev1.Secret) []*corev1.Secret

// Validator inspects a final result set and returns an error if the set is unsuitable for
// downstream use. Validators run after filtering and sorting and short-circuit Collect on the
// first error.
type Validator func(s []*corev1.Secret) error

// ClusterRef is the minimal interface a cluster object must satisfy to scope a Collector to a
// single cluster. *unstructured.Unstructured and most typed cluster objects satisfy this.
type ClusterRef interface {
	GetName() string
}

// DefaultSorter returns a SorterFunc that orders machine-plan secrets by a priority derived from
// their role+OS labels (init-etcd first, plain etcd next, then mixed roles, with Windows workers
// last), breaking ties lexicographically by Name. Use it as the default when the call site has no
// stronger preference.
func DefaultSorter() SorterFunc {
	return func(secrets []*corev1.Secret) []*corev1.Secret {
		sort.Slice(secrets, func(i, j int) bool {
			a := secrets[i]
			b := secrets[j]

			aScore := getNodeScore(a.Labels)
			bScore := getNodeScore(b.Labels)

			// Compare combination score
			if aScore != bScore {
				return aScore < bScore // Lower score means higher priority
			}

			// Fallback to lexicographical sort by Name
			return a.Name < b.Name
		})

		return secrets
	}
}

// getNodeScore assigns a precise numerical rank based on exact role combinations. Lower scores
// rank higher in DefaultSorter's ordering.
func getNodeScore(labels map[string]string) int {
	if labels == nil {
		return 99 // Unlabeled / Unknown priority
	}

	i := labels[labelInitNode] == "true"
	e := labels[labelEtcd] == "true"
	c := labels[labelControl] == "true"
	w := labels[labelWorker] == "true"

	// We assume anything that isn't explicitly Windows is Linux (or defaults to Linux preference)
	isWin := labels[labelOS] == osWindows

	switch {
	case i && e:
		return 1
	case e && !c && !w:
		return 2 // etcd only
	case e && !c && w:
		return 3 // etcd + worker
	case e && c && !w:
		return 4 // etcd + controlplane
	case e && c && w:
		return 5 // etcd + controlplane + worker
	case !e && c && !w:
		return 6 // controlplane only
	case !e && c && w:
		return 7 // controlplane + worker
	case !e && !c && w:
		if isWin {
			return 9 // windows worker
		}
		return 8 // linux worker
	default:
		return 99 // Catch-all for unlabeled or unexpected combinations
	}
}

// Selector defines the contract for building and compiling label queries.
type Selector interface {
	// ToK8sSelectors evaluates the AST and returns a slice of valid K8s selector strings.
	ToK8sSelectors() []labels.Selector
}

// label is a single key=value match. It produces exactly one underlying labels.Selector.
type label struct{ key, value string }

// andSelector composes its children as a conjunction. Each underlying selector must match.
type andSelector struct{ selectors []Selector }

// orSelector composes its children as a disjunction. Any underlying selector may match.
type orSelector struct{ selectors []Selector }

func (l label) ToK8sSelectors() []labels.Selector {
	return []labels.Selector{labels.SelectorFromSet(labels.Set{l.key: l.value})}
}

// ToK8sSelectors for AND performs a Cartesian product of all child selectors.
// Example: AND([A], OR[B, C]) -> ["A,B", "A,C"]
func (a andSelector) ToK8sSelectors() []labels.Selector {
	result := []labels.Selector{labels.Nothing()}
	for _, child := range a.selectors {
		childSelectors := child.ToK8sSelectors()
		var newResult []labels.Selector
		for _, existing := range result {
			for _, cs := range childSelectors {
				if existing == labels.Nothing() {
					newResult = append(newResult, cs)
				} else {
					req, _ := cs.Requirements()
					newResult = append(newResult, existing.DeepCopySelector().Add(req...))
				}
			}
		}
		result = newResult
	}
	return result
}

// ToK8sSelectors for OR simply flattens and concatenates child queries.
func (o orSelector) ToK8sSelectors() []labels.Selector {
	var result []labels.Selector
	for _, child := range o.selectors {
		result = append(result, child.ToK8sSelectors()...)
	}
	return result
}

// Label returns a Selector that matches a single key=value label.
func Label(key, value string) Selector { return label{key: key, value: value} }

// And returns a Selector that matches the conjunction of all provided selectors.
// At the cache level this expands to one query per branch of any nested OR.
func And(selectors ...Selector) Selector { return andSelector{selectors: selectors} }

// Or returns a Selector that matches the disjunction of all provided selectors. The cache is
// queried once per branch and the results are deduplicated by UID.
func Or(selectors ...Selector) Selector { return orSelector{selectors: selectors} }

// Collector is a fluent query builder for machine-plan secrets scoped to a single cluster. The
// cluster's name is auto-applied as a rke.cattle.io/cluster-name label filter; additional label
// selectors, post-fetch filters, sorters, and validators can be chained on before Collect.
//
// Example:
//
//	secrets, err := planapi.NewCollector(cache, clusterObj, namespace).
//	    WithLabels(planapi.Label(capr.EtcdRoleLabel, "true")).
//	    WithSorter(planapi.DefaultSorter()).
//	    WithValidator(planapi.AtLeast(1, capr.EtcdRoleLabel)).
//	    Collect()
//
// Collector is not safe for concurrent use — build one per query.
type Collector struct {
	client      SecretClient
	namespace   string
	clusterName string
	selectors   []Selector
	filters     []FilterFunc
	sorters     []SorterFunc
	validators  []Validator
}

// NewCollector creates a Collector scoped to the given cluster's machine-plan secrets within
// the given namespace.
//
// The cluster's name (via cluster.GetName()) is automatically AND-composed into every query as a
// rke.cattle.io/cluster-name=<name> filter — callers do not need to pass it through WithLabels.
//
// Passing a nil cluster returns a Collector that matches every secret in the namespace (no
// auto-filter), which is occasionally useful for cluster-list operations but is rarely what you
// want — prefer passing the cluster object explicitly.
func NewCollector(cache SecretClient, cluster ClusterRef, namespace string) *Collector {
	c := &Collector{
		client:    cache,
		namespace: namespace,
	}
	if cluster != nil {
		c.clusterName = cluster.GetName()
	}
	return c
}

// WithLabels adds one or more label selectors to the query. All selectors AND-compose with the
// auto-applied cluster name filter and with each other. WithLabels may be called multiple times;
// each call appends to the existing selector set.
//
// To express disjunctions, wrap with Or(...):
//
//	c.WithLabels(planapi.Or(planapi.Label(...), planapi.Label(...)))
func (c *Collector) WithLabels(selectors ...Selector) *Collector {
	c.selectors = append(c.selectors, selectors...)
	return c
}

// WithFilter adds a post-fetch predicate that runs against each candidate secret. Multiple
// WithFilter calls AND together — a secret is kept only when every filter returns true.
//
// Use WithFilter for conditions a label selector cannot express (annotations, data fields, or
// "absent label means linux"-style defaults).
func (c *Collector) WithFilter(filter FilterFunc) *Collector {
	if filter != nil {
		c.filters = append(c.filters, filter)
	}
	return c
}

// WithSorter sets a SorterFunc applied to the final result set. Multiple WithSorter calls chain
// in declaration order; later sorters re-sort the slice and so effectively break ties left by
// earlier ones (sort.Slice is not stable — wrap with a stable sorter if you need ordering
// guarantees beyond the last sorter).
func (c *Collector) WithSorter(sorter SorterFunc) *Collector {
	if sorter != nil {
		c.sorters = append(c.sorters, sorter)
	}
	return c
}

// WithValidator appends one or more Validators that run against the filtered + sorted result.
// Validators short-circuit Collect on the first error; their order matches the WithValidator
// call order.
//
// Use WithValidator to assert invariants that should fail the operation rather than silently
// proceed with an empty or invalid set:
//
//	c.WithValidator(planapi.AtLeast(1, capr.EtcdRoleLabel))
//	c.WithValidator(planapi.Exactly(1, capr.InitNodeLabel))
func (c *Collector) WithValidator(validators ...Validator) *Collector {
	for _, v := range validators {
		if v != nil {
			c.validators = append(c.validators, v)
		}
	}
	return c
}

type CollectorError struct {
	Err       error
	Transient bool
}

func (c CollectorError) Error() string {
	return c.Err.Error()
}

func (c CollectorError) Unwrap() error {
	return c.Err
}

// IsTransient returns true if the CollectorError is transient (e.g. cache failure).
// By default, all non-CollectorError errors are transient.
// A `nil` error is considered not transient.
// The primary use case for this is to distinguish between cache failures and validation failures.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := errors.AsType[CollectorError](err); ok {
		return e.Transient
	}
	return true
}

// Collect executes the query pipeline: fetch by selector → deduplicate by UID → filter → sort →
// validate. Returns the resulting secrets, or the first error encountered (cache failure or
// validator failure).
//
// The cluster name auto-filter and the WithLabels selectors are combined into a single AND.
// Any nested OR is expanded into multiple cache queries and the results are deduplicated by UID
// before filtering.
func (c *Collector) Collect() ([]*corev1.Secret, error) {
	if c.client == nil {
		return nil, errors.New("plan: Collector has no SecretCache")
	}

	root := c.composedSelector()

	queries := []labels.Selector{labels.Everything()}
	if root != nil {
		queries = root.ToK8sSelectors()
	}

	var secrets []*corev1.Secret
	for _, sel := range queries {
		list, err := c.client.List(c.namespace, metav1.ListOptions{
			LabelSelector: sel.String(),
			FieldSelector: fmt.Sprintf("type=%s", SecretTypeMachinePlan),
		})
		if err != nil {
			return nil, err
		} else if list == nil {
			continue
		}
		for secret := range list.Items {
			secrets = append(secrets, &list.Items[secret])
		}
	}

	// Deduplicate by UID. Overlapping OR branches can surface the same secret more than once.
	uniqueMap := make(map[string]*corev1.Secret, len(secrets))
	for _, s := range secrets {
		key := string(s.UID)
		if key == "" { // Fallback if UID isn't populated in mock data
			key = s.Name
		}
		uniqueMap[key] = s
	}

	uniqueSecrets := make([]*corev1.Secret, 0, len(uniqueMap))
	for _, s := range uniqueMap {
		uniqueSecrets = append(uniqueSecrets, s)
	}

	// Run all filters.
	filtered := make([]*corev1.Secret, 0, len(uniqueSecrets))
	for _, s := range uniqueSecrets {
		keep := true
		for _, f := range c.filters {
			if !f(s) {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, s)
		}
	}

	// Apply sorters in order.
	result := filtered
	for _, sortFunc := range c.sorters {
		result = sortFunc(result)
	}

	// Run validators against the final set. First failure wins.
	for _, v := range c.validators {
		if err := v(result); err != nil {
			return nil, CollectorError{Err: err, Transient: false}
		}
	}

	return result, nil
}

// composedSelector builds the root Selector for the query: the auto cluster-name filter AND any
// WithLabels selectors. Returns nil when no filters are configured (caller treats nil as
// "match-everything" via labels.Everything()).
func (c *Collector) composedSelector() Selector {
	var parts []Selector
	if c.clusterName != "" {
		parts = append(parts, Label(labelClusterName, c.clusterName))
	}
	parts = append(parts, c.selectors...)
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	default:
		return And(parts...)
	}
}

// AtLeast returns a Validator that requires at least n secrets in the result to carry the given
// label set to "true". An empty labelKey checks the total count of the result set instead.
//
// Use AtLeast as a guard against silently proceeding with an empty set:
//
//	c.WithValidator(planapi.AtLeast(1, capr.EtcdRoleLabel))
func AtLeast(n int, labelKey string) Validator {
	return func(secrets []*corev1.Secret) error {
		got := countWithLabel(secrets, labelKey)
		if got < n {
			return fmt.Errorf("plan: expected at least %d secrets matching %s, got %d", n, describeLabel(labelKey), got)
		}
		return nil
	}
}

// AtMost returns a Validator that fails when more than n secrets in the result carry the given
// label set to "true". An empty labelKey checks the total count of the result set instead.
//
// Use AtMost to catch unexpected duplicates — e.g. more than one init node in a cluster:
//
//	c.WithValidator(planapi.AtMost(1, capr.InitNodeLabel))
func AtMost(n int, labelKey string) Validator {
	return func(secrets []*corev1.Secret) error {
		got := countWithLabel(secrets, labelKey)
		if got > n {
			return fmt.Errorf("plan: expected at most %d secrets matching %s, got %d", n, describeLabel(labelKey), got)
		}
		return nil
	}
}

// Exactly returns a Validator that requires exactly n secrets in the result to carry the given
// label set to "true". An empty labelKey checks the total count of the result set instead.
//
// Exactly(n, key) is equivalent to chaining AtLeast(n, key) and AtMost(n, key) but reports a
// single combined error message.
func Exactly(n int, labelKey string) Validator {
	return func(secrets []*corev1.Secret) error {
		got := countWithLabel(secrets, labelKey)
		if got != n {
			return fmt.Errorf("plan: expected exactly %d secrets matching %s, got %d", n, describeLabel(labelKey), got)
		}
		return nil
	}
}

// countWithLabel returns the number of secrets with labelKey="true". An empty labelKey returns
// the total length of the slice.
func countWithLabel(secrets []*corev1.Secret, labelKey string) int {
	if labelKey == "" {
		return len(secrets)
	}
	n := 0
	for _, s := range secrets {
		if s != nil && s.Labels[labelKey] == "true" {
			n++
		}
	}
	return n
}

// describeLabel renders a label key for inclusion in validator error messages.
func describeLabel(labelKey string) string {
	if labelKey == "" {
		return "any label"
	}
	return fmt.Sprintf("label %q", labelKey)
}
