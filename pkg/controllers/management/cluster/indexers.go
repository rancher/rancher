package cluster

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

const ByCloudCredential = "byCloudCredential"

func RegisterIndexers(context *config.ScaledContext) {
	context.Wrangler.Mgmt.Cluster().Cache().AddIndexer(ByCloudCredential, byCloudCredentialIndexer)
}

func byCloudCredentialIndexer(obj *v3.Cluster) ([]string, error) {
	switch {
	case obj.Spec.EKSConfig != nil:
		if obj.Spec.EKSConfig.AmazonCredentialSecret != "" {
			return []string{obj.Spec.EKSConfig.AmazonCredentialSecret}, nil
		}
	case obj.Spec.AKSConfig != nil:
		if obj.Spec.AKSConfig.AzureCredentialSecret != "" {
			return []string{obj.Spec.AKSConfig.AzureCredentialSecret}, nil
		}
	case obj.Spec.GKEConfig != nil:
		if obj.Spec.GKEConfig.GoogleCredentialSecret != "" {
			return []string{obj.Spec.GKEConfig.GoogleCredentialSecret}, nil
		}
	}
	return nil, nil
}
