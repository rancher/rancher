package suseconnect

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchSccRegistrationCodeFrom(secrets controllerv1.SecretController, reference *corev1.SecretReference) string {
	logrus.Debugf("Fetching SCC Registration Code from secret %s/%s", reference.Namespace, reference.Name)
	regSecret, err := secrets.Get(reference.Namespace, reference.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("Failed to get SCC Registration Code from secret %s/%s: %v", reference.Namespace, reference.Name, err)
		return ""
	}
	logrus.Debugf("Found secret %s/%s", reference.Namespace, reference.Name)

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		logrus.Warnf("registration secret `%v` does not contain expected data `%s`", reference, util.RegCodeSecretKey)
		return ""
	}

	return string(regCode)
}

func StoreSccOfflineRegistration(secrets controllerv1.SecretController, request *v1.Registration, offlineBlob []byte) (*corev1.Secret, error) {
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.RancherSCCOfflineRequestSecretName,
			Namespace: "cattle-system",
			Annotations: map[string]string{
				"owner": request.Name,
			},
		},
		StringData: map[string]string{
			util.RegCertSecretKey: string(offlineBlob),
		},
	}
	created, err := secrets.Create(newSecret)
	if err != nil {
		return nil, err
	}

	// TODO: update Request status to point to creds secret
	return created, nil
}
