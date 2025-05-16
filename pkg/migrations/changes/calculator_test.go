package changes

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/rancher/rancher/pkg/migrations/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestCreateMergePatchChange(t *testing.T) {
	oldObj := createService()
	newObj := createService(func(s *corev1.Service) {
		s.Spec.ClusterIPs = []string{
			"10.43.25.18",
			"10.43.26.16",
		}
		s.ObjectMeta.Labels = map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
			"example.com/testing":          "test",
		}
	})
	change, err := CreateMergePatchChange(oldObj, newObj, test.NewFakeMapper())
	if err != nil {
		t.Fatal(err)
	}

	want := &PatchChange{
		ResourceRef: ResourceReference{
			ObjectRef: types.NamespacedName{
				Name:      oldObj.Name,
				Namespace: oldObj.Namespace,
			},
			Resource: "services",
			Version:  "v1",
		},
		MergePatch: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"example.com/testing": "test",
				},
			},
			"spec": map[string]any{
				"clusterIPs": []any{
					"10.43.25.18",
					"10.43.26.16",
				},
			},
		},
		Type: MergePatchJSON,
	}

	if diff := cmp.Diff(want, change); diff != "" {
		t.Fatalf("unexpected changes: diff -want +got\n%s", diff)
	}
}

func createService(opts ...func(*corev1.Service)) *corev1.Service {
	s := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "gitjob",
			Namespace:         "cattle-fleet-system",
			ResourceVersion:   "3632",
			UID:               types.UID("606df6ae-3114-4d1a-9960-0583693be4eb"),
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "fleet",
				"meta.helm.sh/release-namespace": "cattle-fleet-system",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.43.25.18",
			ClusterIPs: []string{
				"10.43.25.18",
			},
			InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
			IPFamilies: []corev1.IPFamily{
				corev1.IPv4Protocol,
			},
			IPFamilyPolicy: ptr.To(corev1.IPFamilyPolicySingleStack),
			Selector: map[string]string{
				"app": "gitjob",
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http-80",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: int32(8080)},
				},
			},
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
