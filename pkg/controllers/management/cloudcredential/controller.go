package cloudcredential

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"slices"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	typesv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/apply"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	harvesterCloudCredentialTokenChecksumAnnotation = "management.cattle.io/harvester-token-checksum"
	harvesterCloudCredentialFinalizer               = "management.cattle.io/harvester-token-cleanup"
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
// handle deletion when the cloud credential secret is deleted.
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
			secret, err = c.secretClient.Update(secret)
			if err != nil {
				return nil, fmt.Errorf("unable to update harvester cloud credential secret %s: %w", key, err)
			}
		}

		// in practice a kubeconfig will only ever have one token, but we need to handle the case where users may be
		// modifying the secret directly and properly extend/delete tokens as necessary.
		tokenNames, err := c.tokensFromHarvesterCloudCredential(secret)
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

type set[E comparable] = map[E]struct{}

func (c *Controller) tokensFromHarvesterCloudCredential(secret *v1.Secret) (set[string], error) {
	var kcc []byte
	if kcc = secret.Data["harvestercredentialConfig-kubeconfigContent"]; len(kcc) == 0 {
		// not a (valid) harvester cloud credential
		return nil, fmt.Errorf("secret %s/%s is not a harvester cloud credential", secret.Namespace, secret.Name)
	}

	var kc store.KubeConfig
	err := yaml.Unmarshal(kcc, &kc)
	if err != nil {
		return nil, err
	}

	tokens := make(set[string])

	for _, user := range kc.Users {
		if !strings.HasPrefix(user.User.Token, "kubeconfig-u-") && !strings.HasPrefix(user.User.Token, "kubeconfig-user-") {
			continue
		}
		tokenName, _, found := strings.Cut(user.User.Token, ":")
		if !found {
			return nil, fmt.Errorf("unable to get token name from token for harvester cloud credential secret %s/%s", secret.Namespace, secret.Name)
		}
		tokens[tokenName] = struct{}{}
	}
	return tokens, nil
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
