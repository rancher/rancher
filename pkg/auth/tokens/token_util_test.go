package tokens

import (
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVerifyToken(t *testing.T) {
	tokenName := "test-token"
	hashedTokenName := "hashed-test-token"

	tokenKey := "dddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	badTokenKey := "cccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	// SHA3 hash of tokenKey
	hashedTokenKey := "$3:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA"
	invalidHashToken := "$-1:111:111"
	unhashedToken := apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenName,
		},
		Token:     tokenKey,
		TTLMillis: 0,
	}
	hashedToken := apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: hashedTokenName,
			Annotations: map[string]string{
				TokenHashed: "true",
			},
		},
		Token:     hashedTokenKey,
		TTLMillis: 0,
	}
	invalidHashedToken := *hashedToken.DeepCopy()
	invalidHashedToken.Token = invalidHashToken

	tests := []struct {
		name      string
		token     *apiv3.Token
		tokenName string
		tokenKey  string

		wantResponseCode int
		wantErr          bool
	}{
		{
			name:             "valid non-hashed token",
			token:            &unhashedToken,
			tokenName:        tokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 200,
		},
		{
			name:             "valid hashed token",
			token:            &hashedToken,
			tokenName:        hashedTokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 200,
		},
		{
			name:             "valid hashed token, incorrect key",
			token:            &hashedToken,
			tokenName:        hashedTokenName,
			tokenKey:         badTokenKey,
			wantResponseCode: 422,
			wantErr:          true,
		},
		{
			name:             "wrong token",
			token:            &unhashedToken,
			tokenName:        hashedTokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 422,
			wantErr:          true,
		},
		{
			name:             "incorrect token key",
			token:            &unhashedToken,
			tokenName:        tokenName,
			tokenKey:         badTokenKey,
			wantResponseCode: 422,
			wantErr:          true,
		},
		{
			name:             "expired token",
			token:            expireToken(&unhashedToken),
			tokenName:        tokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 410,
			wantErr:          true,
		},
		{
			name:             "expired hashed token",
			token:            expireToken(&hashedToken),
			tokenName:        hashedTokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 410,
			wantErr:          true,
		},
		{
			name:             "nil token",
			token:            nil,
			tokenName:        tokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 422,
			wantErr:          true,
		},
		{
			name:             "unable to retrieve hasher",
			token:            &invalidHashedToken,
			tokenName:        hashedTokenName,
			tokenKey:         tokenKey,
			wantResponseCode: 500,
			wantErr:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responseCode, err := VerifyToken(test.token, test.tokenName, test.tokenKey)
			if test.wantErr {
				require.Error(t, err)
			}
			require.Equal(t, test.wantResponseCode, responseCode)
		})
	}
}

func TestConvertTokenKeyToHash(t *testing.T) {
	plaintextToken := "cccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	token := apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
		},
		Token:     plaintextToken,
		TTLMillis: 0,
	}
	tests := []struct {
		name                string
		tokenHashingEnabled bool
		token               *apiv3.Token

		wantError            bool
		wantHashedAnnotation bool
		wantHashedVal        bool
	}{
		{
			name:                "token hashing enabled",
			tokenHashingEnabled: true,
			token:               &token,

			wantHashedAnnotation: true,
			wantHashedVal:        true,
		},
		{
			name:                "token hashing disabled",
			tokenHashingEnabled: false,
			token:               &token,

			wantHashedAnnotation: false,
			wantHashedVal:        false,
		},
		{
			name:                "nil token",
			tokenHashingEnabled: false,
			token:               nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// token will be modified by the consuming function, deep copy to avoid changing the original token
			features.TokenHashing.Set(test.tokenHashingEnabled)
			var testToken *apiv3.Token
			if test.token != nil {
				testToken = test.token.DeepCopy()
			}
			err := ConvertTokenKeyToHash(testToken)
			if test.wantError {
				require.Error(t, err)
			}
			if test.wantHashedAnnotation {
				require.Contains(t, testToken.Annotations, TokenHashed)
				require.Equal(t, "true", testToken.Annotations[TokenHashed])
			} else {
				if test.token != nil {
					require.NotContains(t, testToken.Annotations, TokenHashed)
				}
			}
			if test.wantHashedVal {
				hasher, err := hashers.GetHasherForHash(testToken.Token)
				require.NoError(t, err)
				err = hasher.VerifyHash(testToken.Token, plaintextToken)
				require.NoError(t, err)
			} else {
				if test.token != nil {
					require.Equal(t, plaintextToken, testToken.Token)
				}
			}
		})
	}
}

func expireToken(token *apiv3.Token) *apiv3.Token {
	newToken := token.DeepCopy()
	newToken.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Second * 10))
	newToken.TTLMillis = 1
	return newToken
}

func TestIsIdleExpired(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()

	notExpired := metav1.NewTime(now.Add(-time.Duration(idleTimeout) * time.Minute).Add(time.Second))
	expired := metav1.NewTime(notExpired.Add(-2 * time.Second))
	zero := metav1.Time{}

	tests := []struct {
		desc               string
		activityLastSeenAt *metav1.Time
		expired            bool
	}{
		{
			desc:               "no activity set",
			activityLastSeenAt: &notExpired,
		},
		{
			desc:               "should not expire",
			activityLastSeenAt: &notExpired,
		},
		{
			desc:               "about to expire",
			activityLastSeenAt: &now,
		},
		{
			desc:               "activity not set",
			activityLastSeenAt: &notExpired,
		},
		{
			desc:               "zero value",
			activityLastSeenAt: &zero,
		},
		{
			desc:               "expired",
			activityLastSeenAt: &expired,
			expired:            true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			token := &apiv3.Token{}
			if tt.activityLastSeenAt != nil {
				token.ActivityLastSeenAt = tt.activityLastSeenAt
			}

			t.Log("activity", token.ActivityLastSeenAt.Format(time.RFC3339))
			t.Log("now", now.Format(time.RFC3339))

			assert.Equal(t, tt.expired, IsIdleExpired(token, now.Time))
		})
	}
}
