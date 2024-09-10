package hashers

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicSha3Hash(t *testing.T) {
	secretKey := "hello world"
	hasher := Sha3Hasher{}
	hash, err := hasher.CreateHash(secretKey)
	require.Nil(t, err)
	require.NotNil(t, hash)
	splitHash := strings.Split(hash, ":")
	require.Len(t, splitHash, 4)
	require.Equal(t, strconv.Itoa(int(SHA3Version)), splitHash[0][1:])
	require.Equal(t, splitHash[1], "1")
	// Now check it
	require.Nil(t, hasher.VerifyHash(hash, secretKey))
	require.NotNil(t, hasher.VerifyHash(hash, "incorrect"))
}

func TestSha3LongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hasher := Sha3Hasher{}
	hash, err := hasher.CreateHash(secretKey)
	require.Nil(t, err)
	require.NotNil(t, hash)
	splitHash := strings.Split(hash, ":")
	require.Len(t, splitHash, 4)
	require.Equal(t, strconv.Itoa(int(SHA3Version)), splitHash[0][1:])
	require.Equal(t, splitHash[1], "1")
	// Now check it
	require.Nil(t, hasher.VerifyHash(hash, secretKey))
	require.NotNil(t, hasher.VerifyHash(hash, "incorrect"))
}

func TestSHA3VerifyHash(t *testing.T) {
	tests := []struct {
		name      string
		hash      string
		secretKey string
		wantError bool
	}{
		{
			name:      "valid hash",
			hash:      "$3:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: false,
		},
		{
			name:      "invalid hash format",
			hash:      "$3:1:uFrxm43ggfw",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "invalid hash version",
			hash:      "$2:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "invalid sha variation",
			hash:      "$3:2:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "invalid secret key",
			hash:      "$3:2:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "wrong",
			wantError: true,
		},
		{
			name:      "missing $ prefix",
			hash:      "3:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "non-int hash version",
			hash:      "$A:1:uFrxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "non base64 character in salt",
			hash:      "$3:B:#Frxm43ggfw:zsN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
		{
			name:      "non base64 character in hash",
			hash:      "$3:B:uFrxm43ggfw:#sN1zEFC7SvABTdR58o7yjIqfrI4cQ/HSYz3jBwwVnx5X+/ph4etGDIU9dvIYuy1IvnYUVe6a/Ar95xE+gfjhA",
			secretKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hasher := Sha3Hasher{}
			err := hasher.VerifyHash(test.hash, test.secretKey)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
