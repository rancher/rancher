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

	// maxConsecutiveExpiredRetries limits the consecutive resumes after an expired continue token.
	maxConsecutiveExpiredRetries = 3
)

// ErrStop stops EachPage without an error. The callback returns it.
var ErrStop = errors.New("stop paging")

// Option changes the behavior of EachPage.
type Option func(*settings)

type settings struct {
	noResume       bool
	ignoreNotFound bool
}

// WithoutResume makes EachPage return the expired error instead of resuming.
// Use it when fn needs a consistent view of all pages, for example to find orphans.
func WithoutResume() Option {
	return func(s *settings) { s.noResume = true }
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
// When the API server expires a continue token, it returns a new token in the error.
// EachPage resumes from that token. The resumed list is not a consistent snapshot.
// EachPage does not call fn twice for the same page.
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

	var expiredRetries int
	for {
		page, err := list(opts)
		if err != nil {
			if cfg.ignoreNotFound && opts.Continue == "" && apierrors.IsNotFound(err) {
				return nil
			}
			token, ok := expiredContinueToken(err)
			if cfg.noResume || !ok || opts.Continue == "" || token == opts.Continue || expiredRetries >= maxConsecutiveExpiredRetries {
				return err
			}
			expiredRetries++
			opts.Continue = token
			continue
		}
		expiredRetries = 0

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

// expiredContinueToken returns the continue token that the API server sends with an expired error.
func expiredContinueToken(err error) (string, bool) {
	if !apierrors.IsResourceExpired(err) {
		return "", false
	}
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return "", false
	}
	token := status.Status().ListMeta.Continue
	return token, token != ""
}
