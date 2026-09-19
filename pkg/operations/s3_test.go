package operations

import (
	"encoding/base64"
	"fmt"
	"testing"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const testCADir = "/var/lib/rancher/rke2/etc/config-files"

// --- S3Enabled ------------------------------------------------------------------------------

func TestS3Enabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		s3   *rkev1.ETCDSnapshotS3
		want bool
	}{
		{"nil", nil, false},
		{"empty", &rkev1.ETCDSnapshotS3{}, false},
		{"bucket", &rkev1.ETCDSnapshotS3{Bucket: "bucket"}, true},
		{"endpoint", &rkev1.ETCDSnapshotS3{Endpoint: "s3.example.com"}, true},
		{"folder", &rkev1.ETCDSnapshotS3{Folder: "folder"}, true},
		{"region", &rkev1.ETCDSnapshotS3{Region: "us-east-1"}, true},
		{"cloud credential only", &rkev1.ETCDSnapshotS3{CloudCredentialName: "cattle-global-data:cc-abcde"}, true},
		// SkipSSLVerify on its own describes no location, so it does not enable S3.
		{"skip ssl verify only", &rkev1.ETCDSnapshotS3{SkipSSLVerify: true}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, S3Enabled(tc.s3))
		})
	}
}

// --- RenderS3 -------------------------------------------------------------------------------

func TestRenderS3(t *testing.T) {
	t.Parallel()

	full := &S3Target{
		Bucket:            "bucket",
		Endpoint:          "s3.example.com",
		Region:            "us-east-1",
		Folder:            "folder",
		AccessKey:         "access",
		SecretKey:         "secret",
		SkipSSLVerify:     true,
		Retention:         7,
		EndpointCAPath:    "/var/lib/rancher/rke2/etc/config-files/s3-endpoint-ca-abcde.crt",
		EndpointCAContent: base64.StdEncoding.EncodeToString([]byte("ca")),
	}

	t.Run("nil target renders nothing", func(t *testing.T) {
		t.Parallel()
		args, env, files := RenderS3(nil, "etcd-", true)
		assert.Empty(t, args)
		assert.Empty(t, env)
		assert.Empty(t, files)
	})

	t.Run("empty target does not enable s3", func(t *testing.T) {
		t.Parallel()
		args, env, files := RenderS3(&S3Target{}, "etcd-", true)
		assert.Empty(t, args, "the trailing --etcd-s3 flag must not be rendered on its own")
		assert.Empty(t, env)
		assert.Empty(t, files)
	})

	t.Run("secret key as argument", func(t *testing.T) {
		t.Parallel()
		args, env, files := RenderS3(full, "etcd-", false)

		assert.Equal(t, []string{
			"--etcd-s3-bucket=bucket",
			"--etcd-s3-access-key=access",
			"--etcd-s3-secret-key=secret",
			"--etcd-s3-region=us-east-1",
			"--etcd-s3-folder=folder",
			"--etcd-s3-endpoint=s3.example.com",
			"--etcd-s3-skip-ssl-verify",
			"--etcd-s3-endpoint-ca=" + full.EndpointCAPath,
			"--etcd-s3-retention=7",
			"--etcd-s3",
		}, args)
		assert.Empty(t, env)

		require.Len(t, files, 1)
		assert.Equal(t, full.EndpointCAPath, files[0].Path)
		assert.Equal(t, full.EndpointCAContent, files[0].Content)
	})

	t.Run("secret key in environment", func(t *testing.T) {
		t.Parallel()
		args, env, _ := RenderS3(full, "etcd-", true)

		assert.NotContains(t, args, "--etcd-s3-secret-key=secret")
		assert.Equal(t, []string{"AWS_SECRET_ACCESS_KEY=secret"}, env)
	})

	t.Run("endpoint ca without content renders the argument only", func(t *testing.T) {
		t.Parallel()
		args, _, files := RenderS3(&S3Target{
			Bucket:         "bucket",
			EndpointCAPath: CAPRKE2EndpointCAPath,
		}, "etcd-", true)

		assert.Contains(t, args, "--etcd-s3-endpoint-ca="+CAPRKE2EndpointCAPath)
		assert.Empty(t, files, "a CA already on the node must not be re-rendered")
	})

	t.Run("prefix is applied to every argument", func(t *testing.T) {
		t.Parallel()
		args, _, _ := RenderS3(&S3Target{Bucket: "bucket"}, "", false)
		assert.Equal(t, []string{"--s3-bucket=bucket", "--s3"}, args)
	})
}

