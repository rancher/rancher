package operations

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	"github.com/rancher/rancher/pkg/plan"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/name"
)

// EndpointCADirectory is the directory, relative to the distro data directory, where rendered
// config files (including S3 endpoint CAs) are written on the node. It matches the location the
// CAPR planner uses for the same files, so a cluster whose CA was written by the planner resolves
// to the same path here.
const EndpointCADirectory = "etc/config-files"

// S3Target is the provider-neutral description of an etcd snapshot S3 location together with the
// credentials required to reach it.
//
// Every adapter resolves its own typed S3 configuration — the shared mgmt/rkev1 ETCDSnapshotS3 for
// CAPR and imported clusters, CAPRKE2's secret-backed EtcdS3 for CAPRKE2 — into this shape, and
// RenderS3 turns it into the distro arguments, environment variables, and files a plan needs.
type S3Target struct {
	Bucket   string
	Endpoint string
	Region   string
	Folder   string

	AccessKey string
	SecretKey string

	SkipSSLVerify bool

	// Retention is the number of snapshots to keep in the bucket. Zero leaves the distro default
	// (or whatever the node's own config sets) in place.
	Retention int

	// EndpointCAPath is the path on the node that the s3-endpoint-ca argument references. Empty
	// when the endpoint is signed by a CA the node already trusts.
	EndpointCAPath string

	// EndpointCAContent is the base64-encoded PEM to write at EndpointCAPath. Empty means the file
	// is expected to already exist on the node — whatever configured the cluster wrote it — so
	// only the argument is rendered and the file is left alone.
	EndpointCAContent string
}

// S3Credential holds the S3 access details resolved from a Rancher cloud credential.
type S3Credential struct {
	AccessKey     string
	SecretKey     string
	Region        string
	Endpoint      string
	EndpointCA    string
	SkipSSLVerify bool
	Bucket        string
	Folder        string
}

// S3Enabled reports whether s3 describes an S3 location at all. It mirrors planner.S3Enabled: any
// one of the location fields being set is enough, since the remaining values can come from the
// referenced cloud credential.
func S3Enabled(s3 *rkev1.ETCDSnapshotS3) bool {
	if s3 == nil {
		return false
	}
	return s3.Bucket != "" || s3.Endpoint != "" || s3.Folder != "" || s3.CloudCredentialName != "" || s3.Region != ""
}

// RenderS3 renders the distro arguments, environment variables, and files for target.
//
// prefix is prepended to every argument name ("etcd-" produces --etcd-s3-bucket, which is what
// both `<runtime> etcd-snapshot save` and `<runtime> server --cluster-reset` expect). When
// secretKeyInEnv is set the secret key is passed as AWS_SECRET_ACCESS_KEY instead of an argument,
// keeping it out of the process command line — use it for anything that runs as an instruction,
// and leave it unset when the arguments are being folded into a config file.
//
// The trailing --<prefix>s3 flag is only appended when at least one other argument was rendered,
// so a target with nothing resolved produces no arguments rather than enabling S3 with no
// destination.
func RenderS3(target *S3Target, prefix string, secretKeyInEnv bool) (args []string, env []string, files []plan.File) {
	if target == nil {
		return nil, nil, nil
	}

	if target.Bucket != "" {
		args = append(args, fmt.Sprintf("--%ss3-bucket=%s", prefix, target.Bucket))
	}
	if target.AccessKey != "" {
		args = append(args, fmt.Sprintf("--%ss3-access-key=%s", prefix, target.AccessKey))
	}
	if target.SecretKey != "" {
		if secretKeyInEnv {
			env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", target.SecretKey))
		} else {
			args = append(args, fmt.Sprintf("--%ss3-secret-key=%s", prefix, target.SecretKey))
		}
	}
	if target.Region != "" {
		args = append(args, fmt.Sprintf("--%ss3-region=%s", prefix, target.Region))
	}
	if target.Folder != "" {
		args = append(args, fmt.Sprintf("--%ss3-folder=%s", prefix, target.Folder))
	}
	if target.Endpoint != "" {
		args = append(args, fmt.Sprintf("--%ss3-endpoint=%s", prefix, target.Endpoint))
	}
	if target.SkipSSLVerify {
		args = append(args, fmt.Sprintf("--%ss3-skip-ssl-verify", prefix))
	}
	if target.EndpointCAPath != "" {
		args = append(args, fmt.Sprintf("--%ss3-endpoint-ca=%s", prefix, target.EndpointCAPath))
		if target.EndpointCAContent != "" {
			files = append(files, plan.File{
				Content: target.EndpointCAContent,
				Path:    target.EndpointCAPath,
			})
		}
	}
	if target.Retention > 0 {
		args = append(args, fmt.Sprintf("--%ss3-retention=%d", prefix, target.Retention))
	}

	if len(args) > 0 {
		args = append(args, fmt.Sprintf("--%ss3", prefix))
	}

	return args, env, files
}

