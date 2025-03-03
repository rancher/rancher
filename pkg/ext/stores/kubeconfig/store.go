package kubeconfig

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	ctrlv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	kconfig "github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	extapi "github.com/rancher/steve/pkg/ext"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/util/retry"
)

const (
	Kind                     = "Kubeconfig"
	Singular                 = "kubeconfig"
	GroupCattleAuthenticated = "system:cattle:authenticated"
)

var gvr = ext.SchemeGroupVersion.WithResource(ext.KubeconfigResourceName)

type userManager interface {
	EnsureClusterToken(clusterID string, input user.TokenInput) (string, error)
	EnsureToken(input user.TokenInput) (string, error)
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
// Store implements storage for [ext.Kubeconfig], which is an ephemeral resource.
// It only supports create, get, list, and delete operations.
type Store struct {
	authorizer          authorizer.Authorizer
	userCache           ctrlv3.UserCache
	clusterCache        ctrlv3.ClusterCache
	nodeCache           ctrlv3.NodeCache
	tokenCache          ctrlv3.TokenCache
	tokens              ctrlv3.TokenClient
	userMgr             userManager
	getDefaultTTL       func() (*int64, error)
	getServerURL        func() string
	shouldGenerateToken func() bool
}

// New creates a new instance of [Store].
func New(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer, userMgr user.Manager) *Store {
	return &Store{
		userCache:     wranglerContext.Mgmt.User().Cache(),
		clusterCache:  wranglerContext.Mgmt.Cluster().Cache(),
		nodeCache:     wranglerContext.Mgmt.Node().Cache(),
		tokenCache:    wranglerContext.Mgmt.Token().Cache(),
		tokens:        wranglerContext.Mgmt.Token(),
		userMgr:       userMgr,
		authorizer:    authorizer,
		getServerURL:  settings.ServerURL.Get,
		getDefaultTTL: tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds,
		shouldGenerateToken: func() bool {
			return strings.EqualFold(settings.KubeconfigGenerateToken.Get(), "true")
		},
	}
}

// isUnique returns true if the given slice of strings contains unique values.
func isUnique(ids []string) bool {
	set := make(map[string]struct{}, len(ids))

	for _, id := range ids {
		if _, ok := set[id]; ok {
			return false
		}
		set[id] = struct{}{}
	}

	return true
}

// New implements [rest.Creater]
func (s *Store) New() runtime.Object {
	return &ext.Kubeconfig{}
}

// userFrom is a helper that extracts and validates the user info from the request's context.
func (s *Store) userFrom(ctx context.Context) (k8suser.Info, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, false, fmt.Errorf("missing user info")
	}

	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "*",
		Resource:        "*",
		ResourceRequest: true,
	})
	if err != nil {
		return nil, false, false, err
	}

	isAdmin := decision == authorizer.DecisionAllow

	isRancherUser := false

	if name := userInfo.GetName(); !strings.Contains(name, ":") { // E.g. system:admin
		_, err := s.userCache.Get(name)
		if err == nil {
			isRancherUser = true
		} else if !apierrors.IsNotFound(err) {
			return nil, false, false, fmt.Errorf("error getting user %s: %w", name, err)
		}
	}

	return userInfo, isAdmin, isRancherUser, nil
}

