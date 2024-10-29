package cred

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/norman/customization/namespacedresource"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
)

const CloudCredentialExpirationAnnotation = "rancher.io/expiration-timestamp"

func Wrap(store types.Store, ns v1.NamespaceInterface, nodeTemplateLister v3.NodeTemplateLister, provClusterCache provv1.ClusterCache, tokenLister v3.TokenLister) types.Store {
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
		Store:              transformStore,
		NodeTemplateLister: nodeTemplateLister,
		ProvClusterCache:   provClusterCache,
		TokenLister:        tokenLister,
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

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if !configExists(data) {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}

	return nil
}

type Store struct {
	types.Store
	NodeTemplateLister v3.NodeTemplateLister
	ProvClusterCache   provv1.ClusterCache
	TokenLister        v3.TokenLister
}

// Create will add an expiration timestamp in milliseconds since Unix Epoch as a CloudCredentialExpirationAnnotation if
// the credential is a harvester credential based on kubeconfig with a configured token, and then create the credential.
func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.processHarvesterCloudCredentialExpiration(data, false); err != nil {
		return nil, fmt.Errorf("failed to process harvester cloud credential expiration: %w", err)
	}

	return s.Store.Create(apiContext, schema, data)
}

// Update will add an expiration timestamp in milliseconds since Unix Epoch as a CloudCredentialExpirationAnnotation if
// the credential is a harvester credential based on kubeconfig with a configured token, as well as update the existing
// token if the timestamp has changed (including removal of the existing annotation )and then create the credential.
func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.processHarvesterCloudCredentialExpiration(data, true); err != nil {
		return nil, fmt.Errorf("failed to process harvester cloud credential expiration: %w", err)
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

	// make sure the cloud credential isn't being used by an RKE1 node template
	// which may be used by an active cluster
	nodeTemplates, err := s.NodeTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return nil, err
	}
	if len(nodeTemplates) > 0 {
		for _, template := range nodeTemplates {
			if template.Spec.CloudCredentialName != id {
				continue
			}
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("Cloud credential is currently referenced by node template %s", template.Name))
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

// processHarvesterCloudCredentialExpiration will extract the kubeconfig from a harvester cloud credential, extract the
// expiration in milliseconds from the associated token, and set the CloudCredentialExpirationAnnotation on the
// secret, removing on update if not present. Cloud credentials are transformed by norman, and thus the secret key of
// `harvestercredentialConfig-kubeconfigContent` is unrolled into a top level map with `kubeconfigContent` as a nested
// element within that map (see: decodeNonPasswordFields). If remove is false, the existing annotation (if present) is
// not removed. If remove is true, the existing annotation will only be removed if the associated token does not have a
// valid expiration. If the credential is not a harvester credential, this function does nothing.
func (s *Store) processHarvesterCloudCredentialExpiration(data map[string]any, remove bool) error {
	if hc, ok := data["harvestercredentialConfig"].(map[string]any); ok && hc != nil {
		if kubeconfigYaml, ok := hc["kubeconfigContent"].(string); ok && kubeconfigYaml != "" {

			expiration, err := GetHarvesterCloudCredentialExpirationFromKubeconfig(kubeconfigYaml, func(tokenName string) (*apimgmtv3.Token, error) {
				return s.TokenLister.Get("", tokenName)
			})
			if err != nil {
				return fmt.Errorf("failed to get harvester cloud credential expiration from kubeconfig: %w", err)
			}

			if expiration != "" {
				values.PutValue(data, expiration, "annotations", CloudCredentialExpirationAnnotation)
			} else if remove {
				values.RemoveValue(data, "annotations", CloudCredentialExpirationAnnotation)
			}
		}
	}
	return nil
}

// GetHarvesterCloudCredentialExpirationFromKubeconfig extracts the name of the associated token from the kubeconfig,
// gets the associated token and returns its `ExpiresAt` field as milliseconds since Unix Epoch. If the kubeconfig does
// not have an associated token or if `ExpiresAt` is not set on the token, this function will return an empty string
// with no error, indicating that expiration is not valid within the context of this cloud credential.
func GetHarvesterCloudCredentialExpirationFromKubeconfig(kubeconfigYaml string, getToken func(string) (*apimgmtv3.Token, error)) (string, error) {
	tokenName, err := getHarvesterCredentialTokenNameFromKubeconfig(kubeconfigYaml)
	if err != nil {
		return "", fmt.Errorf("failed to get harvester credential config token name: %w", err)
	}

	if tokenName != "" {
		token, err := getToken(tokenName)
		if err != nil {
			return "", fmt.Errorf("failed to get harvester credential config token: %w", err)
		}

		if ms, err := getMillisecondsUntilTokenExpiration(token); err != nil {
			return "", fmt.Errorf("failed to get harvester credential config token expiration: %w", err)
		} else if ms != nil {
			return strconv.FormatInt(*ms, 10), nil
		}
	}

	return "", nil
}

// getHarvesterCredentialTokenNameFromKubeconfig parses a kubeconfig for a user with a token matching the prefix
// "kubeconfig-user-", and will return the name of the token if found. If a satisfying token cannot be found no error is
// returned, but an empty string will be returned to indicate this kubeconfig is not configured with an associated
// token.
func getHarvesterCredentialTokenNameFromKubeconfig(kubeconfigYaml string) (string, error) {
	kubeConfig := map[string]any{}
	if err := yaml.Unmarshal([]byte(kubeconfigYaml), &kubeConfig); err != nil {
		return "", err
	}

	if userList, ok := kubeConfig["users"].([]any); ok && userList != nil && len(userList) > 0 {
		for _, u := range userList {
			if entry, ok := u.(map[string]any); ok && entry != nil {
				if user, ok := entry["user"].(map[string]any); ok && user != nil {
					if token, ok := user["token"].(string); ok && token != "" {
						if strings.HasPrefix(token, "kubeconfig-user-") {
							token, _, _ = strings.Cut(token, ":")
							return token, nil
						}
					}
				}
			}
		}
	}
	return "", nil
}

// getMillisecondsUntilTokenExpiration will transform the `ExpiresAt` field from one matching the time.RFC3339 format to
// milliseconds since Unix Epoch. If the token does not expire, this function return nil, nil to indicate expiration is
// not applicable to this token.
func getMillisecondsUntilTokenExpiration(token *apimgmtv3.Token) (*int64, error) {
	if token.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse token expiration time: %w", err)
		}
		return ptr.To(t.UnixMilli()), nil
	}
	return nil, nil
}
