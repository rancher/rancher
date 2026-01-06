package clusterauthtoken

import (
	"fmt"
	"reflect"
	"sort"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken/common"
	"github.com/rancher/rancher/pkg/features"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type tokenAttributeCompare struct {
	username  string
	expiresAt string
	enabled   bool
	value     string
}

type tokenHandler struct {
	namespace                  string
	clusterAuthToken           clusterv3.ClusterAuthTokenInterface
	clusterAuthTokenLister     clusterv3.ClusterAuthTokenLister
	clusterUserAttribute       clusterv3.ClusterUserAttributeInterface
	clusterUserAttributeLister clusterv3.ClusterUserAttributeLister
	tokenIndexer               cache.Indexer
	extTokenIndexer            cache.Indexer
	userLister                 managementv3.UserLister
	userAttributeLister        managementv3.UserAttributeLister
	clusterSecret              wcore.SecretClient
	clusterSecretLister        wcore.SecretCache
}

// extCreate is called when a given ext token is created, and is responsible for
// updating/creating the ClusterAuthToken in the downstream cluster.
func (h *tokenHandler) extCreate(token *extv1.Token) (*extv1.Token, error) {
	logrus.Debugf("[%s] ext CREATE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.Spec.ClusterName)

	_, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if !errors.IsNotFound(err) {
		return h.ExtUpdated(token)
	}

	// we can sync tokens which are hashed by copying the hash downstream
	// token is hashed, we can safely attempt to sync downstream
	hashVersion, err := hashers.GetHashVersion(token.Status.Hash)
	if err != nil {
		// the token hash is unlikely to change, re-enqueing would just produce a flood of errors
		logrus.Errorf("unable to determine hash version of token [%s], will not sync token: %s", token.Name, err.Error())
		return token, generic.ErrSkip
	}
	// we only sync tokens downstream that were created with SHA3
	if hashVersion == hashers.SHA3Version {
		return nil, h.createClusterAuthToken(token, token.Status.Hash)
	}

	// token is hashed, but we can't sync it since we don't have the raw value
	logrus.Warnf("token [%s] will not be synced or useable for ACE because it uses an older hash version, generate a new token to use ACE", token.Name)
	// don't re-enqueue, we can't sync this token
	return nil, generic.ErrSkip
}

// ExtUpdated is called when a given ext token is modified, and is responsible
// for updating/creating the ClusterAuthToken in a downstream cluster.
func (h *tokenHandler) ExtUpdated(token *extv1.Token) (*extv1.Token, error) {
	logrus.Debugf("[%s] ext UPDATE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.Spec.ClusterName)

	clusterAuthToken, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if errors.IsNotFound(err) {
		return h.extCreate(token)
	}
	if err != nil {
		return nil, err
	}

	forced := false
	clusterAuthTokenSecret, err := h.clusterSecretLister.Get(h.namespace, common.ClusterAuthTokenSecretName(token.Name))
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// While the cluster auth token exists, the associated secret is
		// missing. Make it now, and force an update later.

		forced = true
		hashedValue := token.Status.Hash
		clusterAuthTokenSecret = common.NewClusterAuthTokenSecret(h.namespace, token, hashedValue)
	}

	err = h.updateClusterUserAttribute(token.GetUserID())
	if err != nil {
		return nil, err
	}

	tokenEnabled := token.GetIsEnabled()
	current := tokenAttributeCompare{
		enabled:   tokenEnabled,
		expiresAt: token.Status.ExpiresAt,
		username:  token.Spec.UserID,
	}
	old := tokenAttributeCompare{
		enabled:   clusterAuthToken.Enabled,
		expiresAt: clusterAuthToken.ExpiresAt,
		username:  clusterAuthToken.UserName,
	}

	// note: ext tokens are always hashed (contrary to v3 Tokens)
	hashVersion, err := hashers.GetHashVersion(token.Status.Hash)
	if err != nil {
		logrus.Errorf("unable to determine hash version of token [%s], will not sync token: %s", token.Name, err.Error())
		return token, generic.ErrSkip
	}
	// we only sync tokens downstream that were created with SHA3
	if hashVersion == hashers.SHA3Version {
		// trigger the compare to compare the values of the tokens
		current.value = token.Status.Hash
		old.value = common.ClusterAuthTokenSecretValue(clusterAuthTokenSecret)
	}

	if forced {
		current.value = common.ClusterAuthTokenSecretValue(clusterAuthTokenSecret)
	}

	if reflect.DeepEqual(current, old) {
		return nil, nil
	}

	// if we were comparing token values, then the token was hashed, so we can update the value downstream
	if current.value != "" {
		clusterAuthTokenSecret.Data["hash"] = []byte(current.value)
		_, err = h.clusterSecret.Update(clusterAuthTokenSecret)
		if errors.IsNotFound(err) {
			_, err = h.clusterSecret.Create(clusterAuthTokenSecret)
		}
	}

	clusterAuthToken.UserName = token.Spec.UserID
	clusterAuthToken.Enabled = tokenEnabled
	clusterAuthToken.ExpiresAt = token.Status.ExpiresAt

	_, err = h.clusterAuthToken.Update(clusterAuthToken)
	if errors.IsNotFound(err) {
		_, err = h.clusterAuthToken.Create(clusterAuthToken)
	}
	return nil, err
}

// ExtRemove is called when a given ext token is deleted,
// and removes the ClusterAuthToken in the downstream cluster.
func (h *tokenHandler) ExtRemove(token *extv1.Token) (*extv1.Token, error) {
	logrus.Debugf("[%s] ext REMOVE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.Spec.ClusterName)

	return nil, h.remove(token.GetName(), token.GetUserID(), extTokenUserClusterKey(token))
}

// Create is called when a given token is created, and is responsible for creating a ClusterAuthToken in a downstream cluster.
func (h *tokenHandler) Create(token *managementv3.Token) (runtime.Object, error) {
	logrus.Debugf("[%s] v3 CREATE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.ClusterName)

	_, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if !errors.IsNotFound(err) {
		return h.Updated(token)
	}

	if features.TokenHashing.Enabled() {
		// we can sync tokens which are hashed by copying the hash downstream
		if token.Annotations[tokens.TokenHashed] != "true" {
			// re-enqueue until the token has been hashed
			return token, fmt.Errorf("token [%s] has not been hashed yet, re-enqueing until has has completed", token.Name)
		}
		// token is hashed, we can safely attempt to sync downstream
		hashVersion, err := hashers.GetHashVersion(token.Token)
		if err != nil {
			// the token hash is unlikely to change, re-enqueing would just produce a flood of errors
			logrus.Errorf("unable to determine hash version of token [%s], will not sync token: %s", token.Name, err.Error())
			return token, generic.ErrSkip
		}
		// we only sync tokens downstream that were created with SHA3
		if hashVersion == hashers.SHA3Version {
			return nil, h.createClusterAuthToken(token, token.Token)
		}
		// token is hashed, but we can't sync it since we don't have the raw value
		logrus.Warnf("token [%s] will not be synced or useable for ACE because it uses an older hash version, generate a new token to use ACE", token.Name)
		// don't re-enqueue, we can't sync this token
		return nil, generic.ErrSkip
	}

	// token isn't hashed, hash the value only for downstream
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	if err != nil {
		return nil, fmt.Errorf("unable to hash value for token [%s]: %w", token.Name, err)
	}
	return nil, h.createClusterAuthToken(token, hashedValue)
}

// createClusterAuthToken handles actions commonly taken to create a clusterAuthToken from a token.
func (h *tokenHandler) createClusterAuthToken(token accessor.TokenAccessor, hashedValue string) error {
	err := h.updateClusterUserAttribute(token.GetUserID())
	if err != nil {
		return err
	}

	clusterAuthToken := common.NewClusterAuthToken(token, hashedValue)
	clusterAuthTokenSecret := common.NewClusterAuthTokenSecret(h.namespace, token, hashedValue)

	// Creating the secret first, then the token for it. This ensures that
	// kube-api-auth either sees nothing, or a working combination of
	// resources. In case of trouble the partial set of resources (orphan
	// secret) is removed.

	clusterAuthTokenSecret, err = h.clusterSecret.Create(clusterAuthTokenSecret)
	if err != nil && errors.IsAlreadyExists(err) {
		// Overwrite an existing secret.
		_, err = h.clusterSecret.Update(clusterAuthTokenSecret)
	}

	if err != nil {
		return err
	}

	// Now create the shadow token.
	clusterAuthToken, err = h.clusterAuthToken.Create(clusterAuthToken)
	if err == nil {
		return nil
	}

	// Avoid leaving partially created resources.
	if err := h.clusterAuthToken.Delete(clusterAuthToken.Name, &metav1.DeleteOptions{}); err != nil {
		logrus.Errorf("failed to delete cluster auth token `%s` after creation failure: %v",
			clusterAuthToken.Name, err)
	}

	// Report creation failure
	return err
}

// Updated is called when a token is updated, and is responsible for creating/updating the corresponding
// ClusterAuthTokens in the downstream cluster.
func (h *tokenHandler) Updated(token *managementv3.Token) (runtime.Object, error) {
	logrus.Debugf("[%s] v3 UPDATE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.ClusterName)

	clusterAuthToken, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if errors.IsNotFound(err) {
		return h.Create(token)
	}
	if err != nil {
		return nil, err
	}

	forced := false
	clusterAuthTokenSecret, err := h.clusterSecretLister.Get(h.namespace, common.ClusterAuthTokenSecretName(token.Name))
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// While the cluster auth token exists, the associated secret is
		// missing. Make it now, and force an update later.

		forced = true
		hashedValue := token.Token
		if token.Annotations[tokens.TokenHashed] != "true" {
			hasher := hashers.GetHasher()
			hashed, err := hasher.CreateHash(token.Token)
			if err != nil {
				return nil, fmt.Errorf("unable to hash value for token [%s]: %w", token.Name, err)
			}
			hashedValue = hashed
		}

		clusterAuthTokenSecret = common.NewClusterAuthTokenSecret(h.namespace, token, hashedValue)
	}

	err = h.updateClusterUserAttribute(token.GetUserID())
	if err != nil {
		return nil, err
	}

	tokenEnabled := token.Enabled == nil || *token.Enabled
	current := tokenAttributeCompare{
		enabled:   tokenEnabled,
		expiresAt: token.ExpiresAt,
		username:  token.UserID,
	}
	old := tokenAttributeCompare{
		enabled:   clusterAuthToken.Enabled,
		expiresAt: clusterAuthToken.ExpiresAt,
		username:  clusterAuthToken.UserName,
	}

	// if the token is hashed, compare its value to make sure the downstream has the latest hash
	//
	// BEWARE! for an unhashed token a comparison here is bogus. the downstream hash was
	// made on creation and any hash we make here to compare with will be different from
	// it due to the random salt!
	if token.Annotations[tokens.TokenHashed] == "true" {
		hashVersion, err := hashers.GetHashVersion(token.Token)
		if err != nil {
			logrus.Errorf("unable to determine hash version of token [%s], will not sync token: %s", token.Name, err.Error())
			return token, generic.ErrSkip
		}
		// we only sync tokens downstream that were created with SHA3
		if hashVersion == hashers.SHA3Version {
			// trigger the compare to compare the values of the tokens
			current.value = token.Token
			old.value = common.ClusterAuthTokenSecretValue(clusterAuthTokenSecret)
		}
	}

	if forced {
		current.value = common.ClusterAuthTokenSecretValue(clusterAuthTokenSecret)
	}

	if !forced && reflect.DeepEqual(current, old) {
		return nil, nil
	}

	// If we were comparing token values, then the token was hashed, so we can update the value in the downstream.
	if current.value != "" {
		clusterAuthTokenSecret.Data["hash"] = []byte(current.value)
		_, err = h.clusterSecret.Update(clusterAuthTokenSecret)
		if errors.IsNotFound(err) {
			_, err = h.clusterSecret.Create(clusterAuthTokenSecret)
			if err != nil {
				return nil, err
			}
		}
	}

	clusterAuthToken.UserName = token.UserID
	clusterAuthToken.Enabled = tokenEnabled
	clusterAuthToken.ExpiresAt = token.ExpiresAt

	_, err = h.clusterAuthToken.Update(clusterAuthToken)
	if errors.IsNotFound(err) {
		_, err = h.clusterAuthToken.Create(clusterAuthToken)
	}

	return nil, err
}

func (h *tokenHandler) Remove(token *managementv3.Token) (runtime.Object, error) {
	logrus.Debugf("[%s] v3 REMOVE FOR %q INTO %q", clusterAuthTokenController, token.Name, token.ClusterName)

	return nil, h.remove(token.GetName(), token.GetUserID(), tokenUserClusterKey(token))
}

func (h *tokenHandler) remove(name, userID, key string) error {
	tokens, err := h.tokenIndexer.ByIndex(tokenByUserAndClusterIndex, key)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// skip ext tokens if feature is disabled
	var extTokens []any
	if h.extTokenIndexer != nil {
		extTokens, err = h.extTokenIndexer.ByIndex(tokenByUserAndClusterIndex, key)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	err = h.clusterSecret.Delete(h.namespace, common.ClusterAuthTokenSecretName(name), &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = h.clusterAuthToken.Delete(name, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if len(tokens)+len(extTokens) > 1 {
		return nil
	}

	var lastName string
	if len(tokens) == 1 {
		lastName = tokens[0].(*managementv3.Token).Name
	} else if len(extTokens) == 1 {
		lastName = extTokens[0].(*extv1.Token).Name
	}

	if name == lastName {
		// we are about to remove the last token for this user & cluster,
		// also remove user data from cluster
		err = h.clusterUserAttribute.Delete(userID, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (h *tokenHandler) updateClusterUserAttribute(userID string) error {
	user, err := h.userLister.Get("", userID)
	if err != nil {
		return err
	}

	userAttribute, err := h.userAttributeLister.Get("", userID)
	if err != nil {
		return err
	}

	var groups []string
	for _, gp := range userAttribute.GroupPrincipals {
		for i := range gp.Items {
			groups = append(groups, gp.Items[i].Name)
		}
	}
	sort.Strings(groups)

	userEnabled := user.Enabled == nil || *user.Enabled

	clusterUserAttribute, err := h.clusterUserAttributeLister.Get(h.namespace, userID)
	if errors.IsNotFound(err) {
		_, err = h.clusterUserAttribute.Create(&clusterv3.ClusterUserAttribute{
			ObjectMeta: metav1.ObjectMeta{
				Name: userID,
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterUserAttribute",
			},
			Groups:       groups,
			Enabled:      userEnabled,
			LastRefresh:  userAttribute.LastRefresh,
			NeedsRefresh: userAttribute.NeedsRefresh,
		})
		return err
	}
	if err != nil {
		return err
	}

	current := userAttributeCompare{
		groups:       groups,
		lastRefresh:  userAttribute.LastRefresh,
		needsRefresh: userAttribute.NeedsRefresh,
		enabled:      userEnabled,
	}
	old := userAttributeCompare{
		groups:       clusterUserAttribute.Groups,
		lastRefresh:  clusterUserAttribute.LastRefresh,
		needsRefresh: clusterUserAttribute.NeedsRefresh,
		enabled:      clusterUserAttribute.Enabled,
	}

	if reflect.DeepEqual(current, old) {
		return nil
	}
	clusterUserAttribute = clusterUserAttribute.DeepCopy()
	clusterUserAttribute.Groups = groups
	clusterUserAttribute.Enabled = userEnabled
	clusterUserAttribute.LastRefresh = userAttribute.LastRefresh
	clusterUserAttribute.NeedsRefresh = userAttribute.NeedsRefresh

	_, err = h.clusterUserAttribute.Update(clusterUserAttribute)
	return err
}
