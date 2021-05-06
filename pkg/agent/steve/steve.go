package steve

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/pkg/apply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func Run(ctx context.Context) error {
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	apply, err := apply.NewForConfig(c)
	if err != nil {
		return err
	}

	token, url, err := cluster.TokenAndURL()
	if err != nil {
		return err
	}

	data := map[string][]byte{
		"CATTLE_SERVER": []byte(url),
		"CATTLE_TOKEN":  []byte(token),
		"url":           []byte(url + "/v3/connect"),
		"token":         []byte("steve-cluster-" + token),
	}

	ca, err := ioutil.ReadFile("/etc/kubernetes/ssl/certs/serverca")
	if os.IsNotExist(err) {
	} else if err != nil {
		return err
	} else {
		data["ca.crt"] = ca
	}

	return apply.
		WithDynamicLookup().
		WithSetID("rancher-steve-aggregation").
		WithListerNamespace(namespace.System).
		ApplyObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.System,
				Name:      "steve-aggregation",
			},
			Data: data,
		})
}
