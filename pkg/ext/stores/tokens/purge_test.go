package tokens

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// makeTokenSecret builds a token-backing Secret with the given name, creation
// time, and TTL. A negative TTL encodes an infinite lifetime.
func makeTokenSecret(t *testing.T, name string, created time.Time, ttl time.Duration) corev1.Secret {
	t.Helper()
	principalBytes, err := json.Marshal(properPrincipal)
	require.NoError(t, err)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				UserIDLabel:     properUser,
				SecretKindLabel: SecretKindLabelValue,
			},
			UID:               types.UID("uid-" + name),
			CreationTimestamp: metav1.NewTime(created),
		},
		Data: map[string][]byte{
			FieldDescription:    []byte(""),
			FieldEnabled:        []byte("false"),
			FieldHash:           []byte("hash-" + name),
			FieldKind:           []byte(IsLogin),
			FieldLastUpdateTime: []byte("13:00:05"),
			FieldPrincipal:      principalBytes,
			FieldTTL:            []byte(strconv.FormatInt(ttl.Milliseconds(), 10)),
			FieldUID:            []byte("uid-" + name),
			FieldUserID:         []byte(properUser),
		},
	}
}

func TestSystemStoreDeleteExpired(t *testing.T) {
	t.Parallel()

	now := time.Now()
	longAgo := now.Add(-24 * time.Hour)
	recent := now.Add(-1 * time.Minute)

	const (
		shortTTL    = time.Minute
		longTTL     = 365 * 24 * time.Hour
		infiniteTTL = -1 * time.Millisecond // negative TTL encodes infinite
	)

	someErr := errors.New("some error")

	cases := []struct {
		name            string
		secrets         []corev1.Secret
		listErr         error
		deleteResults   map[string]error
		expectedDeletes []string
		wantCount       int
		wantErr         bool
	}{
		{
			name:    "empty list, no deletes",
			secrets: nil,
		},
		{
			name: "all fresh, no deletes",
			secrets: []corev1.Secret{
				makeTokenSecret(t, "fresh-1", recent, longTTL),
				makeTokenSecret(t, "fresh-2", recent, longTTL),
			},
		},
		{
			name: "mix expired + fresh, only expired deleted",
			secrets: []corev1.Secret{
				makeTokenSecret(t, "expired-1", longAgo, shortTTL),
				makeTokenSecret(t, "fresh-1", recent, longTTL),
				makeTokenSecret(t, "expired-2", longAgo, shortTTL),
			},
			expectedDeletes: []string{"expired-1", "expired-2"},
			wantCount:       2,
		},
		{
			name: "delete returns IsNotFound is tolerated",
			secrets: []corev1.Secret{
				makeTokenSecret(t, "expired-1", longAgo, shortTTL),
			},
			deleteResults: map[string]error{
				"expired-1": apierrors.NewNotFound(GVR.GroupResource(), "expired-1"),
			},
			expectedDeletes: []string{"expired-1"},
			wantCount:       1,
		},
		{
			name: "per-token error surfaced, loop continues, count reflects only successes",
			secrets: []corev1.Secret{
				makeTokenSecret(t, "expired-1", longAgo, shortTTL),
				makeTokenSecret(t, "expired-2", longAgo, shortTTL),
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
		{
			name: "infinite TTL never purged",
			secrets: []corev1.Secret{
				makeTokenSecret(t, "immortal", longAgo, infiniteTTL),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			secretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)

			if tt.listErr != nil {
				secretClient.EXPECT().
					List(TokenNamespace, gomock.Any()).
					Return(nil, tt.listErr).
					Times(1)
			} else {
				secretClient.EXPECT().
					List(TokenNamespace, gomock.Any()).
					Return(&corev1.SecretList{Items: tt.secrets}, nil).
					Times(1)
			}

			deletedNames := map[string]int{}
			for _, name := range tt.expectedDeletes {
				resErr := tt.deleteResults[name]
				secretClient.EXPECT().
					Delete(TokenNamespace, name, gomock.Any()).
					DoAndReturn(func(_, gotName string, _ *metav1.DeleteOptions) error {
						deletedNames[gotName]++
						return resErr
					}).
					Times(1)
			}

			store := &SystemStore{secretClient: secretClient}
			count, err := store.DeleteExpired()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCount, count)

			for _, name := range tt.expectedDeletes {
				assert.Equal(t, 1, deletedNames[name], "expected exactly one Delete for %s", name)
			}
			assert.Len(t, deletedNames, len(tt.expectedDeletes),
				"unexpected extra Delete calls: %v", deletedNames)
		})
	}
}
