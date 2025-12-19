package cred

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/api/norman/customization/namespacedresource"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"
)

func Wrap(store types.Store, ns v1.NamespaceInterface, secretLister v1.SecretLister, provClusterCache provv1.ClusterCache, tokenLister v3.TokenLister) types.Store {
	transformStore := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			if configExists(data) {
				data["type"] = "cloudCredential"
				if err := decodeNonPasswordFields(data); err != nil {
					return nil, err
				}
				return data, nil
			}
			return nil, nil
		},
	}

	newStore := &Store{
		Store:            transformStore,
		SecretLister:     secretLister,
		ProvClusterCache: provClusterCache,
		TokenLister:      tokenLister,
	}

	return namespacedresource.Wrap(newStore, ns, namespace.GlobalNamespace)
}

func configExists(data map[string]interface{}) bool {
	for key, val := range data {
		if strings.HasSuffix(key, "Config") {
			if convert.ToString(val) != "" {
				return true
			}
		}
	}
	return false
}

func decodeNonPasswordFields(data map[string]interface{}) error {
	for key, val := range data {
		if strings.HasSuffix(key, "Config") {
			ans := convert.ToMapInterface(val)
			for field, value := range ans {
				decoded, err := base64.StdEncoding.DecodeString(convert.ToString(value))
				if err != nil {
					return err
				}
				ans[field] = string(decoded)
			}
		}
	}
	return nil
}

func Validator(_ *types.APIContext, _ *types.Schema, data map[string]interface{}) error {
	if !configExists(data) {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}

	return nil
}

type Store struct {
	types.Store
	SecretLister     v1.SecretLister
	ProvClusterCache provv1.ClusterCache
	TokenLister      v3.TokenLister
}

type Set[E comparable] = map[E]struct{}

func TokenNamesFromContent(content []byte, tokens Set[string]) error {
	var kc store.KubeConfig
	err := yaml.Unmarshal(content, &kc)
	if err != nil {
		return fmt.Errorf("failed to unmarshal kubeconfig: %w", err)
	}

	for _, user := range kc.Users {
		if !strings.HasPrefix(user.User.Token, "kubeconfig-u-") && !strings.HasPrefix(user.User.Token, "kubeconfig-user-") {
			continue
		}
		tokenName, _, found := strings.Cut(user.User.Token, ":")
		if !found {
			return errors.New("unable to get token name from content")
		}
		tokens[tokenName] = struct{}{}
	}

	return nil
}

func (s *Store) processHarvesterCloudCredential(data map[string]any) error {
	if hc, ok := data["harvestercredentialConfig"].(map[string]any); ok && hc != nil {
		if kubeconfigYaml, ok := hc["kubeconfigContent"].(string); ok && kubeconfigYaml != "" {
			tokens := make(Set[string])
			err := TokenNamesFromContent([]byte(kubeconfigYaml), tokens)
			if err != nil {
				return fmt.Errorf("failed to get tokens from kubeconfig: %w", err)
			}

			for tokenName := range tokens {
				token, err := s.TokenLister.Get("", tokenName)
				if err != nil {
					return fmt.Errorf("failed to get token: %w", err)
				} else if token.Expired {
					return fmt.Errorf("token %s is expired", tokenName)
				}
			}

			secrets, err := s.SecretLister.List("cattle-global-data", labels.NewSelector())
			if err != nil {
				return fmt.Errorf("failed to list secrets: %w", err)
			}

			knownTokens := make(Set[string])
			for _, secret := range secrets {
				if len(secret.Data["harvestercredentialConfig-kubeconfigContent"]) == 0 {
					continue
				}

				err := TokenNamesFromContent(secret.Data["harvestercredentialConfig-kubeconfigContent"], knownTokens)
				if err != nil {
					// If a secret is all messed up, let the user do whatever they want: it shouldn't be usable anyway and will be remediated when updated.
					logrus.Errorf("failed to get tokens from secret: %v", err)
					continue
				}
			}

			for tokenName := range tokens {
				if _, ok := knownTokens[tokenName]; ok {
					return fmt.Errorf("token %s is already in use by secret", tokenName)
				}
			}
		}
	}
	return nil
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.processHarvesterCloudCredential(data); err != nil {
		return nil, fmt.Errorf("failed to process harvester cloud credential: %w", err)
	}

	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.processHarvesterCloudCredential(data); err != nil {
		return nil, fmt.Errorf("failed to process harvester cloud credential: %w", err)
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	// make sure the credential isn't being used by an active RKE2/K3s cluster
	if provClusters, err := s.ProvClusterCache.GetByIndex(cluster.ByCloudCred, id); err != nil {
		return nil, httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("An error was encountered while attempting to delete the cloud credential: %s", err))
	} else if len(provClusters) > 0 {
		return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Cloud credential is currently referenced by provisioning cluster %s", provClusters[0].Name))
	}

	// make sure the credential isn't being used by an active machine pool within a RKE2/K3s cluster
	if provClusters, err := s.ProvClusterCache.GetByIndex(cluster.ByMachinePoolCloudCred, id); err != nil {
		return nil, httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("An error was encountered while attempting to delete the cloud credential: %s", err))
	} else if len(provClusters) > 0 {
		return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Cloud credential is currently referenced by provisioning cluster %s", provClusters[0].Name))
	}

	return s.Store.Delete(apiContext, schema, id)
}
