package pagination_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/pagination"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// listResult is one scripted response of the fake list function.
type listResult struct {
	items []string
	next  string
	err   error
}

func TestEachPage(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "x")

	tests := []struct {
		name           string
		opts           metav1.ListOptions
		results        []listResult
		fnErrOnPage    int   // 1-based page that makes fn fail; 0 means never
		fnErr          error // error that fn returns on fnErrOnPage; defaults to errBoom
		noResume       bool
		ignoreNotFound bool
		wantItems      []string
		wantTokens     []string // Continue value of each list call
		wantErr        error
		wantErrCheck   func(error) bool
	}{
		{
			name:       "single page",
			results:    []listResult{{items: []string{"a", "b"}}},
			wantItems:  []string{"a", "b"},
			wantTokens: []string{""},
		},
		{
			name: "three pages pass each token to the next call",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{items: []string{"b"}, next: "t2"},
				{items: []string{"c"}},
			},
			wantItems:  []string{"a", "b", "c"},
			wantTokens: []string{"", "t1", "t2"},
		},
		{
			name: "list error on page 2 is returned unchanged",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: errBoom},
			},
			wantItems:  []string{"a"},
			wantTokens: []string{"", "t1"},
			wantErr:    errBoom,
		},
		{
			name: "callback error stops paging",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{items: []string{"b"}},
			},
			fnErrOnPage: 1,
			wantItems:   []string{"a"},
			wantTokens:  []string{""},
			wantErr:     errBoom,
		},
		{
			name: "ErrStop stops paging without an error",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{items: []string{"b"}},
			},
			fnErrOnPage: 1,
			fnErr:       pagination.ErrStop,
			wantItems:   []string{"a"},
			wantTokens:  []string{""},
		},
		{
			name: "expired token resumes from the token in the error",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t1-new")},
				{items: []string{"b"}},
			},
			wantItems:  []string{"a", "b"},
			wantTokens: []string{"", "t1", "t1-new"},
		},
		{
			name: "WithoutResume returns the expired error",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t1-new")},
			},
			noResume:     true,
			wantItems:    []string{"a"},
			wantTokens:   []string{"", "t1"},
			wantErrCheck: apierrors.IsResourceExpired,
		},
		{
			name: "wrapped expired error resumes",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: fmt.Errorf("list failed: %w", paginationtest.ExpiredError("t1-new"))},
				{items: []string{"b"}},
			},
			wantItems:  []string{"a", "b"},
			wantTokens: []string{"", "t1", "t1-new"},
		},
		{
			name: "retry counter resets after a successful page",
			results: []listResult{
				{items: []string{"a"}, next: "t0"},
				{err: paginationtest.ExpiredError("t1")},
				{items: []string{"b"}, next: "t2"},
				{err: paginationtest.ExpiredError("t3")},
				{items: []string{"c"}},
			},
			wantItems:  []string{"a", "b", "c"},
			wantTokens: []string{"", "t0", "t1", "t2", "t3"},
		},
		{
			name: "non-expired error after a resume is returned",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t1-new")},
				{err: errBoom},
			},
			wantItems:  []string{"a"},
			wantTokens: []string{"", "t1", "t1-new"},
			wantErr:    errBoom,
		},
		{
			name: "expired error without a token is returned",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("")},
			},
			wantItems:    []string{"a"},
			wantTokens:   []string{"", "t1"},
			wantErrCheck: apierrors.IsResourceExpired,
		},
		{
			name: "expired error on the first page is returned",
			results: []listResult{
				{err: paginationtest.ExpiredError("t1")},
			},
			wantTokens:   []string{""},
			wantErrCheck: apierrors.IsResourceExpired,
		},
		{
			name: "expired error that repeats the same token is returned",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t1")},
			},
			wantItems:    []string{"a"},
			wantTokens:   []string{"", "t1"},
			wantErrCheck: apierrors.IsResourceExpired,
		},
		{
			name: "three expired errors in a row still resume",
			results: []listResult{
				{items: []string{"a"}, next: "t0"},
				{err: paginationtest.ExpiredError("t1")},
				{err: paginationtest.ExpiredError("t2")},
				{err: paginationtest.ExpiredError("t3")},
				{items: []string{"b"}},
			},
			wantItems:  []string{"a", "b"},
			wantTokens: []string{"", "t0", "t1", "t2", "t3"},
		},
		{
			name: "four expired errors in a row are limited",
			results: []listResult{
				{items: []string{"a"}, next: "t0"},
				{err: paginationtest.ExpiredError("t1")},
				{err: paginationtest.ExpiredError("t2")},
				{err: paginationtest.ExpiredError("t3")},
				{err: paginationtest.ExpiredError("t4")},
			},
			wantItems:    []string{"a"},
			wantTokens:   []string{"", "t0", "t1", "t2", "t3"},
			wantErrCheck: apierrors.IsResourceExpired,
		},
		{
			name: "label selector and unrelated options stay on every page",
			opts: metav1.ListOptions{LabelSelector: "app=x", FieldSelector: "metadata.name=y", ResourceVersion: "5", Limit: 7},
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{items: []string{"b"}},
			},
			wantItems:  []string{"a", "b"},
			wantTokens: []string{"", "t1"},
		},
		{
			name:           "NotFound on page 1 returns nil with ignoreNotFound",
			results:        []listResult{{err: notFound}},
			ignoreNotFound: true,
			wantTokens:     []string{""},
		},
		{
			name: "NotFound on page 2 is returned with ignoreNotFound",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: notFound},
			},
			ignoreNotFound: true,
			wantItems:      []string{"a"},
			wantTokens:     []string{"", "t1"},
			wantErrCheck:   apierrors.IsNotFound,
		},
		{
			name: "NotFound after a resume is returned with ignoreNotFound",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t2")},
				{err: notFound},
			},
			ignoreNotFound: true,
			wantItems:      []string{"a"},
			wantTokens:     []string{"", "t1", "t2"},
			wantErrCheck:   apierrors.IsNotFound,
		},
		{
			name:         "NotFound on page 1 is returned without ignoreNotFound",
			results:      []listResult{{err: notFound}},
			wantTokens:   []string{""},
			wantErrCheck: apierrors.IsNotFound,
		},
		{
			name:           "other list error is returned with ignoreNotFound",
			results:        []listResult{{err: errBoom}},
			ignoreNotFound: true,
			wantTokens:     []string{""},
			wantErr:        errBoom,
		},
		{
			name:           "NotFound from fn is returned with ignoreNotFound",
			results:        []listResult{{items: []string{"a"}}},
			fnErrOnPage:    1,
			fnErr:          notFound,
			ignoreNotFound: true,
			wantItems:      []string{"a"},
			wantTokens:     []string{""},
			wantErrCheck:   apierrors.IsNotFound,
		},
		{
			name: "same continue token on consecutive pages returns an error",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{items: []string{"b"}, next: "t1"},
			},
			wantItems:    []string{"a", "b"},
			wantTokens:   []string{"", "t1"},
			wantErrCheck: func(err error) bool { return strings.Contains(err.Error(), "same continue token") },
		},
		{
			name: "same continue token after a resume returns an error",
			results: []listResult{
				{items: []string{"a"}, next: "t1"},
				{err: paginationtest.ExpiredError("t2")},
				{items: []string{"b"}, next: "t2"},
			},
			wantItems:    []string{"a", "b"},
			wantTokens:   []string{"", "t1", "t2"},
			wantErrCheck: func(err error) bool { return strings.Contains(err.Error(), "same continue token") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotOpts []metav1.ListOptions
			list := func(opts metav1.ListOptions) (*corev1.ConfigMapList, error) {
				gotOpts = append(gotOpts, opts)
				require.LessOrEqual(t, len(gotOpts), len(tc.results), "unexpected extra list call")
				res := tc.results[len(gotOpts)-1]
				if res.err != nil {
					return nil, res.err
				}
				lst := &corev1.ConfigMapList{ListMeta: metav1.ListMeta{Continue: res.next}}
				for _, name := range res.items {
					lst.Items = append(lst.Items, corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name}})
				}
				return lst, nil
			}

			var gotItems []string
			var pages int
			var options []pagination.Option
			if tc.noResume {
				options = append(options, pagination.WithoutResume())
			}
			if tc.ignoreNotFound {
				options = append(options, pagination.WithIgnoreNotFound())
			}
			processConfigMaps := func(l *corev1.ConfigMapList) error {
				pages++
				for _, item := range l.Items {
					gotItems = append(gotItems, item.Name)
				}
				if pages == tc.fnErrOnPage {
					if tc.fnErr != nil {
						return tc.fnErr
					}
					return errBoom
				}
				return nil
			}
			err := pagination.EachPage(list, tc.opts, processConfigMaps, options...)

			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			case tc.wantErrCheck != nil:
				require.Error(t, err)
				assert.True(t, tc.wantErrCheck(err), "unexpected error: %v", err)
			default:
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantItems, gotItems)

			var gotTokens []string
			for _, o := range gotOpts {
				gotTokens = append(gotTokens, o.Continue)
				assert.Equal(t, pagination.PageSize, o.Limit, "every call must set the page size")
				assert.Empty(t, o.ResourceVersion, "resource version must not be sent")
				assert.Equal(t, tc.opts.LabelSelector, o.LabelSelector)
				assert.Equal(t, tc.opts.FieldSelector, o.FieldSelector)
			}
			assert.Equal(t, tc.wantTokens, gotTokens)
		})
	}
}
