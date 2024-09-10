package configserver

import (
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	api "k8s.io/api/core/v1"
)

func (r *RKE2ConfigServer) findMachineByProvisioningSA(req *http.Request) (string, string, error) {
	token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	secrets, err := r.secretsCache.GetByIndex(tokenIndex, token)
	if err != nil || len(secrets) == 0 {
		return "", "", err
	}

	sa, err := r.serviceAccountsCache.Get(secrets[0].Namespace, secrets[0].Annotations[api.ServiceAccountNameKey])
	if err != nil {
		return "", "", err
	}

	if sa.Labels[capr.RoleLabel] != capr.RoleBootstrap || string(sa.UID) != secrets[0].Annotations[api.ServiceAccountUIDKey] {
		return "", "", err
	}

	if foundParent, err := capr.IsOwnedByMachine(r.bootstrapCache, sa.Labels[capr.MachineNameLabel], sa); err != nil || !foundParent {
		return "", "", err
	}

	return sa.Namespace, sa.Labels[capr.MachineNameLabel], nil
}