// Create implements [rest.Creater]
func (s *Store) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	userInfo, _, isRancherUser, err := s.userFrom(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	if !isRancherUser {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userInfo.GetName()))
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

	authToken, err := s.tokenCache.Get(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	kubeconfig, ok := obj.(*ext.Kubeconfig)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", obj))
	}

	if len(kubeconfig.Spec.Clusters) == 0 {
		return nil, apierrors.NewBadRequest("spec.clusters is required")
	}

	if !isUnique(kubeconfig.Spec.Clusters) {
		return nil, apierrors.NewBadRequest("spec.clusters must be unique")
	}

	host := s.getServerURL()
	if host != "" {
		u, err := url.Parse(host)
		if err == nil {
			host = u.Host
		}
	}
	if host == "" {
		host = first(extras[common.ExtraRequestHost])
		if host == "" {
			return nil, apierrors.NewBadRequest("can't determine the server URL")
		}
	}

	var kubeConfigID string
	err = retry.OnError(retry.DefaultRetry, func(_ error) bool {
		return true // Retry all errors.
	}, func() error {
		kubeConfigID = names.SimpleNameGenerator.GenerateName("kubeconfig-")
		exists, err := s.exists(kubeConfigID)
		if err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("kubeconfig %s already exists", kubeConfigID)
		}

		return nil
	})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error checking if kubeconfig %s exists: %w", kubeConfigID, err))
	}

	var clusters []*apiv3.Cluster
	if kubeconfig.Spec.Clusters[0] == "*" {
		// The first id in the spec.clusters "*" means all clusters.
		clusters, err = s.clusterCache.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("error listing clusters: %w", err)
		}
	} else {
		// Individualy listed clusters.
		for _, clusterID := range kubeconfig.Spec.Clusters {
			cluster, err := s.clusterCache.Get(clusterID)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, apierrors.NewBadRequest(fmt.Sprintf("cluster %s not found", clusterID))
				}
				return nil, apierrors.NewInternalError(fmt.Errorf("error getting cluster %s: %w", clusterID, err))
			}

			clusters = append(clusters, cluster)
		}
	}

	defaultTokenTTL, err := s.getDefaultTTL()
	if err != nil {
		return nil, fmt.Errorf("failed to get default token TTL: %w", err)
	}

	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	generateToken := s.shouldGenerateToken()
	input := make([]kconfig.Input, 0, len(kubeconfig.Spec.Clusters))
	for _, cluster := range clusters {
		decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
			User:            userInfo,
			Verb:            "get",
			APIGroup:        mgmt.GroupName,
			Resource:        apiv3.ClusterResourceName,
			ResourceRequest: true,
			Name:            cluster.Name,
		})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error authorizing user %s to access cluster %s: %w", userInfo.GetName(), cluster.Name, err))
		}

		if decision != authorizer.DecisionAllow {
			return nil, apierrors.NewForbidden(gvr.GroupResource(), kubeConfigID, fmt.Errorf("user %s is not allowed to access cluster %s", userInfo.GetName(), cluster.Name))
		}

		var nodes []*apiv3.Node
		if cluster.Spec.LocalClusterAuthEndpoint.Enabled {
			nodes, err = s.nodeCache.List(cluster.Name, labels.Everything())
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error listing nodes for cluster %s: %w", cluster.Name, err))
			}
		}

		var tokenKey, sharedTokenKey string

		if !dryRun && generateToken {
			input := s.createTokenInput(kubeConfigID, userInfo.GetName(), authToken, defaultTokenTTL)
			if cluster.Spec.LocalClusterAuthEndpoint.Enabled {
				tokenKey, err = s.userMgr.EnsureClusterToken(cluster.Name, input)
			} else {
				if sharedTokenKey != "" { // Reuse the same token for clusters without ACE.
					tokenKey = sharedTokenKey
				} else {
					tokenKey, err = s.userMgr.EnsureToken(input)
					sharedTokenKey = tokenKey
				}
			}
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error creating a kubeconfig token for cluster %s: %w", cluster.Name, err))
			}
		}

		input = append(input, kconfig.Input{
			ClusterID: cluster.Name,
			Cluster:   cluster,
			Nodes:     nodes,
			TokenKey:  tokenKey,
		})
	}

	generated, err := kconfig.Generate(host, input)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error generating kubeconfig: %w", err))
	}

	created := kubeconfig.DeepCopy()
	created.Name = kubeConfigID
	created.Status = &ext.KubeconfigStatus{Value: generated}

	return created, nil
}

// exists is a helper that checks if a kubeconfig with the given name already exists
// by listing tokens with the label [tokens.TokenKubeconfigIDLabel] matching the name.
func (s *Store) exists(name string) (bool, error) {
	kindReq, err := labels.NewRequirement(tokens.TokenKindLabel, selection.Equals, []string{"kubeconfig"})
	if err != nil {
		return false, fmt.Errorf("error creating selector requirement: %w", err)
	}
	idReq, err := labels.NewRequirement(tokens.TokenKubeconfigIDLabel, selection.Equals, []string{name})
	if err != nil {
		return false, fmt.Errorf("error creating selector requirement for label %s: %w", tokens.TokenKubeconfigIDLabel, err)
	}
	selector := labels.NewSelector().Add(*kindReq).Add(*idReq)

	tokenList, err := s.tokenCache.List(selector)
	if err != nil {
		return false, fmt.Errorf("error listing tokens for kubeconfig %s: %w", name, err)
	}

	return len(tokenList) > 0, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// createTokenInput is a helper that builds [user.TokenInput] for a kubeconfig token.
func (s *Store) createTokenInput(kubeConfigID, userName string, authToken *apiv3.Token, ttl *int64) user.TokenInput {
	return user.TokenInput{
		TokenName:     "kubeconfig-" + userName,
		Description:   "Kubeconfig token",
		Kind:          "kubeconfig",
		UserName:      userName,
		AuthProvider:  authToken.AuthProvider,
		TTL:           ttl,
		Randomize:     true,
		UserPrincipal: authToken.UserPrincipal,
		Labels: map[string]string{
			tokens.TokenKubeconfigIDLabel: kubeConfigID,
		},
	}
}

func tokenSelector(isAdmin bool, userID, tokenID string) (labels.Selector, error) {
	kindReq, err := labels.NewRequirement(tokens.TokenKindLabel, selection.Equals, []string{"kubeconfig"})
	if err != nil {
		return nil, fmt.Errorf("error creating selector requirement for label %s: %w", tokens.TokenKindLabel, err)
	}
	selector := labels.NewSelector().Add(*kindReq)

	if tokenID != "" {
		idReq, err := labels.NewRequirement(tokens.TokenKubeconfigIDLabel, selection.Equals, []string{tokenID})
		if err != nil {
			return nil, fmt.Errorf("error creating selector requirement for label %s: %w", tokens.TokenKubeconfigIDLabel, err)
		}
		selector = selector.Add(*idReq)
	}

	if !isAdmin {
		userIDReq, err := labels.NewRequirement(tokens.UserIDLabel, selection.Equals, []string{userID})
		if err != nil {
			return nil, fmt.Errorf("error creating selector requirement for label %s: %w", tokens.UserIDLabel, err)
		}
		selector = selector.Add(*userIDReq)
	}

	return selector, nil
}

// Get implements [rest.Getter]
func (s *Store) Get(
	ctx context.Context,
	name string,
	_ *metav1.GetOptions, // Ignored because kubeconfig is an ephemeral resource.
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	selector, err := tokenSelector(isAdmin, userInfo.GetName(), name)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	tokenList, err := s.tokenCache.List(selector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
		return nil, fmt.Errorf("error listing tokens for kubeconfig %s: %w", name, err)
	}

	if len(tokenList) == 0 {
		return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	return &ext.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: tokenList[0].CreationTimestamp,
		},
	}, nil
}

