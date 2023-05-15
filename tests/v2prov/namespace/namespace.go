package namespace

import (
	"github.com/rancher/rancher/tests/v2prov/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Random(clients *clients.Clients) (*corev1.Namespace, error) {
	ns, err := clients.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-ns-",
		},
	})
	if err != nil {
		return nil, err
	}
	clients.OnClose(func() {
		clients.Core.Namespace().Delete(ns.Name, nil)
	})

	return ns, nil
}