// ResolveS3Target resolves an S3 location into an S3Target, filling in whatever the location does
// not specify from clusterS3 and from the cloud credential either of them references.
//
// s3 is the location to reach — the S3 block recorded on the snapshot being restored, or the
// cluster's own S3 configuration when rendering cluster config. A nil s3 falls back to clusterS3
// entirely. Snapshots never record credentials, which is why clusterS3 is always consulted for the
// cloud credential name.
//
// caDir is the directory rendered endpoint CA files are written to, and credNamespace the
// namespace a cloud credential name without an explicit "namespace:name" prefix is resolved in.
// Returns nil when no S3 location is configured.
func ResolveS3Target(secrets corecontrollers.SecretCache, s3, clusterS3 *rkev1.ETCDSnapshotS3, credNamespace, caDir string) (*S3Target, error) {
	if s3 == nil {
		s3 = clusterS3
	}
	if !S3Enabled(s3) {
		return nil, nil
	}

	credName := s3.CloudCredentialName
	if credName == "" && clusterS3 != nil {
		credName = clusterS3.CloudCredentialName
	}

	cred, err := GetS3Credential(secrets, credNamespace, credName)
	if err != nil {
		return nil, err
	}

	target := &S3Target{
		Bucket:        first(s3.Bucket, cred.Bucket),
		Endpoint:      first(s3.Endpoint, cred.Endpoint),
		Region:        first(s3.Region, cred.Region),
		Folder:        first(s3.Folder, cred.Folder),
		AccessKey:     cred.AccessKey,
		SecretKey:     cred.SecretKey,
		SkipSSLVerify: s3.SkipSSLVerify || cred.SkipSSLVerify,
		Retention:     s3.Retention,
	}

	if ca := first(s3.EndpointCA, cred.EndpointCA); ca != "" {
		if ca == s3.EndpointCA && strings.HasSuffix(ca, ".crt") {
			// The S3 block recorded the path of the CA file that was used when the snapshot was
			// taken rather than its content (this is what a snapshot backpopulated from a node's
			// config looks like). Reference that path, and re-render the file when the credential
			// or the cluster spec still holds content which hashes to the very same path — a
			// replaced node would otherwise have the argument but not the file.
			target.EndpointCAPath = ca
			for _, candidate := range []string{cred.EndpointCA, clusterEndpointCA(clusterS3)} {
				if candidate == "" {
					continue
				}
				if candidatePath, content := endpointCAFile(caDir, candidate); candidatePath == ca {
					target.EndpointCAContent = content
					break
				}
			}
		} else {
			target.EndpointCAPath, target.EndpointCAContent = endpointCAFile(caDir, ca)
		}
	}

	return target, nil
}

// GetS3Credential resolves a Rancher cloud credential into its S3 access details. An empty name
// yields a zero credential, which lets a cluster rely entirely on the node's IAM role.
//
// The credential name may be either "namespace:name" (the cluster-scoped form the mgmt API uses,
// resolved against the global namespace) or a plain name resolved in namespace.
func GetS3Credential(secrets corecontrollers.SecretCache, namespace, credentialName string) (S3Credential, error) {
	var result S3Credential

	if credentialName == "" {
		return result, nil
	}

	secret, err := machineprovision.GetCloudCredentialSecret(secrets, namespace, credentialName)
	if err != nil {
		return result, fmt.Errorf("failed to lookup cloud credential %s: %w", credentialName, err)
	}

	// Cloud credential keys are prefixed by driver ("s3credentialConfig-accessKey"); the driver is
	// irrelevant here, only the trailing field name matters.
	data := map[string][]byte{}
	for k, v := range secret.Data {
		_, k = kv.RSplit(k, "-")
		data[k] = v
	}

	return S3Credential{
		AccessKey:     string(data["accessKey"]),
		SecretKey:     string(data["secretKey"]),
		Region:        string(data["defaultRegion"]),
		Endpoint:      string(data["defaultEndpoint"]),
		EndpointCA:    string(data["defaultEndpointCA"]),
		SkipSSLVerify: string(data["defaultSkipSSLVerify"]) == "true",
		Bucket:        string(data["defaultBucket"]),
		Folder:        string(data["defaultFolder"]),
	}, nil
}

// endpointCAFile returns the path a CA with the given content is written to, and that content
// base64-encoded ready for a plan file. The path is derived from the content, so identical CAs
// resolve to identical paths across reconciles and across the planner and operations code paths.
func endpointCAFile(caDir, ca string) (string, string) {
	if _, err := base64.StdEncoding.DecodeString(ca); err != nil {
		// Not base64 — the spec holds the PEM itself, which a plan file needs encoded.
		ca = base64.StdEncoding.EncodeToString([]byte(ca))
	}
	return path.Join(caDir, fmt.Sprintf("s3-endpoint-ca-%s.crt", name.Hex(ca, 5))), ca
}

// clusterEndpointCA returns the endpoint CA recorded on the cluster's own S3 configuration, if any.
func clusterEndpointCA(clusterS3 *rkev1.ETCDSnapshotS3) string {
	if clusterS3 == nil {
		return ""
	}
	return clusterS3.EndpointCA
}

// first returns the first non-empty string of the two, left to right.
func first(one, two string) string {
	if one == "" {
		return two
	}
	return one
}
