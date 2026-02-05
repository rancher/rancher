package namespace

import (
	"github.com/rancher/rancher/tests/v2prov/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func random(clients *clients.Clients, annotations map[string]string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "namespace-",
		},
	}
	if len(annotations) > 0 {
		ns.Annotations = map[string]string{}
		for k, v := range annotations {
			ns.Annotations[k] = v
		}
	}
	ns, err := clients.Core.Namespace().Create(ns)
	if err != nil {
		return nil, err
	}
	clients.OnClose(func() {
		clients.Core.Namespace().Delete(ns.Name, nil)
	})

	return ns, nil
}

func Random(clients *clients.Clients) (*corev1.Namespace, error) {
	return random(clients, nil)
}

func RandomWithAnnotations(clients *clients.Clients, annotations map[string]string) (*corev1.Namespace, error) {
	return random(clients, annotations)
}
