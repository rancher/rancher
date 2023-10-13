package requests

import (
	"testing"
	"time"

	jwtv4 "github.com/golang-jwt/jwt/v4"
)

func Test_isTokenExpired(t *testing.T) {
	tests := []struct {
		name           string
		expirationTime *jwtv4.NumericDate
		want           bool
	}{
		{
			name: "empty expiration",
			want: false,
		},
		{
			name:           "expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(-time.Hour)),
			want:           true,
		},
		{
			name:           "not expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(time.Hour)),
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTokenExpired(tt.expirationTime); got != tt.want {
				t.Errorf("isTokenExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}
