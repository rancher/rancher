// Package pagination provides a helper that lists Kubernetes objects in pages.
package pagination

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// PageSize is the number of objects that EachPage requests in each List call.
	PageSize int64 = 100
)

// ErrStop stops EachPage without an error. The callback returns it.
var ErrStop = errors.New("stop paging")

// Option changes the behavior of EachPage.
type Option func(*settings)

type settings struct {
	ignoreNotFound bool
}

// WithIgnoreNotFound makes EachPage return nil when the first list call returns NotFound.
// EachPage still returns NotFound from later list calls and from fn.
func WithIgnoreNotFound() Option {
	return func(s *settings) { s.ignoreNotFound = true }
}

// EachPage lists objects in pages of PageSize and calls fn once for each page.
// Selectors in opts apply to every page. EachPage sets opts.Limit to PageSize
// and clears opts.ResourceVersion and opts.ResourceVersionMatch.
//
// EachPage does not resume a list after the API server expires a continue token (HTTP 410).
// A resumed list is not a consistent snapshot and can skip objects. EachPage returns the error,
// so the caller can run the whole operation again.
//
// EachPage stops at the first error and returns it unchanged.
// If a response has the same continue token as its request, EachPage stops with a new error.
// If fn returns ErrStop, EachPage stops and returns nil.
func EachPage[L runtime.Object](list func(metav1.ListOptions) (L, error), opts metav1.ListOptions, fn func(L) error, options ...Option) error {
	var cfg settings
	for _, o := range options {
		o(&cfg)
	}
	opts.Limit = PageSize
	opts.ResourceVersion = ""
	opts.ResourceVersionMatch = ""

	for {
		page, err := list(opts)
		if err != nil {
			if cfg.ignoreNotFound && opts.Continue == "" && apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if err := fn(page); err != nil {
			if errors.Is(err, ErrStop) {
				return nil
			}
			return err
		}

		accessor, err := meta.ListAccessor(page)
		if err != nil {
			return fmt.Errorf("failed to read list metadata: %w", err)
		}
		next := accessor.GetContinue()
		if next == "" {
			return nil
		}
		if next == opts.Continue {
			return errors.New("the API server returned the same continue token for consecutive pages")
		}
		opts.Continue = next
	}
}
