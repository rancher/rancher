package config

import (
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	planapi "github.com/rancher/rancher/pkg/plan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// stubS3Renderer stands in for the S3 half of ops.Adapter: it records the spec it was asked to
// render and returns canned arguments and files.
type stubS3Renderer struct {
	args  []string
	files []planapi.File
	err   error

	requested      *rkev1.ETCDSnapshotS3
	prefix         string
	secretKeyInEnv bool
	timesCalled    int
}

func (s *stubS3Renderer) ToS3ArgsEnvAndFiles(_ *corev1.Secret, s3 *rkev1.ETCDSnapshotS3, prefix string, secretKeyInEnv bool) ([]string, []string, []planapi.File, error) {
	s.requested = s3
	s.prefix = prefix
	s.secretKeyInEnv = secretKeyInEnv
	s.timesCalled++
	if s.err != nil {
		return nil, nil, nil, s.err
	}
	return s.args, nil, s.files, nil
}

func TestManagedRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *apimgmtv3.Cluster
		expected string
	}{
		{
			name: "imported rke2",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abcde"},
				Status:     apimgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
			},
			expected: capr.RuntimeRKE2,
		},
		{
			name: "imported k3s",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abcde"},
				Status:     apimgmtv3.ClusterStatus{Provider: capr.RuntimeK3S},
			},
			expected: capr.RuntimeK3S,
		},
		{
			name: "local cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local"},
				Status:     apimgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
			},
		},
		{
			name: "turtles imported capi cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-m-abcde",
					Labels: map[string]string{
						capr.CAPIClusterOwnerLabel:   "downstream",
						capr.CAPIClusterOwnerNSLabel: "fleet-default",
					},
				},
				Status: apimgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
			},
		},
		{
			name: "partially labeled capi cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "c-m-abcde",
					Labels: map[string]string{capr.CAPIClusterOwnerLabel: "downstream"},
				},
				Status: apimgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
			},
		},
		{
			name: "administrated cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abcde",
					Annotations: map[string]string{administratedAnnotation: "true"},
				},
				Status: apimgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
			},
		},
		{
			name: "non rke2/k3s provider",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abcde"},
				Status:     apimgmtv3.ClusterStatus{Provider: "rke"},
			},
		},
		{
			name: "provider not yet detected",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abcde"},
				Status:     apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverRke2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := managedRuntime(tt.cluster); got != tt.expected {
				t.Fatalf("expected runtime %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestETCDSpec(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			Rke2Config: &apimgmtv3.Rke2Config{ETCD: apimgmtv3.ETCD{SnapshotRetention: 10}},
			K3sConfig:  &apimgmtv3.K3sConfig{ETCD: apimgmtv3.ETCD{SnapshotRetention: 20}},
		},
	}

	if got := etcdSpec(cluster, capr.RuntimeRKE2).SnapshotRetention; got != 10 {
		t.Fatalf("expected rke2 retention 10, got %d", got)
	}
	if got := etcdSpec(cluster, capr.RuntimeK3S).SnapshotRetention; got != 20 {
		t.Fatalf("expected k3s retention 20, got %d", got)
	}

	// An unset distro config is equivalent to an empty etcd config.
	empty := &apimgmtv3.Cluster{}
	if got := etcdSpec(empty, capr.RuntimeRKE2); got != (apimgmtv3.ETCD{}) {
		t.Fatalf("expected empty etcd config, got %+v", got)
	}
	if got := etcdSpec(cluster, "rke"); got != (apimgmtv3.ETCD{}) {
		t.Fatalf("expected empty etcd config for unknown runtime, got %+v", got)
	}
}

func TestRenderETCDConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		etcd       apimgmtv3.ETCD
		s3Args     []string
		s3Files    []planapi.File
		expected   string
		configured bool
		files      int
	}{
		{
			name:     "nothing configured",
			etcd:     apimgmtv3.ETCD{},
			expected: "{}",
		},
		{
			name:       "retention only",
			etcd:       apimgmtv3.ETCD{SnapshotRetention: 10},
			expected:   "{\n  \"etcd-snapshot-retention\": 10\n}",
			configured: true,
		},
		{
			name: "all snapshot settings",
			etcd: apimgmtv3.ETCD{
				DisableSnapshots:     true,
				SnapshotRetention:    5,
				SnapshotScheduleCron: "0 */5 * * *",
			},
			expected:   "{\n  \"etcd-disable-snapshots\": true,\n  \"etcd-snapshot-retention\": 5,\n  \"etcd-snapshot-schedule-cron\": \"0 */5 * * *\"\n}",
			configured: true,
		},
		{
			name:     "zero retention is not rendered",
			etcd:     apimgmtv3.ETCD{SnapshotRetention: 0},
			expected: "{}",
		},
		{
			name: "s3 arguments become config keys",
			etcd: apimgmtv3.ETCD{S3: &rkev1.ETCDSnapshotS3{Bucket: "bucket"}},
			s3Args: []string{
				"--etcd-s3-bucket=bucket",
				"--etcd-s3-access-key=access",
				"--etcd-s3-secret-key=secret",
				"--etcd-s3-endpoint=s3.example.com",
				"--etcd-s3-skip-ssl-verify",
				"--etcd-s3-retention=3",
				"--etcd-s3",
			},
			expected:   "{\n  \"etcd-s3\": true,\n  \"etcd-s3-access-key\": \"access\",\n  \"etcd-s3-bucket\": \"bucket\",\n  \"etcd-s3-endpoint\": \"s3.example.com\",\n  \"etcd-s3-retention\": 3,\n  \"etcd-s3-secret-key\": \"secret\",\n  \"etcd-s3-skip-ssl-verify\": true\n}",
			configured: true,
		},
		{
			name: "s3 endpoint CA file is carried alongside the config",
			etcd: apimgmtv3.ETCD{S3: &rkev1.ETCDSnapshotS3{Bucket: "bucket"}},
			s3Args: []string{
				"--etcd-s3-endpoint-ca=/var/lib/rancher/rke2/etc/config-files/s3-endpoint-ca-abcde.crt",
				"--etcd-s3",
			},
			s3Files: []planapi.File{{
				Path:    "/var/lib/rancher/rke2/etc/config-files/s3-endpoint-ca-abcde.crt",
				Content: "Y2E=",
			}},
			expected:   "{\n  \"etcd-s3\": true,\n  \"etcd-s3-endpoint-ca\": \"/var/lib/rancher/rke2/etc/config-files/s3-endpoint-ca-abcde.crt\"\n}",
			configured: true,
			files:      1,
		},
		{
			name: "snapshot settings and s3 are combined",
			etcd: apimgmtv3.ETCD{
				SnapshotRetention: 5,
				S3:                &rkev1.ETCDSnapshotS3{Bucket: "bucket"},
			},
			s3Args:     []string{"--etcd-s3-bucket=bucket", "--etcd-s3"},
			expected:   "{\n  \"etcd-s3\": true,\n  \"etcd-s3-bucket\": \"bucket\",\n  \"etcd-snapshot-retention\": 5\n}",
			configured: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			renderer := &stubS3Renderer{args: tt.s3Args, files: tt.s3Files}

			content, files, configured, err := renderETCDConfig(renderer, nil, tt.etcd)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.expected {
				t.Fatalf("expected content %q, got %q", tt.expected, string(content))
			}
			if configured != tt.configured {
				t.Fatalf("expected configured %v, got %v", tt.configured, configured)
			}
			if len(files) != tt.files {
				t.Fatalf("expected %d extra files, got %d", tt.files, len(files))
			}

			if renderer.requested != tt.etcd.S3 {
				t.Fatalf("expected the cluster's own S3 spec to be rendered")
			}
			if renderer.prefix != etcdConfigArgPrefix {
				t.Fatalf("expected prefix %q, got %q", etcdConfigArgPrefix, renderer.prefix)
			}
			if renderer.secretKeyInEnv {
				t.Fatalf("config file rendering must not push the secret key into an environment variable")
			}
		})
	}
}

func TestRenderETCDConfigS3Error(t *testing.T) {
	t.Parallel()

	renderer := &stubS3Renderer{err: errors.New("credential missing")}

	if _, _, _, err := renderETCDConfig(renderer, nil, apimgmtv3.ETCD{S3: &rkev1.ETCDSnapshotS3{Bucket: "bucket"}}); err == nil {
		t.Fatal("expected an unresolvable S3 configuration to be an error rather than a config file without credentials")
	}
}

func TestRenderETCDConfigHashIsStable(t *testing.T) {
	t.Parallel()

	etcd := apimgmtv3.ETCD{
		DisableSnapshots:     true,
		SnapshotRetention:    5,
		SnapshotScheduleCron: "0 */5 * * *",
	}

	first, _, _, err := renderETCDConfig(&stubS3Renderer{}, nil, etcd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, _, _, err := renderETCDConfig(&stubS3Renderer{}, nil, etcd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planapi.PlanHash(first) != planapi.PlanHash(second) {
		t.Fatalf("expected identical etcd configs to hash identically")
	}

	changed, _, _, err := renderETCDConfig(&stubS3Renderer{}, nil, apimgmtv3.ETCD{SnapshotRetention: 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if planapi.PlanHash(first) == planapi.PlanHash(changed) {
		t.Fatalf("expected differing etcd configs to hash differently")
	}

	// A rotated S3 credential changes the rendered file, which is what triggers redelivery.
	withCred, _, _, err := renderETCDConfig(&stubS3Renderer{args: []string{"--etcd-s3-secret-key=secret", "--etcd-s3"}}, nil, etcd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rotatedCred, _, _, err := renderETCDConfig(&stubS3Renderer{args: []string{"--etcd-s3-secret-key=rotated", "--etcd-s3"}}, nil, etcd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if planapi.PlanHash(withCred) == planapi.PlanHash(rotatedCred) {
		t.Fatalf("expected a rotated credential to change the hash")
	}
}

func TestTargetPending(t *testing.T) {
	t.Parallel()

	node := func(appliedHash string) *apimgmtv3.Node {
		n := &apimgmtv3.Node{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-abcde", Name: "node-1"}}
		if appliedHash != "" {
			n.Annotations = map[string]string{AppliedETCDConfigHashAnnotation: appliedHash}
		}
		return n
	}

	tests := []struct {
		name       string
		applied    string
		configured bool
		pending    bool
	}{
		{name: "up to date", applied: "current", configured: true},
		{name: "stale hash", applied: "stale", configured: true, pending: true},
		{name: "never delivered", applied: "", configured: true, pending: true},
		{
			name:       "nothing configured and nothing delivered",
			applied:    "",
			configured: false,
		},
		{
			// The drop-in has to be delivered empty so the keys it used to set are cleared.
			name:       "nothing configured but previously delivered",
			applied:    "stale",
			configured: false,
			pending:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tgt := target{node: node(tt.applied), hash: "current", configured: tt.configured}
			if got := tgt.pending(); got != tt.pending {
				t.Fatalf("expected pending %v, got %v", tt.pending, got)
			}
		})
	}
}
