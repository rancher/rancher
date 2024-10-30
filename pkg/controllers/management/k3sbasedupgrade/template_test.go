package k3sbasedupgrade

import (
	"testing"

	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_parseVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			"base",
			"v1.17.3+k3s1",
			"v1.17.3-k3s1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVersion(tt.version); got != tt.want {
				t.Errorf("parseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateMasterPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		version     string
		concurrency int
		drain       bool
		image       string
		expected    planv1.Plan
	}{
		{
			name:        "simple plan",
			version:     "test-version",
			concurrency: 1,
			drain:       false,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "master-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "drain plan",
			version:     "test-version",
			concurrency: 1,
			drain:       true,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "master-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Drain: &planv1.DrainSpec{
						Force: true,
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "concurrent plan",
			version:     "test-version",
			concurrency: 3,
			drain:       false,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "master-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        3,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateMasterPlan(tt.version, tt.concurrency, tt.drain, tt.image, "master-plan"))
		})
	}
}

func TestGenerateWorkerPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		version     string
		concurrency int
		drain       bool
		image       string
		expected    planv1.Plan
	}{
		{
			name:        "simple plan",
			version:     "test-version",
			concurrency: 1,
			drain:       false,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "drain plan",
			version:     "test-version",
			concurrency: 1,
			drain:       true,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Drain: &planv1.DrainSpec{
						Force: true,
					},
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "concurrent plan",
			version:     "test-version",
			concurrency: 3,
			drain:       false,
			image:       "test-image",
			expected: planv1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "upgrade.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-plan",
					Namespace: "cattle-system",
					Labels: map[string]string{
						"rancher-managed": "true",
					},
				},
				Spec: planv1.PlanSpec{
					Concurrency:        3,
					ServiceAccountName: "system-upgrade-controller",
					Cordon:             true,
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateWorkerPlan(tt.version, tt.concurrency, tt.drain, tt.image, "worker-plan", "master-plan"))
		})
	}
}

func TestConfigureMasterPlan(t *testing.T) {
	t.Parallel()
	masterPlan := planv1.Plan{
		Spec: planv1.PlanSpec{
			Upgrade: &planv1.ContainerSpec{
				Image: "test-image",
			},
		},
	}
	tests := []struct {
		name        string
		version     string
		concurrency int
		drain       bool
		expected    planv1.Plan
	}{
		{
			name:        "simple plan",
			version:     "test-version",
			concurrency: 1,
			drain:       false,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "drain plan",
			version:     "test-version",
			concurrency: 1,
			drain:       true,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Drain: &planv1.DrainSpec{
						Force: true,
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "concurrent plan",
			version:     "test-version",
			concurrency: 3,
			drain:       false,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        3,
					ServiceAccountName: "system-upgrade-controller",
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"true"},
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			masterPlan := masterPlan
			assert.Equal(t, tt.expected, configureMasterPlan(masterPlan, tt.version, tt.concurrency, tt.drain, "master-plan"))
		})
	}
}

func TestConfigureWorkerPlan(t *testing.T) {
	t.Parallel()
	workerPlan := planv1.Plan{
		Spec: planv1.PlanSpec{
			Upgrade: &planv1.ContainerSpec{
				Image: "test-image",
			},
		},
	}
	tests := []struct {
		name        string
		version     string
		concurrency int
		drain       bool
		expected    planv1.Plan
	}{
		{
			name:        "simple plan",
			version:     "test-version",
			concurrency: 1,
			drain:       false,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "drain plan",
			version:     "test-version",
			concurrency: 1,
			drain:       true,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        1,
					ServiceAccountName: "system-upgrade-controller",
					Drain: &planv1.DrainSpec{
						Force: true,
					},
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
		{
			name:        "concurrent plan",
			version:     "test-version",
			concurrency: 3,
			drain:       false,
			expected: planv1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-plan",
				},
				Spec: planv1.PlanSpec{
					Concurrency:        3,
					ServiceAccountName: "system-upgrade-controller",
					Prepare: &planv1.ContainerSpec{
						Image: "test-image:test-version",
						Args:  []string{"prepare", "master-plan"},
					},
					Upgrade: &planv1.ContainerSpec{
						Image: "test-image",
					},
					Version: "test-version",
					NodeSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "node-role.kubernetes.io/master",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
							{
								Key:      "upgrade.cattle.io/kubernetes-upgrade",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
				Status: planv1.PlanStatus{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			workerPlan := workerPlan
			assert.Equal(t, tt.expected, configureWorkerPlan(workerPlan, tt.version, tt.concurrency, tt.drain, "test-image", "worker-plan", "master-plan"))
		})
	}
}