// --- ResolveS3Target ------------------------------------------------------------------------

// newCloudCredential builds a cloud credential secret in the shape getS3Credential expects:
// driver-prefixed keys whose trailing segment is the field name.
func newCloudCredential(namespace, credName string, data map[string]string) *corev1.Secret {
	secretData := map[string][]byte{}
	for k, v := range data {
		secretData["s3credentialConfig-"+k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: credName},
		Data:       secretData,
	}
}

func TestResolveS3Target(t *testing.T) {
	t.Parallel()

	t.Run("nil and disabled specs resolve to no target", func(t *testing.T) {
		t.Parallel()

		target, err := ResolveS3Target(nil, nil, nil, "fleet-default", testCADir)
		assert.NoError(t, err)
		assert.Nil(t, target)

		target, err = ResolveS3Target(nil, &rkev1.ETCDSnapshotS3{}, &rkev1.ETCDSnapshotS3{}, "fleet-default", testCADir)
		assert.NoError(t, err)
		assert.Nil(t, target)
	})

	t.Run("spec values take precedence over the credential", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secretCache.EXPECT().Get("cattle-global-data", "cc-abcde").Return(newCloudCredential("cattle-global-data", "cc-abcde", map[string]string{
			"accessKey":            "access",
			"secretKey":            "secret",
			"defaultRegion":        "cred-region",
			"defaultEndpoint":      "cred-endpoint",
			"defaultBucket":        "cred-bucket",
			"defaultFolder":        "cred-folder",
			"defaultSkipSSLVerify": "true",
		}), nil)

		target, err := ResolveS3Target(secretCache, &rkev1.ETCDSnapshotS3{
			CloudCredentialName: "cattle-global-data:cc-abcde",
			Bucket:              "spec-bucket",
			Region:              "spec-region",
			Retention:           3,
		}, nil, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)

		assert.Equal(t, "spec-bucket", target.Bucket)
		assert.Equal(t, "spec-region", target.Region)
		assert.Equal(t, "cred-endpoint", target.Endpoint, "unset spec fields fall back to the credential")
		assert.Equal(t, "cred-folder", target.Folder)
		assert.Equal(t, "access", target.AccessKey)
		assert.Equal(t, "secret", target.SecretKey)
		assert.True(t, target.SkipSSLVerify, "the credential may turn off verification on its own")
		assert.Equal(t, 3, target.Retention)
	})

	t.Run("credential name falls back to the cluster spec", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secretCache.EXPECT().Get("cattle-global-data", "cc-cluster").Return(newCloudCredential("cattle-global-data", "cc-cluster", map[string]string{
			"accessKey": "access",
			"secretKey": "secret",
		}), nil)

		// A snapshot records where it was written but never how to authenticate.
		snapshotS3 := &rkev1.ETCDSnapshotS3{Bucket: "snapshot-bucket", Endpoint: "s3.example.com"}
		clusterS3 := &rkev1.ETCDSnapshotS3{CloudCredentialName: "cattle-global-data:cc-cluster", Bucket: "cluster-bucket"}

		target, err := ResolveS3Target(secretCache, snapshotS3, clusterS3, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)

		assert.Equal(t, "snapshot-bucket", target.Bucket, "the snapshot's own location must win")
		assert.Equal(t, "access", target.AccessKey)
		assert.Equal(t, "secret", target.SecretKey)
	})

	t.Run("nil spec falls back to the cluster spec entirely", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)

		target, err := ResolveS3Target(secretCache, nil, &rkev1.ETCDSnapshotS3{Bucket: "cluster-bucket"}, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)
		assert.Equal(t, "cluster-bucket", target.Bucket)
	})

	t.Run("endpoint CA content is rendered to a content-derived path", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)

		ca := "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----"
		encoded := base64.StdEncoding.EncodeToString([]byte(ca))

		target, err := ResolveS3Target(secretCache, &rkev1.ETCDSnapshotS3{
			Bucket:     "bucket",
			EndpointCA: ca,
		}, nil, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)

		want := fmt.Sprintf("%s/s3-endpoint-ca-%s.crt", testCADir, name.Hex(encoded, 5))
		assert.Equal(t, want, target.EndpointCAPath)
		assert.Equal(t, encoded, target.EndpointCAContent, "plan file content must be base64")
	})

	t.Run("recorded endpoint CA path is re-rendered when the cluster still holds its content", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)

		ca := base64.StdEncoding.EncodeToString([]byte("ca-content"))
		caPath := fmt.Sprintf("%s/s3-endpoint-ca-%s.crt", testCADir, name.Hex(ca, 5))

		// This is what a backpopulated snapshot looks like: EndpointCA is the path of the file
		// that was on the node, not the certificate itself.
		snapshotS3 := &rkev1.ETCDSnapshotS3{Bucket: "bucket", EndpointCA: caPath}
		clusterS3 := &rkev1.ETCDSnapshotS3{EndpointCA: ca}

		target, err := ResolveS3Target(secretCache, snapshotS3, clusterS3, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)

		assert.Equal(t, caPath, target.EndpointCAPath)
		assert.Equal(t, ca, target.EndpointCAContent)
	})

	t.Run("recorded endpoint CA path with no known content is referenced only", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)

		caPath := testCADir + "/s3-endpoint-ca-zzzzz.crt"
		target, err := ResolveS3Target(secretCache, &rkev1.ETCDSnapshotS3{Bucket: "bucket", EndpointCA: caPath}, nil, "fleet-default", testCADir)
		require.NoError(t, err)
		require.NotNil(t, target)

		assert.Equal(t, caPath, target.EndpointCAPath)
		assert.Empty(t, target.EndpointCAContent)
	})

	t.Run("credential lookup failure is returned", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secretCache.EXPECT().Get("cattle-global-data", "cc-missing").Return(nil, fmt.Errorf("not found"))

		target, err := ResolveS3Target(secretCache, &rkev1.ETCDSnapshotS3{
			Bucket:              "bucket",
			CloudCredentialName: "cattle-global-data:cc-missing",
		}, nil, "fleet-default", testCADir)
		assert.Error(t, err)
		assert.Nil(t, target)
	})
}

