package aggregation

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	saTokenHashIndex = "saTokenHashIndex"
)

func New(wrangler *wrangler.Context) remotedialer.Authorizer {
	a := &APIServiceAuthorizer{
		serviceAccount: wrangler.Core.ServiceAccount().Cache(),
		apiServices:    wrangler.Mgmt.APIService().Cache(),
		secrets:        wrangler.Core.Secret().Cache(),
	}

	wrangler.Core.Secret().Cache().AddIndexer(saTokenHashIndex, func(obj *corev1.Secret) ([]string, error) {
		return []string{
			hashSecret(obj),
		}, nil
	})
	return a.Authorize
}

type APIServiceAuthorizer struct {
	secrets        corecontrollers.SecretCache
	serviceAccount corecontrollers.ServiceAccountCache
	apiServices    mgmtcontrollers.APIServiceCache
}

func hashSecret(sa *corev1.Secret) string {
	if sa.Type == corev1.SecretTypeServiceAccountToken {
		hash := sha256.Sum256(sa.Data[corev1.ServiceAccountTokenKey])
		return base64.StdEncoding.EncodeToString(hash[:])
	}
	return ""
}

func (a *APIServiceAuthorizer) Authorize(req *http.Request) (clientKey string, authed bool, err error) {
	token := strings.TrimSpace(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		return "", false, nil
	}

	secrets, err := a.secrets.GetByIndex(saTokenHashIndex, token)
	if apierror.IsNotFound(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	for _, secret := range secrets {
		if secret.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		saName := secret.Annotations[corev1.ServiceAccountNameKey]
		saUID := secret.Annotations[corev1.ServiceAccountUIDKey]
		if saName == "" || saUID == "" {
			continue
		}

		sa, err := a.serviceAccount.Get(secret.Namespace, saName)
		if apierror.IsNotFound(err) {
			continue
		} else if err != nil {
			return "", false, err
		}

		if string(sa.UID) != saUID {
			continue
		}

		for _, owner := range sa.OwnerReferences {
			if owner.Kind != "APIService" {
				continue
			}

			apiService, err := a.apiServices.Get(owner.Name)
			if apierror.IsNotFound(err) {
				continue
			} else if err != nil {
				return "", false, err
			}

			if apiService.Status.ServiceAccountNamespace == sa.Namespace &&
				apiService.Status.ServiceAccountName == sa.Name {
				return keyFromUUID(string(apiService.UID)), true, nil
			}
		}
	}

	return "", false, nil
}
