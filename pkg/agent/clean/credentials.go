package clean

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
)

// UnusedCattleCredentials removes all of the cattle-credentials that aren't being used by the current pod.
func UnusedCattleCredentials() error {
	client, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k8s, err := kubernetes.NewForConfig(client)
	if err != nil {
		return err
	}

	s := k8s.CoreV1().Secrets("cattle-system")

	secrets, err := s.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, sec := range secrets.Items {
		if !strings.HasPrefix(sec.Name, "cattle-credentials-") {
			continue
		}

		if sec.Name != os.Getenv("CATTLE_CREDENTIAL_NAME") {
			err = s.Delete(context.Background(), sec.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
