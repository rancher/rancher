package configserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var errRetrievalInvalidated = errors.New("secret retrieval invalidated")

func (r *RKE2ConfigServer) findMachineByProvisioningSA(req *http.Request) (*corev1.ObjectReference, error) {
	token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	secrets, err := r.secretsCache.GetByIndex(tokenIndex, token)
	if err != nil || len(secrets) == 0 {
		return nil, err
	}

	sa, err := r.serviceAccountsCache.Get(secrets[0].Namespace, secrets[0].Annotations[corev1.ServiceAccountNameKey])
	if err != nil {
		return nil, err
	}

	if sa.Labels[capr.RoleLabel] != capr.RoleBootstrap || string(sa.UID) != secrets[0].Annotations[corev1.ServiceAccountUIDKey] {
		return nil, err
	}

	if foundParent, err := capr.IsOwnedByMachine(r.bootstrapCache, sa.Labels[capr.MachineNameLabel], sa); err != nil || !foundParent {
		return nil, err
	}

	if secrets[0].Annotations[capr.InvalidatedBootstrapTokenAnnotation] == "true" {
		return nil, errRetrievalInvalidated
	}

	secret := secrets[0].DeepCopy()
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	secret.Annotations[capr.BootstrapTokenLastAccessTimeAnnotation] = time.Now().UTC().Format(time.RFC3339)

	_, err = r.secrets.Update(secret)
	if err != nil {
		return nil, err
	}

	return &corev1.ObjectReference{Kind: "Machine", APIVersion: capi.GroupVersion.String(), Namespace: sa.Namespace, Name: sa.Labels[capr.MachineNameLabel]}, nil
}
