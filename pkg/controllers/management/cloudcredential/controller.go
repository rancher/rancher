package cloudcredential

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	corev1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	cloudCredentials  corev1.SecretInterface
	managementContext *config.ManagementContext
}

func Register(ctx context.Context, management *config.ManagementContext) {
	cloudCredentials := management.Core.Secrets("")
	m := Controller{
		cloudCredentials:  cloudCredentials,
		managementContext: management,
	}
	m.cloudCredentials.AddHandler(ctx, "management-cloudcredential-controller", m.ccSync)
}

func (n *Controller) ccSync(key string, cloudCredential *v1.Secret) (runtime.Object, error) {
	if cloudCredential == nil || cloudCredential.DeletionTimestamp != nil {
		return cloudCredential, nil
	}
	if !configExists(cloudCredential.Data) {
		return cloudCredential, nil
	}
	metaAccessor, err := meta.Accessor(cloudCredential)
	if err != nil {
		return cloudCredential, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[globalnamespacerbac.CreatorIDAnn]
	if !ok {
		return cloudCredential, fmt.Errorf("cloud credential %v has no creatorId annotation", cloudCredential.Name)
	}
	if err := globalnamespacerbac.CreateRoleAndRoleBinding(
		globalnamespacerbac.CloudCredentialResource, cloudCredential.Name, "v1", creatorID, []string{"*"}, cloudCredential.UID, []v3.Member{},
		n.managementContext); err != nil {
		return nil, err
	}

	return cloudCredential, nil
}

func configExists(data map[string][]byte) bool {
	for key := range data {
		splitKey := strings.Split(key, "-")
		if len(splitKey) == 2 && strings.HasSuffix(splitKey[0], "Config") {
			return true
		}
	}
	return false
}