// --- adapters -------------------------------------------------------------------------------

func TestImportedAdapter_ETCDSnapshotS3(t *testing.T) {
	t.Parallel()

	rke2S3 := &rkev1.ETCDSnapshotS3{Bucket: "rke2-bucket"}
	k3sS3 := &rkev1.ETCDSnapshotS3{Bucket: "k3s-bucket"}

	cluster := func(provider string) *mgmtv3.Cluster {
		return &mgmtv3.Cluster{
			Spec: mgmtv3.ClusterSpec{
				Rke2Config: &mgmtv3.Rke2Config{ETCD: mgmtv3.ETCD{S3: rke2S3}},
				K3sConfig:  &mgmtv3.K3sConfig{ETCD: mgmtv3.ETCD{S3: k3sS3}},
			},
			Status: mgmtv3.ClusterStatus{Provider: provider},
		}
	}

	assert.Equal(t, rke2S3, (&ImportedAdapter{cluster: cluster("rke2")}).ETCDSnapshotS3())
	assert.Equal(t, k3sS3, (&ImportedAdapter{cluster: cluster("k3s")}).ETCDSnapshotS3())

	// No distro config at all means no S3 configuration.
	assert.Nil(t, (&ImportedAdapter{cluster: &mgmtv3.Cluster{
		Status: mgmtv3.ClusterStatus{Provider: "rke2"},
	}}).ETCDSnapshotS3())
}

