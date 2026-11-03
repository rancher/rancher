package cacerts

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"strings"

	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tls"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	tokenHash = "tokenByHash"
)

// hashToken returns the base64-encoded SHA-256 of a plaintext CRT token, the
// form agents present in the Authorization header.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// Handler indexes CRT token secrets by token hash and serves CA certs,
// signing responses with an HMAC so agents can verify authenticity.
func Handler(secretCache corecontrollers.SecretCache) http.HandlerFunc {
	secretCache.AddIndexer(tokenHash, func(obj *corev1.Secret) ([]string, error) {
		tokens := crt.SecretTokenIndexValues(obj)
		if len(tokens) == 0 {
			return nil, nil
		}
		hashes := make([]string, 0, len(tokens))
		for _, token := range tokens {
			hashes = append(hashes, hashToken(token))
		}
		return hashes, nil
	})
	return func(rw http.ResponseWriter, req *http.Request) {
		handler(secretCache, rw, req)
	}
}

func handler(secretCache corecontrollers.SecretCache, rw http.ResponseWriter, req *http.Request) {
	ca := settings.CACerts.Get()
	if v, ok := req.Context().Value(tls.InternalAPI).(bool); ok && v {
		ca = settings.InternalCACerts.Get()
	}

	rw.Header().Set("Content-Type", "text/plain")
	var bytes []byte
	if strings.TrimSpace(ca) != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		bytes = []byte(ca)
	}

	nonce := req.Header.Get("X-Cattle-Nonce")
	authorization := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")

	if authorization != "" && nonce != "" {
		secrets, err := secretCache.GetByIndex(tokenHash, authorization)
		if err == nil && len(secrets) > 0 {
			// During a rotation grace period the secret carries both the
			// current and previous token; key the HMAC with whichever token
			// matches the hash the agent authenticated with.
			var token string
			for _, t := range crt.SecretTokenIndexValues(secrets[0]) {
				if authorization == hashToken(t) {
					token = t
					break
				}
			}
			if token != "" {
				digest := hmac.New(sha512.New, []byte(token))
				digest.Write([]byte(nonce))
				digest.Write([]byte{0})
				digest.Write(bytes)
				digest.Write([]byte{0})
				hash := digest.Sum(nil)
				rw.Header().Set("X-Cattle-Hash", base64.StdEncoding.EncodeToString(hash))
			}
		}
	}

	if len(bytes) > 0 {
		_, _ = rw.Write([]byte(ca))
	}
}
