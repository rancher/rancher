package cacerts

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tls"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

var (
	tokenHash = "tokenByHash"
)

func Handler(clusterRegistrationToken v3.ClusterRegistrationTokenCache, secretCache corecontrollers.SecretCache) http.HandlerFunc {
	clusterRegistrationToken.AddIndexer(tokenHash, func(obj *apimgmtv3.ClusterRegistrationToken) ([]string, error) {
		current, previous, err := crt.GetTokensFromSecret(secretCache, obj)
		if err != nil {
			logrus.Warnf("failed to resolve CRT token for %s/%s: %v", obj.Namespace, obj.Name, err)
			return nil, nil
		}
		if current == "" {
			return nil, nil
		}
		currentHash := sha256.Sum256([]byte(current))
		hashes := []string{base64.StdEncoding.EncodeToString(currentHash[:])}
		if previous != "" {
			prevHash := sha256.Sum256([]byte(previous))
			hashes = append(hashes, base64.StdEncoding.EncodeToString(prevHash[:]))
		}
		return hashes, nil
	})
	return func(rw http.ResponseWriter, req *http.Request) {
		handler(clusterRegistrationToken, secretCache, rw, req)
	}
}

func handler(clusterRegistrationToken v3.ClusterRegistrationTokenCache, secretCache corecontrollers.SecretCache, rw http.ResponseWriter, req *http.Request) {
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
		crts, err := clusterRegistrationToken.GetByIndex(tokenHash, authorization)
		if err == nil && len(crts) > 0 {
			current, previous, err := crt.GetTokensFromSecret(secretCache, crts[0])
			if err != nil {
				logrus.Warnf("failed to resolve CRT token for HMAC: %v", err)
			} else {
				// Use whichever token the agent authenticated with.
				// During grace period both current and previous are indexed;
				// the HMAC must be keyed with the token that matches the
				// authorization hash the agent sent.
				token := current
				if previous != "" {
					prevHash := sha256.Sum256([]byte(previous))
					if authorization == base64.StdEncoding.EncodeToString(prevHash[:]) {
						token = previous
					}
				}
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
