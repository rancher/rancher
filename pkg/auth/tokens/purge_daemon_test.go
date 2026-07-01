package tokens

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rancher/norman/clientbase"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	testShortTTLMillis = int64(60 * 1000)              // 60s
	testLongTTLMillis  = int64(365 * 24 * 3600 * 1000) // 1y
)

func newV3Token(name string, created time.Time, ttlMillis int64) *v3.Token {
	return &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
		TTLMillis: ttlMillis,
	}
}

func newSamlToken(name string, created time.Time) *v3.SamlToken {
	return &v3.SamlToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
	}
}

// clientBaseNotFound returns an error that clientbase.IsNotFound recognises.
func clientBaseNotFound() error {
	return &clientbase.APIError{StatusCode: http.StatusNotFound, Msg: "not found"}
}

func TestPurgerDeleteExpiredV3Tokens(t *testing.T) {
	t.Parallel()

	now := time.Now()
	longAgo := now.Add(-24 * time.Hour)
	recent := now.Add(-1 * time.Minute)

	someErr := errors.New("some error")

	cases := []struct {
		name            string
		tokens          []*v3.Token
		listErr         error
		deleteResults   map[string]error
		expectedDeletes []string
		wantCount       int
		wantErr         bool
	}{
		{
			name: "mix expired + fresh, only expired deleted",
			tokens: []*v3.Token{
				newV3Token("expired-1", longAgo, testShortTTLMillis),
				newV3Token("fresh-1", recent, testLongTTLMillis),
				newV3Token("expired-2", longAgo, testShortTTLMillis),
			},
			expectedDeletes: []string{"expired-1", "expired-2"},
			wantCount:       2,
		},
		{
			name: "delete returns not found is tolerated",
			tokens: []*v3.Token{
				newV3Token("expired-1", longAgo, testShortTTLMillis),
			},
			deleteResults: map[string]error{
				"expired-1": clientBaseNotFound(),
			},
			expectedDeletes: []string{"expired-1"},
			wantCount:       1,
		},
		{
			name: "per-token error surfaced, loop continues",
			tokens: []*v3.Token{
				newV3Token("expired-1", longAgo, testShortTTLMillis),
				newV3Token("expired-2", longAgo, testShortTTLMillis),
			},
			deleteResults: map[string]error{
				"expired-1": someErr,
			},
			expectedDeletes: []string{"expired-1", "expired-2"},
			wantCount:       1,
			wantErr:         true,
		},
		{
			name:    "list error surfaced, no deletes",
			listErr: someErr,
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deleted := map[string]int{}
			lister := &fakes.TokenListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Token, error) {
					if tt.listErr != nil {
						return nil, tt.listErr
					}
					return tt.tokens, nil
				},
			}
			iface := &fakes.TokenInterfaceMock{
				DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
					deleted[name]++
					return tt.deleteResults[name]
				},
			}

			p := &purger{tokenLister: lister, tokens: iface}
			count, err := p.deleteExpiredV3Tokens()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCount, count)
			for _, name := range tt.expectedDeletes {
				assert.Equal(t, 1, deleted[name], "expected exactly one Delete for %s", name)
			}
			assert.Len(t, deleted, len(tt.expectedDeletes),
				"unexpected extra Delete calls: %v", deleted)
		})
	}
}

func TestPurgerDeleteExpiredSamlTokens(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stale := now.Add(-30 * time.Minute) // past the 15-minute grace period
	fresh := now.Add(-1 * time.Minute)  // still within grace period

	someErr := errors.New("some error")

	cases := []struct {
		name            string
		tokens          []*v3.SamlToken
		listErr         error
		deleteResults   map[string]error
		expectedDeletes []string
		wantCount       int
		wantErr         bool
	}{
		{
			name: "mix stale + fresh, only stale deleted",
			tokens: []*v3.SamlToken{
				newSamlToken("stale-1", stale),
				newSamlToken("fresh-1", fresh),
				newSamlToken("stale-2", stale),
			},
			expectedDeletes: []string{"stale-1", "stale-2"},
			wantCount:       2,
		},
		{
			name: "delete returns not found is tolerated",
			tokens: []*v3.SamlToken{
				newSamlToken("stale-1", stale),
			},
			deleteResults: map[string]error{
				"stale-1": clientBaseNotFound(),
			},
			expectedDeletes: []string{"stale-1"},
			wantCount:       1,
		},
		{
			name: "per-token error surfaced, loop continues",
			tokens: []*v3.SamlToken{
				newSamlToken("stale-1", stale),
				newSamlToken("stale-2", stale),
			},
			deleteResults: map[string]error{
				"stale-1": someErr,
			},
			expectedDeletes: []string{"stale-1", "stale-2"},
			wantCount:       1,
			wantErr:         true,
		},
		{
			name:    "list error surfaced, no deletes",
			listErr: someErr,
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deleted := map[string]int{}
			lister := &fakes.SamlTokenListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*v3.SamlToken, error) {
					if tt.listErr != nil {
						return nil, tt.listErr
					}
					return tt.tokens, nil
				},
			}
			iface := &fakes.SamlTokenInterfaceMock{
				DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
					deleted[name]++
					return tt.deleteResults[name]
				},
			}

			p := &purger{samlTokensLister: lister, samlTokens: iface}
			count, err := p.deleteExpiredSamlTokens()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCount, count)
			for _, name := range tt.expectedDeletes {
				assert.Equal(t, 1, deleted[name], "expected exactly one Delete for %s", name)
			}
			assert.Len(t, deleted, len(tt.expectedDeletes),
				"unexpected extra Delete calls: %v", deleted)
		})
	}
}

func TestPurgerPurgeInvokesAll(t *testing.T) {
	t.Parallel()

	var (
		v3Called   int
		samlCalled int
		extCalled  int
	)

	p := &purger{
		tokenLister: &fakes.TokenListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Token, error) {
				v3Called++
				return nil, nil
			},
		},
		tokens: &fakes.TokenInterfaceMock{},
		samlTokensLister: &fakes.SamlTokenListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.SamlToken, error) {
				samlCalled++
				return nil, nil
			},
		},
		samlTokens: &fakes.SamlTokenInterfaceMock{},
		extTokenPurger: extTokenPurgerFunc(func() (int, error) {
			extCalled++
			return 0, nil
		}),
	}

	p.purge()

	assert.Equal(t, 1, extCalled, "expected extTokenPurger.DeleteExpired to be called once")
	assert.Equal(t, 1, v3Called, "expected v3 token lister to be called once")
	assert.Equal(t, 1, samlCalled, "expected saml token lister to be called once")
}

// extTokenPurgerFunc lets a plain function satisfy the extTokenPurger interface.
type extTokenPurgerFunc func() (int, error)

func (f extTokenPurgerFunc) DeleteExpired() (int, error) { return f() }
