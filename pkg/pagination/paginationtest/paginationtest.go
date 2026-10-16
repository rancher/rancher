// Package paginationtest provides fakes for tests of code that lists Kubernetes objects in pages.
package paginationtest

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/pagination"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tokenPrefix = "token-page-"

	// maxCalls stops a test when the code under test ignores the continue token.
	maxCalls = 50
)

// ErrListFailed is the error that FailOnPage uses when it gets a nil error.
var ErrListFailed = errors.New("simulated list error")

// ExpiredError returns the error that the API server sends for an expired continue token.
// The error contains token as the new continue token.
func ExpiredError(token string) error {
	err := apierrors.NewResourceExpired("continue token expired")
	err.ErrStatus.ListMeta.Continue = token
	return err
}

// Pager serves a snapshot of items in pages and records every list call.
// The continue token for the page with the index N is "token-page-N". The first page has no token.
type Pager[T any] struct {
	// Items is the snapshot. NewPager copies the items, so deletes do not shift the pages.
	Items []T
	// PerPage is the number of items on each page. A value of zero or less puts all items on one page.
	PerPage int
	// ListErrs maps the zero-based index of a list call to the error that the call returns instead of a page.
	ListErrs map[int]error
	// ExtraTokens maps an additional continue token to the index of the page to serve for it.
	ExtraTokens map[string]int
	// Calls records the options of every list call.
	Calls []metav1.ListOptions

	// pageEnds holds the end offset of each page in Items. It overrides PerPage when set.
	pageEnds []int
}

// NewPager returns a Pager for a copy of items.
func NewPager[T any](items []T, perPage int) *Pager[T] {
	return &Pager[T]{Items: slices.Clone(items), PerPage: perPage}
}

// NewPagesPager returns a Pager with the given pages. The pages can have different sizes.
// A call for a page after the last page returns a bad request error.
func NewPagesPager[T any](pages ...[]T) *Pager[T] {
	p := &Pager[T]{pageEnds: []int{}}
	for _, page := range pages {
		p.Items = append(p.Items, page...)
		p.pageEnds = append(p.pageEnds, len(p.Items))
	}
	return p
}

// FailOnPage makes the list call for page number page return err. The first page is number 1.
// A nil err means ErrListFailed. A page number of zero or less does nothing.
func (p *Pager[T]) FailOnPage(page int, err error) *Pager[T] {
	if page <= 0 {
		return p
	}
	if err == nil {
		err = ErrListFailed
	}
	if p.ListErrs == nil {
		p.ListErrs = map[int]error{}
	}
	p.ListErrs[page-1] = err
	return p
}

// Serve returns the page for the options and the continue token of the next page.
func (p *Pager[T]) Serve(opts metav1.ListOptions) ([]T, string, error) {
	idx := len(p.Calls)
	p.Calls = append(p.Calls, opts)
	if idx >= maxCalls {
		return nil, "", fmt.Errorf("more than %d list calls, the continue token is probably ignored", maxCalls)
	}
	if err, ok := p.ListErrs[idx]; ok {
		return nil, "", err
	}

	page := 0
	if opts.Continue != "" {
		var ok bool
		if page, ok = p.pageOf(opts.Continue); !ok {
			return nil, "", apierrors.NewBadRequest("unknown continue token " + opts.Continue)
		}
	}

	start, end, hasNext, ok := p.bounds(page)
	if !ok {
		return nil, "", apierrors.NewBadRequest(fmt.Sprintf("page %d is out of range", page))
	}
	next := ""
	if hasNext {
		next = tokenPrefix + strconv.Itoa(page+1)
	}
	return slices.Clone(p.Items[start:end]), next, nil
}

// bounds returns the offsets of the items on a page and whether a page follows it.
func (p *Pager[T]) bounds(page int) (start, end int, hasNext, ok bool) {
	if p.pageEnds != nil {
		if page >= len(p.pageEnds) {
			return 0, 0, false, false
		}
		if page > 0 {
			start = p.pageEnds[page-1]
		}
		return start, p.pageEnds[page], page+1 < len(p.pageEnds), true
	}

	size := p.PerPage
	if size <= 0 {
		size = len(p.Items) + 1
	}
	start = min(page*size, len(p.Items))
	end = min(start+size, len(p.Items))
	return start, end, end < len(p.Items), true
}

func (p *Pager[T]) pageOf(token string) (int, bool) {
	if page, ok := p.ExtraTokens[token]; ok {
		return page, page >= 0
	}
	if after, ok := strings.CutPrefix(token, tokenPrefix); ok {
		page, err := strconv.Atoi(after)
		return page, err == nil && page >= 0
	}
	return 0, false
}

// Tokens returns the continue token of every list call.
func (p *Pager[T]) Tokens() []string {
	var out []string
	for _, o := range p.Calls {
		out = append(out, o.Continue)
	}
	return out
}

// AssertLimit fails the test if a list call did not set Limit to pagination.PageSize.
func (p *Pager[T]) AssertLimit(t testing.TB) {
	t.Helper()
	for i, o := range p.Calls {
		if o.Limit != pagination.PageSize {
			t.Errorf("list call %d has limit %d, want %d", i, o.Limit, pagination.PageSize)
		}
	}
}
