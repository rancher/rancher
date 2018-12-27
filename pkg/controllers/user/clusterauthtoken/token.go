package clusterauthtoken

import (
	"reflect"
	"sort"

	"github.com/rancher/rancher/pkg/controllers/user/clusterauthtoken/common"
	clusterv3 "github.com/rancher/types/apis/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
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
}

type TokenHandler struct {
	namespace                  string
	clusterName                string
	clusterAuthToken           clusterv3.ClusterAuthTokenInterface
	clusterAuthTokenLister     clusterv3.ClusterAuthTokenLister
	clusterUserAttribute       clusterv3.ClusterUserAttributeInterface
	clusterUserAttributeLister clusterv3.ClusterUserAttributeLister
	tokenIndexer               cache.Indexer
	userLister                 managementv3.UserLister
	userAttributeLister        managementv3.UserAttributeLister
}

func (h *TokenHandler) Create(token *managementv3.Token) (runtime.Object, error) {
	if token.ClusterName != h.clusterName {
		return nil, nil
	}

	_, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if !errors.IsNotFound(err) {
		return h.Updated(token)
	}

	err = h.updateClusterUserAttribute(token)
	if err != nil {
		return nil, err
	}

	clusterAuthToken, err := common.NewClusterAuthToken(token)
	if err != nil {
		return nil, err
	}

	_, err = h.clusterAuthToken.Create(clusterAuthToken)
	return nil, err
}

func (h *TokenHandler) Updated(token *managementv3.Token) (runtime.Object, error) {
	if token.ClusterName != h.clusterName {
		return nil, nil
	}

	clusterAuthToken, err := h.clusterAuthTokenLister.Get(h.namespace, token.Name)
	if errors.IsNotFound(err) {
		return h.Create(token)
	}
	if err != nil {
		return nil, err
	}

	err = h.updateClusterUserAttribute(token)
	if err != nil {
		return nil, err
	}

	tokenEnabled := token.Enabled == nil || *token.Enabled
	new := tokenAttributeCompare{
		enabled:   tokenEnabled,
		expiresAt: token.ExpiresAt,
		username:  token.UserID,
	}
	old := tokenAttributeCompare{
		enabled:   clusterAuthToken.Enabled,
		expiresAt: clusterAuthToken.ExpiresAt,
		username:  clusterAuthToken.UserName,
	}
	if reflect.DeepEqual(new, old) {
		return nil, nil
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

func (h *TokenHandler) Remove(token *managementv3.Token) (runtime.Object, error) {
	if token.ClusterName != h.clusterName {
		return nil, nil
	}

	tokens, err := h.tokenIndexer.ByIndex(tokenByUserAndClusterIndex, tokenUserClusterKey(token))
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(tokens) == 1 {
		lastToken := tokens[0].(*managementv3.Token)
		if token.Name == lastToken.Name {
			// we are about to remove the last token for this user & cluster,
			// also remove user data from cluster
			err := h.clusterUserAttribute.Delete(token.UserID, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Error(err)
			}
		}
	}
	err = h.clusterAuthToken.Delete(token.Name, &metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return nil, err
}

func (h *TokenHandler) updateClusterUserAttribute(token *managementv3.Token) error {
	userID := token.UserID
	user, err := h.userLister.Get("", userID)
	if err != nil {
		return err
	}

	userAttribute, err := h.userAttributeLister.Get("", userID)
	if err != nil {
		return err
	}

	groups := []string{}
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

	new := userAttributeCompare{
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

	if reflect.DeepEqual(new, old) {
		return nil
	}

	clusterUserAttribute.Groups = groups
	clusterUserAttribute.Enabled = userEnabled
	clusterUserAttribute.LastRefresh = userAttribute.LastRefresh
	clusterUserAttribute.NeedsRefresh = userAttribute.NeedsRefresh

	_, err = h.clusterUserAttribute.Update(clusterUserAttribute)
	return err
}