func TestImportedAdapter_ToS3ArgsEnvAndFiles(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().Get("cattle-global-data", "cc-abcde").Return(newCloudCredential("cattle-global-data", "cc-abcde", map[string]string{
		"accessKey": "access",
		"secretKey": "secret",
	}), nil)

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-m-abcde"},
			Spec: mgmtv3.ClusterSpec{
				Rke2Config: &mgmtv3.Rke2Config{ETCD: mgmtv3.ETCD{
					S3: &rkev1.ETCDSnapshotS3{
						Bucket:              "bucket",
						Endpoint:            "s3.example.com",
						CloudCredentialName: "cattle-global-data:cc-abcde",
					},
				}},
			},
			Status: mgmtv3.ClusterStatus{Provider: "rke2"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{Core: &stubCoreInterface{secretCache: secretCache}},
		},
	}

	args, env, files, err := adapter.ToS3ArgsEnvAndFiles(nil, adapter.ETCDSnapshotS3(), "etcd-", true)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"--etcd-s3-bucket=bucket",
		"--etcd-s3-access-key=access",
		"--etcd-s3-endpoint=s3.example.com",
		"--etcd-s3",
	}, args)
	assert.Equal(t, []string{"AWS_SECRET_ACCESS_KEY=secret"}, env)
	assert.Empty(t, files)
}

func newCAPRKE2TestAdapter(secretCache generic.CacheInterface[*corev1.Secret], s3 *controlplanev1beta2.EtcdS3) *CAPRKE2Adapter {
	return &CAPRKE2Adapter{
		cluster: &capiv1beta2.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "downstream"},
		},
		controlPlane: &controlplanev1beta2.RKE2ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "downstream"},
			Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
				ServerConfig: controlplanev1beta2.RKE2ServerConfig{
					Etcd: controlplanev1beta2.EtcdConfig{
						BackupConfig: controlplanev1beta2.EtcdBackupConfig{S3: s3},
					},
				},
			},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{Core: &stubCoreInterface{secretCache: secretCache}},
		},
	}
}

func TestCAPRKE2Adapter_S3(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().Get("fleet-default", "s3-creds").Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "s3-creds"},
		Data: map[string][]byte{
			"aws_access_key_id":     []byte("access"),
			"aws_secret_access_key": []byte("secret"),
		},
	}, nil)
	secretCache.EXPECT().Get("fleet-default", "s3-ca").Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "s3-ca"},
		Data:       map[string][]byte{"ca.pem": []byte("ca-content")},
	}, nil)

	adapter := newCAPRKE2TestAdapter(secretCache, &controlplanev1beta2.EtcdS3{
		Bucket:   "bucket",
		Endpoint: "s3.example.com",
		Region:   "us-east-1",
		Folder:   "folder",
		// EnforceSSLVerify left unset: CAPRKE2 treats that as "skip verification".
		S3CredentialSecret: &corev1.ObjectReference{Name: "s3-creds"},
		EndpointCASecret:   &corev1.ObjectReference{Name: "s3-ca"},
	})

	clusterS3 := adapter.ETCDSnapshotS3()
	require.NotNil(t, clusterS3)
	assert.Equal(t, "bucket", clusterS3.Bucket)
	assert.True(t, clusterS3.SkipSSLVerify, "CAPRKE2 inverts the flag: verification is opt-in")
	assert.Empty(t, clusterS3.CloudCredentialName, "CAPRKE2 has no cloud credential to report")

	args, env, files, err := adapter.ToS3ArgsEnvAndFiles(nil, nil, "etcd-", true)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"--etcd-s3-bucket=bucket",
		"--etcd-s3-access-key=access",
		"--etcd-s3-region=us-east-1",
		"--etcd-s3-folder=folder",
		"--etcd-s3-endpoint=s3.example.com",
		"--etcd-s3-skip-ssl-verify",
		"--etcd-s3-endpoint-ca=" + CAPRKE2EndpointCAPath,
		"--etcd-s3",
	}, args)
	assert.Equal(t, []string{"AWS_SECRET_ACCESS_KEY=secret"}, env)

	require.Len(t, files, 1)
	assert.Equal(t, CAPRKE2EndpointCAPath, files[0].Path)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("ca-content")), files[0].Content)
}

func TestCAPRKE2Adapter_S3Disabled(t *testing.T) {
	t.Parallel()

	adapter := newCAPRKE2TestAdapter(nil, nil)

	assert.Nil(t, adapter.ETCDSnapshotS3())

	args, env, files, err := adapter.ToS3ArgsEnvAndFiles(nil, nil, "etcd-", true)
	assert.NoError(t, err)
	assert.Empty(t, args)
	assert.Empty(t, env)
	assert.Empty(t, files)
}
