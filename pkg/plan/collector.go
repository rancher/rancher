package plan

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SecretCache defines the contract for fetching secrets.
type SecretCache interface {
	List(namespace string, selector labels.Selector) ([]*corev1.Secret, error)
}

type FilterFunc func(s *corev1.Secret) bool
type SorterFunc func(s []*corev1.Secret) []*corev1.Secret

const (
	labelInitNode = "rke.cattle.io/init-node"
	labelEtcd     = "rke.cattle.io/etcd-role"
	labelControl  = "rke.cattle.io/controlplane-role"
	labelWorker   = "rke.cattle.io/worker-role"
	labelOS       = "cattle.io/os"

	osWindows = "windows"
)

// DefaultSorter sorts secrets by specific role+OS combination priorities, then by Name.
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

// getNodeScore assigns a precise numerical rank based on exact role combinations.
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

type label struct{ key, value string }
type andSelector struct{ selectors []Selector }
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

// Helper functions for clean composition syntax
func Label(key, value string) Selector   { return label{key: key, value: value} }
func And(selectors ...Selector) Selector { return andSelector{selectors: selectors} }
func Or(selectors ...Selector) Selector  { return orSelector{selectors: selectors} }

type LabelBuilder interface {
	And(selectors ...Selector) LabelBuilder
	WithFilter(f FilterFunc) FilteredBuilder
	WithSorter(s SorterFunc) SortedBuilder
	Collect(cache SecretCache, namespace string) ([]*corev1.Secret, error)
}

type FilteredBuilder interface {
	WithFilter(f FilterFunc) FilteredBuilder
	WithSorter(s SorterFunc) SortedBuilder
	Collect(cache SecretCache, namespace string) ([]*corev1.Secret, error)
}

type SortedBuilder interface {
	WithSorter(s SorterFunc) SortedBuilder
	Collect(cache SecretCache, namespace string) ([]*corev1.Secret, error)
}

type collector struct {
	selector Selector
	filters  []FilterFunc
	sorters  []SorterFunc
}

// NewLabeler initializes the builder pipeline.
func NewLabeler() LabelBuilder {
	return &collector{}
}

func (c *collector) And(selectors ...Selector) LabelBuilder {
	c.selector = And(selectors...)
	return c
}

func (c *collector) WithFilter(f FilterFunc) FilteredBuilder {
	c.filters = append(c.filters, f)
	return c
}

func (c *collector) WithSorter(s SorterFunc) SortedBuilder {
	c.sorters = append(c.sorters, s)
	return c
}

// Collect executes the query pipeline: Fetch -> Deduplicate -> Filter -> Sort.
func (c *collector) Collect(cache SecretCache, namespace string) ([]*corev1.Secret, error) {
	var secrets []*corev1.Secret

	queries := []labels.Selector{} // Fetch nothing if none provided
	if c.selector != nil {
		queries = c.selector.ToK8sSelectors()
	}

	for _, sel := range queries {
		list, err := cache.List(namespace, sel)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, list...)
	}

	// Use UID to deduplicate secrets fetched by overlapping OR queries
	// The top-most selector should always be "And" with the cluster name label, and then an OR to prevent selecting
	// secrets from other clusters unintentionally.
	uniqueMap := make(map[string]*corev1.Secret)
	for _, s := range secrets {
		key := string(s.UID)
		if key == "" { // Fallback if UID isn't populated in mock data
			key = s.Name
		}
		uniqueMap[key] = s
	}

	var uniqueSecrets []*corev1.Secret
	for _, s := range uniqueMap {
		uniqueSecrets = append(uniqueSecrets, s)
	}

	// Filter secrets that cannot be excluded by labels
	var filteredSecrets []*corev1.Secret
	for _, s := range uniqueSecrets {
		keep := true
		for _, f := range c.filters {
			if !f(s) {
				keep = false
				break
			}
		}
		if keep {
			filteredSecrets = append(filteredSecrets, s)
		}
	}

	result := filteredSecrets
	for _, sortFunc := range c.sorters {
		result = sortFunc(result)
	}

	return result, nil
}
