package suseconnect

import (
	"github.com/rancher/rancher/pkg/scc/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchSccRegistrationCodeFrom(secrets controllerv1.SecretController, reference *corev1.SecretReference) string {
	sccContextLogger().Debugf("Fetching SCC Registration Code from secret %s/%s", reference.Namespace, reference.Name)
	regSecret, err := secrets.Get(reference.Namespace, reference.Name, metav1.GetOptions{})
	if err != nil {
		sccContextLogger().Warnf("Failed to get SCC Registration Code from secret %s/%s: %v", reference.Namespace, reference.Name, err)
		return ""
	}
	sccContextLogger().Debugf("Found secret %s/%s", reference.Namespace, reference.Name)

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		sccContextLogger().Warnf("registration secret `%v` does not contain expected data `%s`", reference, util.RegCodeSecretKey)
		return ""
	}

	return string(regCode)
}
