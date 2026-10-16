package paginationtest

import (
	"errors"
	"testing"

	"github.com/rancher/rancher/pkg/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPagerServe(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d", "e"}

	tests := []struct {
		name       string
		perPage    int
		failPage   int
		extra      map[string]int
		continues  []string
		want       [][]string
		wantTokens []string
		wantErrs   []bool
	}{
		{
			name:       "pages of two items",
			perPage:    2,
			continues:  []string{"", "token-page-1", "token-page-2"},
			want:       [][]string{{"a", "b"}, {"c", "d"}, {"e"}},
			wantTokens: []string{"token-page-1", "token-page-2", ""},
			wantErrs:   []bool{false, false, false},
		},
		{
			name:       "zero page size serves one page",
			perPage:    0,
			continues:  []string{""},
			want:       [][]string{{"a", "b", "c", "d", "e"}},
			wantTokens: []string{""},
			wantErrs:   []bool{false},
		},
		{
			name:       "page size equal to the item count serves one page",
			perPage:    5,
			continues:  []string{""},
			want:       [][]string{{"a", "b", "c", "d", "e"}},
			wantTokens: []string{""},
			wantErrs:   []bool{false},
		},
		{
			name:       "failing second page",
			perPage:    2,
			failPage:   2,
			continues:  []string{"", "token-page-1"},
			want:       [][]string{{"a", "b"}, nil},
			wantTokens: []string{"token-page-1", ""},
			wantErrs:   []bool{false, true},
		},
		{
			name:       "extra token resumes at a page",
			perPage:    2,
			extra:      map[string]int{"new-token": 2},
			continues:  []string{"new-token"},
			want:       [][]string{{"e"}},
			wantTokens: []string{""},
			wantErrs:   []bool{false},
		},
		{
			name:       "unknown token is an error",
			perPage:    2,
			continues:  []string{"bogus"},
			want:       [][]string{nil},
			wantTokens: []string{""},
			wantErrs:   []bool{true},
		},
		{
			name:       "negative page token is an error",
			perPage:    2,
			continues:  []string{"token-page--1"},
			want:       [][]string{nil},
			wantTokens: []string{""},
			wantErrs:   []bool{true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := NewPager(items, tc.perPage).FailOnPage(tc.failPage, nil)
			p.ExtraTokens = tc.extra
			for i, token := range tc.continues {
				got, next, err := p.Serve(metav1.ListOptions{Continue: token, Limit: pagination.PageSize})
				assert.Equal(t, tc.wantErrs[i], err != nil, "call %d error", i)
				assert.Equal(t, tc.want[i], got, "call %d items", i)
				assert.Equal(t, tc.wantTokens[i], next, "call %d next token", i)
			}
			assert.Equal(t, tc.continues, p.Tokens())
			p.AssertLimit(t)
		})
	}
}

func TestPagerSnapshotsItems(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c"}
	p := NewPager(items, 2)
	items[0] = "changed"

	got, _, err := p.Serve(metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestPagerStopsAfterTooManyCalls(t *testing.T) {
	t.Parallel()

	p := NewPager([]string{"a", "b"}, 1)
	var err error
	for range maxCalls + 1 {
		_, _, err = p.Serve(metav1.ListOptions{})
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "continue token is probably ignored")
}

func TestFailOnPageUsesTheGivenError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	p := NewPager([]string{"a", "b"}, 1).FailOnPage(1, boom)
	_, _, err := p.Serve(metav1.ListOptions{})
	assert.ErrorIs(t, err, boom)

	p = NewPager([]string{"a", "b"}, 1).FailOnPage(1, nil)
	_, _, err = p.Serve(metav1.ListOptions{})
	assert.ErrorIs(t, err, ErrListFailed)
}

func TestExpiredError(t *testing.T) {
	t.Parallel()

	err := ExpiredError("resume-here")
	require.True(t, apierrors.IsResourceExpired(err))

	var status apierrors.APIStatus
	require.True(t, errors.As(err, &status))
	assert.Equal(t, "resume-here", status.Status().ListMeta.Continue)
}

func TestPagesPagerServe(t *testing.T) {
	t.Parallel()

	p := NewPagesPager([]string{"a"}, []string{"b", "c", "d"}, []string{}, []string{"e"})

	tests := []struct {
		token     string
		want      []string
		wantNext  string
		wantError bool
	}{
		{token: "", want: []string{"a"}, wantNext: "token-page-1"},
		{token: "token-page-1", want: []string{"b", "c", "d"}, wantNext: "token-page-2"},
		{token: "token-page-2", want: []string{}, wantNext: "token-page-3"},
		{token: "token-page-3", want: []string{"e"}, wantNext: ""},
		{token: "token-page-4", wantError: true},
	}
	for _, tc := range tests {
		got, next, err := p.Serve(metav1.ListOptions{Continue: tc.token})
		if tc.wantError {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "token %q", tc.token)
		assert.Equal(t, tc.wantNext, next, "token %q", tc.token)
	}
}