// List implements [rest.Lister]
func (s *Store) NewList() runtime.Object {
	return &ext.KubeconfigList{}
}

// List implements [rest.Lister]
func (s *Store) List(
	ctx context.Context,
	_ *metainternalversion.ListOptions, // Ignored because kubeconfig is an ephemeral resource.
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	allTokens := ""
	selector, err := tokenSelector(isAdmin, userInfo.GetName(), allTokens)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	tokenList, err := s.tokenCache.List(selector)
	if err != nil {
		return nil, fmt.Errorf("error listing tokens for kubeconfigs: %w", err)
	}

	list := &ext.KubeconfigList{}
	seen := make(map[string]bool)
	for _, token := range tokenList {
		kubeConfigID := token.Labels[tokens.TokenKubeconfigIDLabel]
		if kubeConfigID == "" {
			continue
		}
		if seen[kubeConfigID] {
			continue
		}
		seen[kubeConfigID] = true

		list.Items = append(list.Items, ext.Kubeconfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:              kubeConfigID,
				CreationTimestamp: token.CreationTimestamp,
			},
		})
	}

	return list, nil
}

func (s *Store) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object,
) (*metav1.Table, error) {
	return extapi.ConvertToTableDefault[*ext.Kubeconfig](ctx, object, tableOptions, gvr.GroupResource())
}

// Delete implements [rest.GracefulDeleter]
func (s *Store) Delete(
	ctx context.Context,
	name string,
	_ rest.ValidateObjectFunc, // Ignored because kubeconfig is an ephemeral resource.
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	selector, err := tokenSelector(isAdmin, userInfo.GetName(), name)
	if err != nil {
		return nil, false, apierrors.NewInternalError(err)
	}

	tokenList, err := s.tokenCache.List(selector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
		return nil, false, fmt.Errorf("error listing tokens for kubeconfig %s: %w", name, err)
	}

	if len(tokenList) == 0 {
		return nil, false, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	var tokenNames []string
	for _, token := range tokenList {
		tokenNames = append(tokenNames, token.Name)
	}

	for _, tokenName := range tokenNames {
		delOptions := &metav1.DeleteOptions{
			// Preconditions are deliberatly ignored as kubeconfig is an ephemeral resource.
			GracePeriodSeconds: options.GracePeriodSeconds,
			PropagationPolicy:  options.PropagationPolicy,
			// Pass through the dry run flag instead of handling it explicitly.
			DryRun: options.DryRun,
		}
		err := s.tokens.Delete(tokenName, delOptions)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, false, fmt.Errorf("error deleting token %s for kubeconfig %s: %w", tokenName, name, err)
		}
	}

	return nil, true, nil
}

// GetSingularName implements [rest.SingularNameProvider]
func (s *Store) GetSingularName() string {
	return Singular
}

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (s *Store) GroupVersionKind(gv schema.GroupVersion) schema.GroupVersionKind {
	return gv.WithKind(Kind)
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {}

// NamespaceScoped implements [rest.Scoper]
func (t *Store) NamespaceScoped() bool {
	return false
}

var (
	_ rest.Creater                  = &Store{}
	_ rest.Getter                   = &Store{}
	_ rest.Lister                   = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)
