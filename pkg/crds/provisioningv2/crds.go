package provisioningv2

import (
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func List() (result []crd.CRD) {
	result = append(result, provisioning()...)
	if features.RKE2.Enabled() {
		result = append(result, rke2()...)
	}
	return
}

func provisioning() []crd.CRD {
	return []crd.CRD{
		newRancherCRD(&v1.Cluster{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"auth.cattle.io/cluster-indexed": "true",
			}
			return c.
				WithColumn("Ready", ".status.ready").
				WithColumn("Kubeconfig", ".status.clientSecretName")
		}),
	}
}

func clusterIndexed(c crd.CRD) crd.CRD {
	newLabels := map[string]string{}
	for k, v := range c.Labels {
		newLabels[k] = v
	}
	newLabels["auth.cattle.io/cluster-indexed"] = "true"
	c.Labels = newLabels
	return c
}

func rke2() []crd.CRD {
	return []crd.CRD{
		newRancherCRD(&v1.Cluster{}, func(c crd.CRD) crd.CRD {
			return clusterIndexed(c).
				WithColumn("Ready", ".status.ready").
				WithColumn("Kubeconfig", ".status.clientSecretName")
		}),
		newRKECRD(&rkev1.RKECluster{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.RKEControlPlane{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.RKEBootstrap{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.RKEBootstrapTemplate{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.RKEControlPlane{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.CustomMachine{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
		newRKECRD(&rkev1.ETCDSnapshot{}, func(c crd.CRD) crd.CRD {
			c.Labels = map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1",
				"cluster.x-k8s.io/v1beta2": "v1",
			}
			return clusterIndexed(c)
		}),
	}
}

// Webhooks returns empty as the provisioning CAPI webhooks have been removed.
func Webhooks() []runtime.Object {
	return nil
}

func newRKECRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "rke.cattle.io",
			Version: "v1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

func newRancherCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "provisioning.cattle.io",
			Version: "v1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}
