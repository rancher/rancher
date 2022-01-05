package planner

import (
	"encoding/base64"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kv"
)

// s3Args is a struct that contains functions used to generate arguments for etcd snapshots stored in S3
// If env is set to true, it will set the AWS_SECRET_ACCESS_KEY as an environment variable rather than as an argument.
type s3Args struct {
	prefix      string
	secretCache corecontrollers.SecretCache
	env         bool
}

func first(one, two string) string {
	if one == "" {
		return two
	}
	return one
}

func (s *s3Args) ToArgs(s3 *rkev1.ETCDSnapshotS3, controlPlane *rkev1.RKEControlPlane) (args []string, env []string, files []plan.File, err error) {
	if s3 == nil {
		return
	}

	var (
		s3Cred s3Credential
	)

	credName := s3.CloudCredentialName
	if credName == "" && controlPlane.Spec.ETCD != nil && controlPlane.Spec.ETCD.S3 != nil {
		credName = controlPlane.Spec.ETCD.S3.CloudCredentialName
	}

	s3Cred, err = getS3Credential(s.secretCache, controlPlane.Namespace, credName)
	if err != nil {
		return
	}

	if s3.Bucket != "" || s3Cred.Bucket != "" {
		args = append(args, fmt.Sprintf("--%ss3-bucket=%s", s.prefix, first(s3.Bucket, s3Cred.Bucket)))
	}

	if s3Cred.AccessKey != "" {
		args = append(args, fmt.Sprintf("--%ss3-access-key=%s", s.prefix, s3Cred.AccessKey))
	}
	if s3Cred.SecretKey != "" {
		if s.env {
			env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", s3Cred.SecretKey))
		} else {
			args = append(args, fmt.Sprintf("--%ss3-secret-key=%s", s.prefix, s3Cred.SecretKey))
		}
	}
	if v := first(s3.Region, s3Cred.Region); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-region=%s", s.prefix, v))
	}
	if v := first(s3.Folder, s3Cred.Folder); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-folder=%s", s.prefix, v))
	}
	if v := first(s3.Endpoint, s3Cred.Endpoint); v != "" {
		args = append(args, fmt.Sprintf("--%ss3-endpoint=%s", s.prefix, v))
	}
	if s3.SkipSSLVerify || s3Cred.SkipSSLVerify {
		args = append(args, fmt.Sprintf("--%ss3-skip-ssl-verify", s.prefix))
	}
	if v := first(s3.EndpointCA, s3Cred.EndpointCA); v != "" {
		filePath := configFile(controlPlane, "s3-endpoint-ca.crt")
		files = append(files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(v)),
			Path:    filePath,
		})
		args = append(args, fmt.Sprintf("--%ss3-endpoint-ca=%s", s.prefix, filePath))
	}

	if len(args) > 0 {
		args = append(args,
			fmt.Sprintf("--%ss3", s.prefix))
	}

	return
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
