package ensure

import (
	"github.com/rancher/wrangler/pkg/generic"
	"k8s.io/apimachinery/pkg/runtime"
)

// ExistingStrategy describes the strategy to find currently existing objects that an ensurer should validate
// and process as invalid or valid
type ExistingStrategy[T runtime.Object] interface {
	GetCurrent() ([]T, error)
}

// IndexedExistingStrategy implements ExistingStrategy using an Indexer, provided
// the Cache, IndexName, and Key
type IndexedExistingStrategy[T runtime.Object] struct {
	Cache     generic.CacheInterface[T]
	IndexName string
	Key       string
}

func (i *IndexedExistingStrategy[T]) GetCurrent() ([]T, error) {
	return i.Cache.GetByIndex(i.IndexName, i.Key)
}
