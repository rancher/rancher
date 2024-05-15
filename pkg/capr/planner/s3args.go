package planner

import (
	"encoding/base64"
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/name"
)

// s3Args is a struct that contains functions used to generate arguments for etcd snapshots stored in S3
type s3Args struct {
	secretCache corecontrollers.SecretCache
}

// first returns the first non-blank string from the two passed in arguments (left to right)
func first(one, two string) string {
	if one == "" {
		return two
	}
	return one
}

// S3Enabled returns a boolean indicating whether S3 is enabled for the passed in ETCDSnapshotS3 struct.
func S3Enabled(s3 *rkev1.ETCDSnapshotS3) bool {
	if s3 == nil {
		return false
	}
	if s3.Bucket != "" || s3.Endpoint != "" || s3.Folder != "" || s3.CloudCredentialName != "" || s3.Region != "" {
		return true
	}
	return false
}

// ToArgs renders a slice of arguments and environment variables, as well as files (if S3 endpoints are required). If secretKeyInEnv is set to true, it will set the AWS_SECRET_ACCESS_KEY as an environment variable rather than as an argument.
func (s *s3Args) ToArgs(s3 *rkev1.ETCDSnapshotS3, controlPlane *rkev1.RKEControlPlane, prefix string, secretKeyInEnv bool) (args []string, env []string, files []plan.File, err error) {
	if s3 == nil {
		return
	}

	if !S3Enabled(s3) {
		return
	}

	var (
		s3Cred s3Credential
	)

	controlPlaneEtcdS3NotNil := controlPlane.Spec.ETCD != nil && controlPlane.Spec.ETCD.S3 != nil

	credName := s3.CloudCredentialName
	if credName == "" && controlPlaneEtcdS3NotNil {
		credName = controlPlane.Spec.ETCD.S3.CloudCredentialName
	}

	s3Cred, err = getS3Credential(s.secretCache, controlPlane.Namespace, credName)
	if err != nil {
		return
	}

	if s3.Bucket != "" || s3Cred.Bucket != "" {
		args = append(args, fmt.Sprintf("--%ss3-bucket=%s", prefix, first(s3.Bucket, s3Cred.Bucket)))
	}

	if s3Cred.AccessKey != "" {
		args = append(args, fmt.Sprintf("--%ss3-access-key=%s", prefix, s3Cred.AccessKey))
	}
	if s3Cred.SecretKey != "" {
		if secretKeyInEnv {
			env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", s3Cred.SecretKey))
		} else {
			args = append(args, fmt.Sprintf("--%ss3-secret-key=%s", prefix, s3Cred.SecretKey))
		}
	}
	if v := first(s3.Region, s3Cred.Region); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-region=%s", prefix, v))
	}
	if v := first(s3.Folder, s3Cred.Folder); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-folder=%s", prefix, v))
	}
	if v := first(s3.Endpoint, s3Cred.Endpoint); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-endpoint=%s", prefix, v))
	}
	if s3.SkipSSLVerify || s3Cred.SkipSSLVerify {
		args = append(args, fmt.Sprintf("--%ss3-skip-ssl-verify", prefix))
	}
	if v := first(s3.EndpointCA, s3Cred.EndpointCA); v != "" {
		// An etcd s3 snapshot object may have its endpoint CA be the filepath that was used to create the snapshot.
		// If this is the case, search the corresponding cloud credential and controlplane S3 spec for the actual CA data,
		// and use if it exists.
		// TODO: find a better way to bubble an error to the user that a restore is failing due to a missing endpoint CA file.
		if v == s3.EndpointCA && strings.HasSuffix(v, ".crt") {
			args = append(args, fmt.Sprintf("--%ss3-endpoint-ca=%s", prefix, v))
			if s3Cred.EndpointCA != "" {
				possibleCA := generateEndpointCAFileIfPathMatches(controlPlane, v, s3Cred.EndpointCA)
				if possibleCA != nil {
					files = append(files, *possibleCA)
				}
			} else if controlPlaneEtcdS3NotNil && controlPlane.Spec.ETCD.S3.EndpointCA != "" {
				possibleCA := generateEndpointCAFileIfPathMatches(controlPlane, v, controlPlane.Spec.ETCD.S3.EndpointCA)
				if possibleCA != nil {
					files = append(files, *possibleCA)
				}
			}
		} else {
			if _, err := base64.StdEncoding.DecodeString(v); err != nil {
				// There was an error decoding the endpointCA, indicating that it needs to be encoded.
				v = base64.StdEncoding.EncodeToString([]byte(v))
			}
			s3CAName := fmt.Sprintf("s3-endpoint-ca-%s.crt", name.Hex(v, 5))
			filePath := configFile(controlPlane, s3CAName)
			files = append(files, plan.File{
				Content: v,
				Path:    filePath,
			})
			args = append(args, fmt.Sprintf("--%ss3-endpoint-ca=%s", prefix, filePath))
		}
	}

	if len(args) > 0 {
		args = append(args,
			fmt.Sprintf("--%ss3", prefix))
	}

	return
}

func generateEndpointCAFileIfPathMatches(controlPlane *rkev1.RKEControlPlane, existingEndpointCAPath, endpointCA string) *plan.File {
	s3CAName := fmt.Sprintf("s3-endpoint-ca-%s.crt", name.Hex(endpointCA, 5))
	filePath := configFile(controlPlane, s3CAName)
	if existingEndpointCAPath == filePath {
		// If the filepath of the s3cred endpointCA matches the endpoint CA that was used for the snapshot, go ahead and include the file for posterity
		if _, err := base64.StdEncoding.DecodeString(endpointCA); err != nil {
			// There was an error decoding the endpointCA, indicating that it needs to be encoded.
			endpointCA = base64.StdEncoding.EncodeToString([]byte(endpointCA))
		}
		return &plan.File{
			Content: endpointCA,
			Path:    filePath,
		}
	}
	return nil
}

type s3Credential struct {
	AccessKey     string
	SecretKey     string
	Region        string
	Endpoint      string
	EndpointCA    string
	SkipSSLVerify bool
	Bucket        string
	Folder        string
}

func getS3Credential(secretCache corecontrollers.SecretCache, namespace, name string) (result s3Credential, _ error) {
	if name == "" {
		return result, nil
	}

	secret, err := machineprovision.GetCloudCredentialSecret(secretCache, namespace, name)
	if err != nil {
		return result, fmt.Errorf("failed to lookup etcdSnapshotCloudCredentialName: %w", err)
	}

	data := map[string][]byte{}
	for k, v := range secret.Data {
		_, k = kv.RSplit(k, "-")
		data[k] = v
	}

	return s3Credential{
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
