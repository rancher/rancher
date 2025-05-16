package cloudcredential

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/rancher/pkg/api/norman/customization/cred"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	typesv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	harvesterCloudCredentialTokenChecksumAnnotation = "management.cattle.io/harvester-token-checksum"
	harvesterCloudCredentialFinalizer               = "management.cattle.io/harvester-token-cleanup"
	harvesterCloudCredentialExpirationAnnotation    = "rancher.io/expiration-timestamp"
)

type Controller struct {
	managementContext *config.ManagementContext
	secretClient      corecontrollers.SecretClient
	tokenClient       mgmtcontrollers.TokenClient
	apply             apply.Apply
}

func Register(ctx context.Context, management *config.ManagementContext, clients *wrangler.Context) {
	m := Controller{
		managementContext: management,
		secretClient:      clients.Core.Secret(),
		tokenClient:       clients.Mgmt.Token(),
		apply: clients.Apply.WithCacheTypes(
			clients.Core.Secret(),
			clients.Mgmt.Token(),
		),
	}
	management.Core.Secrets("").AddHandler(ctx, "management-cloudcredential-controller", m.ccSync)
	if features.Harvester.Enabled() {
		clients.Core.Secret().OnChange(ctx, "harvester-cloud-credential-token", m.syncHarvesterToken)
	}
}

// syncHarvesterToken will extend the ttl of a token owned by a harvester cloud credential to infinite, as well as
// handle deletion of tokens when the cloud credential secret is deleted. Under normal circumstances, kubeconfigs
// tokens are subject to the settings.KubeconfigDefaultTokenTTLMinutes setting, which denotes a max ttl of default 30
// days. This does not make for a particularly satisfying UX for administrators of Harvester provisioned node driver
// clusters, as cloud credentials require manual rotation or be subject to failures during machine creation/deletion.
// Deletion is managed by this handler via wrangler's apply mechanism, which is used to indicate the secret is the
// owner of the token, sans owner references. A finalizer is applied to the secret, which will be used to delete the
// now infinitely lived tokens when the cloud credential itself is deleted, preventing a resource leak within the
// local cluster.
func (c *Controller) syncHarvesterToken(key string, secret *v1.Secret) (*v1.Secret, error) {
	if secret == nil || secret.Namespace != namespace.GlobalNamespace {
		return secret, nil
	}

	if len(secret.Data["harvestercredentialConfig-kubeconfigContent"]) == 0 {
		// not a (valid) harvester cloud credential
		return secret, nil
	}

	var err error

	d := sha256.Sum256(secret.Data["harvestercredentialConfig-kubeconfigContent"])
	checksum := hex.EncodeToString(d[:])

	if secret.DeletionTimestamp == nil && secret.Annotations[harvesterCloudCredentialTokenChecksumAnnotation] == checksum {
		// token TTL has already been extended and tokens have not been rotated
		return secret, nil
	} else if secret.DeletionTimestamp != nil && !slices.Contains(secret.Finalizers, harvesterCloudCredentialFinalizer) {
		// token TTL was never extended
		return secret, nil
	}

	objs := make([]runtime.Object, 0)
	if secret.DeletionTimestamp == nil {
		if !slices.Contains(secret.Finalizers, harvesterCloudCredentialFinalizer) {
			secret = secret.DeepCopy()
			secret.Finalizers = append(secret.Finalizers, harvesterCloudCredentialFinalizer)

			// remove now defunct expiration annotation if present
			if secret.Annotations[harvesterCloudCredentialExpirationAnnotation] != "" {
				delete(secret.Annotations, harvesterCloudCredentialExpirationAnnotation)
			}

			secret, err = c.secretClient.Update(secret)
			if err != nil {
				return nil, fmt.Errorf("unable to update harvester cloud credential secret %s: %w", key, err)
			}
		}

		// in practice a kubeconfig will only ever have one token, but we need to handle the case where users may be
		// modifying the secret directly and properly extend/delete tokens as necessary.
		tokenNames := make(cred.Set[string])
		err := cred.TokenNamesFromContent(secret.Data["harvestercredentialConfig-kubeconfigContent"], tokenNames)
		if err != nil {
			return nil, err
		}

		for tokenName := range tokenNames {
			token, err := c.tokenClient.Get(tokenName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("unable to get token %s for harvester cloud credential secret %s: %w", tokenName, key, err)
			}

			token = token.DeepCopy()
			token.TTLMillis = 0
			// although the token is updated here, the resource version must be empty because apply expects to create the token
			token.ResourceVersion = ""
			objs = append(objs, token)
		}
	}

	err = c.apply.WithOwner(secret).ApplyObjects(objs...)
	if err != nil {
		return nil, fmt.Errorf("unable to update harvester cloud credential secret %s: %w", key, err)
	}

	if secret.DeletionTimestamp == nil {
		secret = secret.DeepCopy()
		secret.Annotations[harvesterCloudCredentialTokenChecksumAnnotation] = checksum
		secret, err = c.secretClient.Update(secret)
		if err != nil {
			return nil, fmt.Errorf("unable to update harvester cloud credential secret %s: %w", key, err)
		}
	} else {
		secret = secret.DeepCopy()
		secret.Finalizers = removeFromSlice(secret.Finalizers, harvesterCloudCredentialFinalizer)
		secret, err = c.secretClient.Update(secret)
		if err != nil {
			return nil, fmt.Errorf("unable to update harvester cloud credential secret %s: %w", key, err)
		}
	}
	return secret, nil
}

// removeFromSlice removes the first element from the slice that is equal to e.
// If the slice does not contain e, the slice is returned unchanged.
// This function is written in the styles of the generic functions from slices.go
func removeFromSlice[S ~[]E, E comparable](s S, e E) S {
	i := slices.Index(s, e)
	if i == -1 {
		return s
	}
	return append(s[:i], s[i+1:]...)
}

func (c *Controller) ccSync(_ string, cloudCredential *v1.Secret) (runtime.Object, error) {
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
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return cloudCredential, fmt.Errorf("cloud credential %v has no creatorId annotation", cloudCredential.Name)
	}
	if err := rbac.CreateRoleAndRoleBinding(
		rbac.CloudCredentialResource, typesv1.SecretResource.Kind, cloudCredential.Name, namespace.GlobalNamespace, "v1", creatorID, []string{"*"}, cloudCredential.UID, []apimgmtv3.Member{},
		c.managementContext); err != nil {
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
