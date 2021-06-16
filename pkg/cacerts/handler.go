package cacerts

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tls"
)

var (
	tokenHash = "tokenByHash"
)

func Handler(clusterRegistrationToken v3.ClusterRegistrationTokenCache) http.HandlerFunc {
	clusterRegistrationToken.AddIndexer(tokenHash, func(obj *apimgmtv3.ClusterRegistrationToken) ([]string, error) {
		if obj.Status.Token == "" {
			return nil, nil
		}
		hash := sha256.Sum256([]byte(obj.Status.Token))
		return []string{base64.StdEncoding.EncodeToString(hash[:])}, nil
	})
	return func(rw http.ResponseWriter, req *http.Request) {
		handler(clusterRegistrationToken, rw, req)
	}
}

func handler(clusterRegistrationToken v3.ClusterRegistrationTokenCache, rw http.ResponseWriter, req *http.Request) {
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
		crt, err := clusterRegistrationToken.GetByIndex(tokenHash, authorization)
		if err == nil && len(crt) >= 0 {
			digest := hmac.New(sha512.New, []byte(crt[0].Status.Token))
			digest.Write([]byte(nonce))
			digest.Write([]byte{0})
			digest.Write(bytes)
			digest.Write([]byte{0})
			hash := digest.Sum(nil)
			rw.Header().Set("X-Cattle-Hash", base64.StdEncoding.EncodeToString(hash))
		}
	}

	if len(bytes) > 0 {
		_, _ = rw.Write([]byte(ca))
	}
}
