package tokens

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	extcore "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/strings/slices"
)

const (
	TokenNamespace = "cattle-tokens"
	ThirtyDays     = 30 * 24 * 60 * 60 * 1000 // 30 days in milliseconds.
	UserIDLabel    = "authn.management.cattle.io/token-userId"
	KindLabel      = "authn.management.cattle.io/kind"
	IsLogin        = "session"

	GroupCattleAuthenticated = "system:cattle:authenticated"

	// data fields used by the backing secrets to store token information
	FieldAnnotations    = "annotations"
	FieldAuthProvider   = "auth-provider"
	FieldClusterName    = "cluster-name"
	FieldDescription    = "description"
	FieldDisplayName    = "display-name"
	FieldEnabled        = "enabled"
	FieldHash           = "hash"
	FieldKind           = "kind"
	FieldLabels         = "labels"
	FieldLastUpdateTime = "last-update-time"
	FieldLastUsedAt     = "last-used-at"
	FieldLoginName      = "login-name"
	FieldPrincipalID    = "principal-id"
	FieldTTL            = "ttl"
	FieldUID            = "kube-uid"
	FieldUserID         = "user-id"
	// FieldIdleTimeout = "idle-timeout"	FUTURE ((USER ACTIVITY))

	SingularName = "token"
	PluralName   = SingularName + "s"
)

var GV = schema.GroupVersion{
	Group:   "ext.cattle.io",
	Version: "v1",
}

var GVK = schema.GroupVersionKind{
	Group:   GV.Group,
	Version: GV.Version,
	Kind:    "Token",
}
var GVR = schema.GroupVersionResource{
	Group:    GV.Group,
	Version:  GV.Version,
	Resource: "tokens",
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// ////////////////////////////////////////////////////////////////////////////////

// Store is the interface to the token store seen by the extension API and
// users. Wrapped around a SystemStore it performs the necessary checks to
// ensure that Users have only access to the tokens they are permitted to.
type Store struct {
	SystemStore
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemStore is the interface to the token store used internally by other
// parts of rancher. It does not perform any kind of permission checks, and
// operates with admin authority, except where told to not to. IOW it generally
// has access to all the tokens, in all ways.
type SystemStore struct {
	namespaceClient   v1.NamespaceClient // access to namespaces
	initialized       bool               // flag is set when this store ensured presence of the backing namespace
	secretClient      v1.SecretClient
	secretCache       v1.SecretCache
	userClient        v3.UserCache
	normanTokenClient v3.TokenCache // ProviderAndPrincipal extraction

	authorizer authorizer.Authorizer

	timer  timeHandler // subsystem for timestamp generation
	hasher hashHandler // subsystem for generation and hashing of secret values
	auth   authHandler // subsystem for user retrieval from context
}

// ////////////////////////////////////////////////////////////////////////////////
// store contruction methods

// NewFromWrangler is a convenience function for creating a token store.
// It initializes the returned store from the provided wrangler context.
func NewFromWrangler(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return New(
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.User(),
		wranglerContext.Mgmt.Token().Cache(),
		authorizer,
		NewTimeHandler(),
		NewHashHandler(),
		NewAuthHandler(),
	)
}

// New is the main constructor for token stores. It is supplied with accessors
// to all the other controllers the store requires for proper function. Note
// that it is recommended to use the NewFromWrangler convenience function
// instead.
func New(
	namespaceClient v1.NamespaceClient,
	secretClient v1.SecretController,
	userClient v3.UserController,
	tokenClient v3.TokenCache,
	authorizer authorizer.Authorizer,
	timer timeHandler,
	hasher hashHandler,
	auth authHandler,
) *Store {
	tokenStore := Store{
		SystemStore: SystemStore{
			namespaceClient:   namespaceClient,
			secretClient:      secretClient,
			secretCache:       secretClient.Cache(),
			userClient:        userClient.Cache(),
			normanTokenClient: tokenClient,
			authorizer:        authorizer,
			timer:             timer,
			hasher:            hasher,
			auth:              auth,
		},
	}
	return &tokenStore
}

// NewSystemFromWrangler is a convenience function for creating a system token
// store. It initializes the returned store from the provided wrangler context.
func NewSystemFromWrangler(wranglerContext *wrangler.Context) *SystemStore {
	return NewSystem(
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.User(),
		wranglerContext.Mgmt.Token().Cache(),
		NewTimeHandler(),
		NewHashHandler(),
		NewAuthHandler(),
	)
}

// NewSystem is the main constructor for system stores. It is supplied with
// accessors to all the other controllers the store requires for proper
// function. Note that it is recommended to use the NewSystemFromWrangler
// convenience function instead.
func NewSystem(
	namespaceClient v1.NamespaceClient,
	secretClient v1.SecretController,
	userClient v3.UserController,
	tokenClient v3.TokenCache,
	timer timeHandler,
	hasher hashHandler,
	auth authHandler,
) *SystemStore {
	// BEWARE: no authorizer set. this means that (internal) system stores
	// cannot create tokens limited to a cluster, as they are unable to
	// check if the user has access to that cluster. It will panic if you
	// try.
	//
	// NOTE: on the other side, so far all uses of system stores in other
	// places of rancher do not create tokens at all.
	tokenStore := SystemStore{
		namespaceClient:   namespaceClient,
		secretClient:      secretClient,
		secretCache:       secretClient.Cache(),
		userClient:        userClient.Cache(),
		normanTokenClient: tokenClient,
		timer:             timer,
		hasher:            hasher,
		auth:              auth,
	}
	return &tokenStore
}

// ////////////////////////////////////////////////////////////////////////////////
// Required interfaces:
// - [rest.GroupVersionKindProvider],
// - [rest.Scoper],
// - [rest.SingularNameProvider], and
// - [rest.Storage]

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (t *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper]
func (t *Store) NamespaceScoped() bool {
	return false
}

// GetSingularName implements [rest.SingularNameProvider]
func (t *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (t *Store) New() runtime.Object {
	obj := &ext.Token{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage]
func (t *Store) Destroy() {
}

// ////////////////////////////////////////////////////////////////////////////////
// Optional interfaces -- All implemented, supporting all regular k8s verbs
// - [x] create:           [rest.Creater]
// - [x] delete:           [rest.GracefulDeleter]
// - -- deletecollection: [rest.CollectionDeleter]
// - [x] get:              [rest.Getter]
// - [x] list:             [rest.Lister]
// -    patch:            [rest.Patcher] (this is Getter + Updater)
// - [x] update:           [rest.Updater]
// - [x] watch:            [rest.Watcher]

// The interface methods mostly delegate to the actual store methods, with some
// general method-dependent boilerplate behaviour before and/or after.

// Create implements [rest.Creator]
func (t *Store) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		err := createValidation(ctx, obj)
		if err != nil {
			return obj, err
		}
	}

	objToken, ok := obj.(*ext.Token)
	if !ok {
		var zeroT *ext.Token
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T",
			zeroT, obj))
	}

	return t.create(ctx, objToken, options)
}

// Delete implements [rest.GracefulDeleter]
func (t *Store) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	// locate resource first
	obj, err := t.get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	// ensure that deletion is possible
	if deleteValidation != nil {
		err := deleteValidation(ctx, obj)
		if err != nil {
			return nil, false, err
		}
	}

	// and now actually delete
	err = t.delete(ctx, obj, options)
	if err != nil {
		return nil, false, err
	}

	return obj, true, nil
}

// Get implements [rest.Getter]
func (t *Store) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions) (runtime.Object, error) {
	return t.get(ctx, name, options)
}

// NewList implements [rest.Lister]
func (t *Store) NewList() runtime.Object {
	objList := &ext.TokenList{}
	objList.GetObjectKind().SetGroupVersionKind(GVK)
	return objList
}

// List implements [rest.Lister]
func (t *Store) List(
	ctx context.Context,
	internaloptions *metainternalversion.ListOptions) (runtime.Object, error) {
	options, err := extcore.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	return t.list(ctx, options)
}

// ConvertToTable implements [rest.Lister]
func (t *Store) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object) (*metav1.Table, error) {

	return extcore.ConvertToTableDefault[*ext.Token](ctx, object, tableOptions,
		GVR.GroupResource())
}

// Update implements [rest.Updater]
func (t *Store) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return extcore.CreateOrUpdate(ctx, name, objInfo, createValidation,
		updateValidation, forceAllowCreate, options,
		t.get, t.create, t.update)
}

// Watch implements [rest.Watcher]
func (t *Store) Watch(
	ctx context.Context,
	internaloptions *metainternalversion.ListOptions) (watch.Interface, error) {
	options, err := extcore.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	return t.watch(ctx, options)
}

// ////////////////////////////////////////////////////////////////////////////////
// Actual K8s verb implementations

func (t *Store) create(ctx context.Context, token *ext.Token, options *metav1.CreateOptions) (*ext.Token, error) {
	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if !userMatch(user, token) {
		return nil, apierrors.NewBadRequest("unable to create token for other user")
	}
	return t.SystemStore.Create(ctx, GVR.GroupResource(), token, options)
}

func (t *SystemStore) Create(ctx context.Context, group schema.GroupResource, token *ext.Token, options *metav1.CreateOptions) (*ext.Token, error) {
	// check if the user does not wish to actually change anything
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	// ensure existence of the namespace holding our secrets. run once per store.
	if !dryRun && !t.initialized {
		_, err := t.namespaceClient.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: TokenNamespace,
			},
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		t.initialized = true
	}

	ensureNameOrGenerateName(token)
	// we check the Name directly. because of the ensure... we know that
	// GenerateName is not set. as it squashes the name in that case.
	if token.Name != "" {
		// reject creation of a token which already exists
		currentSecret, err := t.secretCache.Get(TokenNamespace, token.Name)
		if err == nil && currentSecret != nil {
			return nil, apierrors.NewAlreadyExists(group, token.Name)
		}
	}

	// reject user-provided token value, or hash
	if token.Status.TokenValue != "" {
		return nil, apierrors.NewBadRequest("User provided token value is not permitted")
	}
	if token.Status.TokenHash != "" {
		return nil, apierrors.NewBadRequest("User provided token hash is not permitted")
	}

	// Get derived User information for the token.
	//
	// NOTE: The creation process is not given `AuthProvider`, `User-` and `GroupPrincipals`.
	// ..... This information has to be retrieved from somewhere else in the system.
	// ..... This is in contrast to the Norman tokens who get this information either
	// ..... as part of the Login process, or by copying the information out of the
	// ..... base token the new one is derived from. None of that is possible here.
	//
	// The token store gets the necessary information (auth provider and
	// principal id) from the extras of the user stored in the context.
	//
	// The `UserPrincipal` data is filled in part from the above, and in
	// part from the associated `User`s fields.
	//
	// `ProviderInfo` is not supported. Norman tokens have it as legacy fallback to hold the
	// `access_token` data managed by OIDC-based auth providers. The actual primary storage for
	// this is actually a regular k8s Secret associated with the User.

	user, err := t.userClient.Get(token.Spec.UserID)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve user %s: %w",
			token.Spec.UserID, err))
	}

	// Reject operation if the user is disabled.
	if user.Enabled != nil && !*user.Enabled {
		return nil, apierrors.NewBadRequest("operation references a disabled user")
	}

	// Retrieve auth provider and principal id for the user associated with
	// the authenticated token of the request.

	authProvider, principalID, err := t.auth.ProviderAndPrincipal(ctx, t)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// Generate secret and its hash

	tokenValue, hashedValue, err := t.hasher.MakeAndHashSecret()
	if err != nil {
		return nil, err
	}

	token.Status.TokenHash = hashedValue
	token.Status.AuthProvider = authProvider
	token.Status.DisplayName = user.DisplayName
	token.Status.LoginName = user.Username
	token.Status.PrincipalID = principalID
	token.Status.LastUpdateTime = t.timer.Now()

	rest.FillObjectMetaSystemFields(token)

	secret, err := secretFromToken(token, nil, nil)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert token %s for storage: %w",
			token.Name, err))
	}

	// Check the user's accessibility of the cluster the token is limited to, if such is done.
	if token.Spec.ClusterName != "" {
		if err := t.IsClusterAccessible(ctx, token.Spec.ClusterName); err != nil {
			return nil, err
		}
	}

	// Abort, user does not wish to actually change anything.
	if dryRun {
		return token, nil
	}

	// Save new secret
	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// can happen despite the early check for a pre-existing secret.
			// something else may have raced us while the secret was assembled.
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to store token %s: %w",
			token.Name, err))
	}

	// Read changes back to return what was truly created, not what we thought we created
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		// An error here means that something broken was stored. Do not
		// leave that broken thing behind.
		t.secretClient.Delete(TokenNamespace, secret.Name, &metav1.DeleteOptions{})

		// And report what was broken
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			token.Name, err))
	}

	// The newly created token cannot have the same name as the token which
	// authenticated the request.
	newToken.Status.Current = false

	// users don't care about the hashed value
	newToken.Status.TokenHash = ""
	newToken.Status.TokenValue = tokenValue

	return newToken, nil
}

func (t *Store) delete(ctx context.Context, token *ext.Token, options *metav1.DeleteOptions) error {
	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	if !userMatch(user, token) {
		return apierrors.NewNotFound(GVR.GroupResource(), token.Name)
	}

	return t.SystemStore.Delete(token.Name, options)
}

func (t *SystemStore) Delete(name string, options *metav1.DeleteOptions) error {
	err := t.secretClient.Delete(TokenNamespace, name, options)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return apierrors.NewInternalError(fmt.Errorf("failed to delete token %s: %w", name, err))
}

func (t *Store) get(ctx context.Context, name string, options *metav1.GetOptions) (*ext.Token, error) {
	sessionID := t.auth.SessionID(ctx)

	// note: have to get token first before we can check for a user mismatch
	token, err := t.SystemStore.Get(name, sessionID, options)
	if err != nil {
		return nil, err
	}

	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if !userMatch(user, token) {
		return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
	}

	return token, nil
}

func (t *SystemStore) Get(name, sessionID string, options *metav1.GetOptions) (*ext.Token, error) {
	// Core token retrieval from backing secrets
	// We try to go through the fast cache as much as we can.

	var err error
	var currentSecret *corev1.Secret
	empty := metav1.GetOptions{}
	if options == nil || *options == empty {
		currentSecret, err = t.secretCache.Get(TokenNamespace, name)
	} else {
		currentSecret, err = t.secretClient.Get(TokenNamespace, name, *options)
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", name, err))
	}
	token, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", name, err))
	}

	token.Status.Current = token.Name == sessionID
	token.Status.TokenValue = ""
	return token, nil
}

func (t *Store) list(ctx context.Context, options *metav1.ListOptions) (*ext.TokenList, error) {
	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	sessionID := t.auth.SessionID(ctx)

	return t.SystemStore.list(false, user, sessionID, options)
}

func (t *SystemStore) List(options *metav1.ListOptions) (*ext.TokenList, error) {
	return t.list(true, "", "", options)
}

func (t *SystemStore) list(fullView bool, user, sessionID string, options *metav1.ListOptions) (*ext.TokenList, error) {
	// Merge our own selection request (user match!) into the caller's demands
	var localOptions metav1.ListOptions
	if fullView {
		// The system is allowed to list all tokens. No internal filtering is applied.
		localOptions = *options
	} else {
		// Non-system requests always filter the tokens down to those of the current user.
		var err error
		localOptions, err = ListOptionMerge(user, options)
		if err != nil {
			return nil,
				apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w",
					err))
		}
		empty := metav1.ListOptions{}
		if localOptions == empty {
			// The setup indicated that we can bail out. I.e the
			// options ask for something which cannot match.
			return &ext.TokenList{}, nil
		}
	}

	// Core token listing from backing secrets
	secrets, err := t.secretClient.List(TokenNamespace, localOptions)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list tokens: %w", err))
	}

	var tokens []ext.Token
	for _, secret := range secrets.Items {
		token, err := tokenFromSecret(&secret)
		// ignore broken tokens
		if err != nil {
			continue
		}

		// Filtering for users is done already, see above where the options are set up and/or merged.

		token.Status.Current = token.Name == sessionID
		tokens = append(tokens, *token)
	}
	list := ext.TokenList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: secrets.ResourceVersion,
		},
		Items: tokens,
	}
	return &list, nil
}

func (t *Store) update(ctx context.Context, token *ext.Token, options *metav1.UpdateOptions) (*ext.Token, error) {
	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if !userMatch(user, token) {
		return nil, apierrors.NewNotFound(GVR.GroupResource(), token.Name)
	}

	sessionID := t.auth.SessionID(ctx)

	return t.SystemStore.update(sessionID, false, token, options)
}

func (t *SystemStore) Update(token *ext.Token, options *metav1.UpdateOptions) (*ext.Token, error) {
	return t.update("", true, token, options)
}

func (t *SystemStore) update(sessionID string, fullPermission bool, token *ext.Token,
	options *metav1.UpdateOptions) (*ext.Token, error) {
	// check if the user does not wish to actually change anything
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	currentSecret, err := t.secretCache.Get(TokenNamespace, token.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w",
			token.Name, err))
	}
	currentToken, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w",
			token.Name, err))
	}

	if token.ObjectMeta.UID != currentToken.ObjectMeta.UID {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit kube uuid",
			token.Name))
	}

	if token.Spec.UserID != currentToken.Spec.UserID {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit user id",
			token.Name))
	}
	if token.Spec.ClusterName != currentToken.Spec.ClusterName {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit cluster name",
			token.Name))
	}
	if token.Spec.Kind != currentToken.Spec.Kind {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit kind",
			token.Name))
	}

	// Work on the time to live (TTL) value is a bit more complicated. Even
	// the owning user is not allowed to extend the TTL, only keep or shrink
	// it. Only the system itself is allowed to perform an extension. Note
	// that nothing currently makes use of that.

	if !fullPermission {
		if token.Spec.TTL > currentToken.Spec.TTL {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to extend time-to-live",
				token.Name))
		}
	}

	// Keep the status of the resource unchanged, never store a token value, etc.
	// IOW changes to display name, login name, etc. are all ignored without a peep.
	token.Status = currentToken.Status
	token.Status.TokenValue = ""
	// Refresh time of last update to current.
	token.Status.LastUpdateTime = t.timer.Now()

	// Save changes to backing secret, properly pass old secret labels/anotations into the new.
	secret, err := secretFromToken(token, currentSecret.Labels, currentSecret.Annotations)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert token %s for storage: %w",
			token.Name, err))
	}

	// Abort, user does not wish to actually change anything.
	if dryRun {
		return token, nil
	}

	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to update token %s: %w",
			token.Name, err))
	}

	// Read changes back to return what was truly saved, not what we thought we saved
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			token.Name, err))
	}

	newToken.Status.Current = newToken.Name == sessionID
	newToken.Status.TokenValue = ""
	return newToken, nil
}

// FUTURE ((USER ACTIVITY)) modify and fill in as required by the field type.
// func (t *SystemTokenStore) UpdateIdleTimeout(name string, now time.Time) error {
// }

func (t *SystemStore) UpdateLastUsedAt(name string, now time.Time) error {
	// Operate directly on the backend secret holding the token

	nowStr := now.Format(time.RFC3339)
	nowEncoded := base64.StdEncoding.EncodeToString([]byte(nowStr))

	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "replace",
		Path:  "/data/" + FieldLastUsedAt,
		Value: nowEncoded,
	}})
	if err != nil {
		return err
	}

	_, err = t.secretClient.Patch(TokenNamespace, name, types.JSONPatchType, patch)
	return err
}

func (t *Store) watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	user, err := t.auth.UserName(ctx, &t.SystemStore)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// the channel to the consumer is given a bit of slack, allowing the
	// producer (the go routine below) to run a bit ahead of the consumer
	// for a burst of events.
	consumer := &watcher{
		ch: make(chan watch.Event, 100),
	}

	localOptions, err := ListOptionMerge(user, options)
	if err != nil {
		return nil,
			apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w",
				err))
	}

	empty := metav1.ListOptions{}
	if localOptions == empty {
		// The setup indicated that we can bail out. I.e the options ask
		// for something which cannot match. Simply return the watcher,
		// without feeding it anything.
		return consumer, nil
	}

	sessionID := t.auth.SessionID(ctx)

	// watch the backend secrets for changes and transform their events into
	// the appropriate token events.
	go func() {
		producer, err := t.secretClient.Watch(TokenNamespace, localOptions)
		if err != nil {
			close(consumer.ch)
			return
		}

		defer producer.Stop()
		for {
			select {
			case <-ctx.Done():
				// terminate if the context got cancelled on us
				// the context also cancels the consumer, i.e. invokes Stop() on it.
				return
			case event, more := <-producer.ResultChan():
				// terminate if the producer has nothing more to deliver
				// should not be possible.
				// and we cannot pass this state up either.
				// making it impossible to it as well.
				if !more {
					return
				}

				// skip bogus events on not-secrets
				secret, ok := event.Object.(*corev1.Secret)
				if !ok {
					continue
				}

				token, err := tokenFromSecret(secret)
				// skip broken tokens
				if err != nil {
					continue
				}

				// skipping tokens not owned by the watching
				// user is not required. The watch filter (see
				// ListOptionMerge above) takes care of only
				// asking for owned tokens

				token.Status.Current = token.Name == sessionID

				// push to consumer, and terminate ourselves if
				// the consumer terminated on us
				if pushed := consumer.addEvent(watch.Event{
					Type:   event.Type,
					Object: token,
				}); !pushed {
					return
				}
			}
		}
	}()

	return consumer, nil
}

// watcher implements [watch.Interface]
type watcher struct {
	closedLock sync.RWMutex
	closed     bool
	ch         chan watch.Event
}

// Stop implements [watch.Interface]
// As documented in [watch] it is forbidden to invoke this method from the
// producer, i.e. here the token store. This method is strictly for use by the
// consumer (the caller of the `watch` method above, i.e. k8s itself).
func (w *watcher) Stop() {
	w.closedLock.Lock()
	defer w.closedLock.Unlock()

	// no operation if called multiple times.
	if w.closed {
		return
	}

	close(w.ch)
	w.closed = true
}

// ResultChan implements [watch.Interface]
func (w *watcher) ResultChan() <-chan watch.Event {
	return w.ch
}

// addEvent pushes a new event to the watcher. This fails if the watcher was
// `Stop()`ed already by the consumer. The boolean result is true on success.
// This is used by the watcher-internal goroutine to determine if it has to
// terminate, or not.
func (w *watcher) addEvent(event watch.Event) bool {
	w.closedLock.RLock()
	defer w.closedLock.RUnlock()
	if w.closed {
		return false
	}

	w.ch <- event
	return true
}

// userMatch hides the details of matching a user name against an ext token.
func userMatch(name string, token *ext.Token) bool {
	return name == token.Spec.UserID
}

// ClusterCheck determines if the user a token is made for, who is the same as
// the user creating the token, has access to the cluster the token-to-be is
// limited in scope to. An error is thrown if not, or when the check itself
// failed.
func (t *SystemStore) IsClusterAccessible(ctx context.Context, clusterName string) error {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return apierrors.NewInternalError(
			fmt.Errorf("error checking authorization of user to access cluster %s: bad context",
				clusterName))
	}

	decision, _, err := t.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "get",
		APIGroup:        "management.cattle.io",
		Resource:        "clusters",
		ResourceRequest: true,
		Name:            clusterName,
	})

	if err != nil {
		return apierrors.NewInternalError(
			fmt.Errorf("error checking authorization of user %s to access cluster %s: %w",
				userInfo.GetName(), clusterName, err))
	}

	if decision != authorizer.DecisionAllow {
		return apierrors.NewForbidden(GVR.GroupResource(), clusterName,
			fmt.Errorf("user %s has no permission to access",
				userInfo.GetName()))
	}

	return nil
}

// Fetch is a convenience function for retrieving a token by name, regardless of
// type. I.e. this function auto-detects if the referenced token is
// norman/legacy, or ext, and returns a proper interface hiding the differences
// from the caller. It is public because it is of use to other parts of rancher,
// not just here.
func (t *SystemStore) Fetch(tokenID string) (accessor.TokenAccessor, error) {
	// checking for a norman token first, as it is the currently more common
	// type of tokens. in other words, high probability that we are done
	// with a single request. or even none, if the token is found in the
	// cache.
	if norman, err := t.normanTokenClient.Get(tokenID); err == nil {
		return norman, nil
	}

	// not a norman token, now check for ext token
	if ext, err := t.Get(tokenID, "", &metav1.GetOptions{}); err == nil {
		return ext, nil
	}

	return nil, fmt.Errorf("unable to fetch unknown token %s", tokenID)
}

// ////////////////////////////////////////////////////////////////////////////////
// Support interfaces for testability.

// Note: Review the interfaces and implementations below when we have more than
// just the token store, to consider generalization for sharing across stores.

// Mockable interfaces for permission checking, secret generation and hashing, and timing

// timeHandler is an interface hiding the details of timestamp generation from
// the store. This makes the operation mockable for store testing.
type timeHandler interface {
	Now() string
}

// hashHandler is an interface hiding the details of secret generation and
// hashing from the store. This makes these operations mockable for store
// testing.
type hashHandler interface {
	MakeAndHashSecret() (string, string, error)
}

// authHandler is an interface hiding the details of retrieving token auth
// information (user name, principal id, auth provider) from the store. This
// makes these operations mockable for store testing.
type authHandler interface {
	ProviderAndPrincipal(ctx context.Context, store *SystemStore) (string, string, error)
	SessionID(ctx context.Context) string
	UserName(ctx context.Context, store *SystemStore) (string, error)
}

// Standard implementations for the above interfaces.

func NewTimeHandler() timeHandler {
	return &tokenTimer{}
}

func NewHashHandler() hashHandler {
	return &tokenHasher{}
}

func NewAuthHandler() authHandler {
	return &tokenAuth{}
}

// tokenTimer is an implementation of the timeHandler interface.
type tokenTimer struct{}

// tokenHasher is an implementation of the hashHandler interface.
type tokenHasher struct{}

// tokenAuth is an implementation of the authHandler interface.
type tokenAuth struct{}

// Now returns the current time as a RFC 3339 formatted string.
func (tp *tokenTimer) Now() string {
	return time.Now().Format(time.RFC3339)
}

// MakeAndHashSecret creates a token secret, hashes it, and returns both secret and hash.
func (tp *tokenHasher) MakeAndHashSecret() (string, string, error) {
	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return "", "", apierrors.NewInternalError(fmt.Errorf("failed to generate token value: %w", err))
	}
	hashedValue, err := hashers.GetHasher().CreateHash(tokenValue)
	if err != nil {
		return "", "", apierrors.NewInternalError(fmt.Errorf("failed to hash token value: %w", err))
	}

	return tokenValue, hashedValue, nil
}

// UserName hides the details of extracting a user name from the request context
func (tp *tokenAuth) UserName(ctx context.Context, store *SystemStore) (string, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return "", fmt.Errorf("context has no user info")
	}

	userName := userInfo.GetName()

	if strings.Contains(userName, ":") { // E.g. system:admin
		return "", apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	if !slices.Contains(userInfo.GetGroups(), GroupCattleAuthenticated) {
		return "", apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	_, err := store.userClient.Get(userName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s not found", userName))
		}

		return "", apierrors.NewInternalError(fmt.Errorf("error getting user %s: %w", userName, err))
	}

	return userName, nil
}

// ProviderAndPrincipal hides the details of extracting the auth provider and
// principal id for the authenticated token from the request context. It uses
// the provided store to fetch the detailed token data.
func (tp *tokenAuth) ProviderAndPrincipal(ctx context.Context, store *SystemStore) (string, string, error) {
	tokenID, err := SessionID(ctx)
	if err != nil {
		return "", "", err
	}

	token, err := store.Fetch(tokenID)
	if err != nil {
		// Well, an invalid token has invalid data
		return "", "", fmt.Errorf("context contains invalid provider/principal data")
	}

	return token.GetAuthProvider(), token.GetUserPrincipal().Name, nil
}

// SessionID hides the details of extracting the name of the authenticated token
// governing the current session from the request context, for a a store. It
// exists purely to allow unit testing to intercept and mock responses.  It also
// DIFFERS from the core function, see below, in that it ignores errors.
// I.e. in case of error the result is simply the empty string. Which means that
// for requests with broken token information no returned token will be marked
// as current, as a kube resource cannot have the empty string as its name.
func (tp *tokenAuth) SessionID(ctx context.Context) string {
	tokenID, _ := SessionID(ctx)
	return tokenID
}

// SessionID hides the details of extracting the name of the authenticated token
// governing the current session from the request context. It is made public for
// use by other parts of rancher. NOTE that no token store is required for its
// use.
func SessionID(ctx context.Context) (string, error) {
	// see also `pkg/auth/requests/authenticate.go` (TokenFromRequest)
	// this here is a customized variant
	// - different origin of the token id
	// - no direct type detection and dispatch
	//   - multiple query attempts instead
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return "", fmt.Errorf("context has no provider/principal data")
	}

	extras := userInfo.GetExtra()
	if extras == nil {
		return "", fmt.Errorf("context has no provider/principal data")
	}

	tokenIDs := extras[common.ExtraRequestTokenID]
	if len(tokenIDs) != 1 {
		return "", fmt.Errorf("context has no provider/principal data")
	}

	tokenID := tokenIDs[0]

	return tokenID, nil
}

// Internal supporting functionality

// ListOptionMerge merges external filter options with the internal filter (for
// the current user).  A non-error empty result indicates that the options
// specified a filter which cannot match anything.  I.e. the calling user
// requests a filter for a different user than itself.
func ListOptionMerge(user string, options *metav1.ListOptions) (metav1.ListOptions, error) {
	var localOptions metav1.ListOptions

	quest := labels.Set(map[string]string{
		UserIDLabel: user,
	})
	empty := metav1.ListOptions{}
	if options == nil || *options == empty {
		// No external filter to contend with, just set the internal
		// filter.
		localOptions = metav1.ListOptions{
			LabelSelector: quest.AsSelector().String(),
		}
	} else {
		// We have to contend with an external filter, and merge ours
		// into it.
		localOptions = *options
		callerQuest, err := labels.ConvertSelectorToLabelsMap(localOptions.LabelSelector)
		if err != nil {
			return localOptions, err
		}
		if callerQuest.Has(UserIDLabel) {
			// The external filter does filter for user as, possible
			// conflict.
			if callerQuest[UserIDLabel] != user {
				// It asks for a user other than the current.
				// We can bail now, with an empty result, as
				// nothing can match.
				return localOptions, nil
			}
			// It asks for the current user, same as the internal
			// filter would do.  We can pass the options as is.
		} else {
			// The external filter has nothing about the user.
			// We can simply add the internal filter.
			localOptions.LabelSelector = labels.Merge(callerQuest, quest).AsSelector().String()
		}
	}

	return localOptions, nil
}

// secretFromToken converts the token argument into the equivalent secrets to
// store in k8s.
func secretFromToken(token *ext.Token, oldBackendLabels, oldBackendAnnotations map[string]string) (*corev1.Secret, error) {
	// inject default on creation
	ttl := token.Spec.TTL
	if ttl == 0 {
		ttl = ThirtyDays
		// pass back to caller (Create)
		token.Spec.TTL = ttl
	}

	// annotations -- encode and store into a data field. on reading decode
	// that field.  this fully separates the annotations on the tokens from
	// the annotations on the backing secrets.

	annotationBytes, err := json.Marshal(token.Annotations)
	if err != nil {
		return nil, err
	}

	// labels -- encode and store into a data field. on reading decode that
	// field.  this fully separates the user-visible labels on the tokens
	// from the labels on the backing secrets.

	labelBytes, err := json.Marshal(token.Labels)
	if err != nil {
		return nil, err
	}

	// labels again -- for proper filtering and searching the referenced
	// user id, and the kind of the token are placed into labels of the
	// backing secret -- the keys for these labels are part of the public
	// API. This may have to be merged with labels set on the secrets by
	// other apps with access to the secrets.

	backendLabels := oldBackendLabels
	if backendLabels == nil {
		backendLabels = map[string]string{}
	}
	backendLabels[UserIDLabel] = token.Spec.UserID
	backendLabels[KindLabel] = token.Spec.Kind

	// ensure that only one of name or generateName is passed through.
	name := token.Name
	genName := token.GenerateName
	if genName != "" {
		name = ""
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    TokenNamespace,
			Name:         name,
			GenerateName: genName,
			Labels:       backendLabels,
			Annotations:  oldBackendAnnotations,
		},
		StringData: make(map[string]string),
		Data:       make(map[string][]byte),
	}

	// system
	secret.StringData[FieldUID] = string(token.ObjectMeta.UID)
	secret.StringData[FieldLabels] = string(labelBytes)
	secret.StringData[FieldAnnotations] = string(annotationBytes)

	// spec
	enabled := token.Spec.Enabled == nil || *token.Spec.Enabled

	secret.StringData[FieldUserID] = token.Spec.UserID
	secret.StringData[FieldClusterName] = token.Spec.ClusterName
	secret.StringData[FieldTTL] = fmt.Sprintf("%d", ttl)
	secret.StringData[FieldEnabled] = fmt.Sprintf("%t", enabled)
	secret.StringData[FieldDescription] = token.Spec.Description
	secret.StringData[FieldKind] = token.Spec.Kind

	lastUsedAsString := ""
	if token.Status.LastUsedAt != nil {
		lastUsedAsString = token.Status.LastUsedAt.Format(time.RFC3339)
	}

	// status
	secret.StringData[FieldHash] = token.Status.TokenHash
	secret.StringData[FieldLastUpdateTime] = token.Status.LastUpdateTime
	secret.StringData[FieldLastUsedAt] = lastUsedAsString

	// FUTURE ((USER ACTIVITY)) change as required by the field type
	// secret.StringData[FieldIdleTimeout] = fmt.Sprintf("%d", token.Status.IdleTimeout)

	// Note:
	// - While the derived expiration data is not stored, the user-related information is.
	// - The expiration data is computed trivially from spec and resource data.
	// - Getting the user-related information on the other hand is expensive.
	// - It is better to cache it in the backing secret

	secret.StringData[FieldAuthProvider] = token.Status.AuthProvider
	secret.StringData[FieldDisplayName] = token.Status.DisplayName
	secret.StringData[FieldLoginName] = token.Status.LoginName
	secret.StringData[FieldPrincipalID] = token.Status.PrincipalID

	return secret, nil
}

// tokenFromSecret converts the secret argument (retrieved from the k8s store)
// into the equivalent token.
func tokenFromSecret(secret *corev1.Secret) (*ext.Token, error) {
	// Basic result. This will be incrementally filled as data is extracted from the secret.
	// On error a partially filled token is returned.
	// See the token store `Delete` (marker **) for where this is important.
	token := &ext.Token{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Token",
			APIVersion: "ext.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			CreationTimestamp: secret.CreationTimestamp,
		},
	}

	token.Spec.Description = string(secret.Data[FieldDescription])
	token.Spec.ClusterName = string(secret.Data[FieldClusterName])
	token.Spec.Kind = string(secret.Data[FieldKind])

	token.Status.DisplayName = string(secret.Data[FieldDisplayName])
	token.Status.LoginName = string(secret.Data[FieldLoginName])

	userId := string(secret.Data[FieldUserID])
	if userId == "" {
		return token, fmt.Errorf("user id missing")
	}
	token.Spec.UserID = userId

	// spec
	enabled, err := strconv.ParseBool(string(secret.Data[FieldEnabled]))
	if err != nil {
		return token, err
	}
	token.Spec.Enabled = &enabled

	ttl, err := strconv.ParseInt(string(secret.Data[FieldTTL]), 10, 64)
	if err != nil {
		return token, err
	}
	// inject default on retrieval
	if ttl == 0 {
		ttl = ThirtyDays
	}
	token.Spec.TTL = ttl

	// FUTURE ((USER ACTIVITY)) change as required by the field type
	//
	// BEWARE. Depending on releases made this code may have to handle
	// rancher instances containing ext tokens without idle information,
	// without crashing. I.e. insert a default when the information is not
	// present, instead of returning an error.
	//
	// idle, err := strconv.ParseInt(string(secret.Data[FieldIdleTimeout]), 10, 64)
	// if err != nil {
	// 	return token, err
	// }
	// token.Status.IdleTimeout = idle

	tokenHash := string(secret.Data[FieldHash])
	if tokenHash == "" {
		return token, fmt.Errorf("token hash missing")
	}
	token.Status.TokenHash = tokenHash

	authProvider := string(secret.Data[FieldAuthProvider])
	if authProvider == "" {
		return token, fmt.Errorf("auth provider missing")
	}
	token.Status.AuthProvider = authProvider

	lastUpdateTime := string(secret.Data[FieldLastUpdateTime])
	if lastUpdateTime == "" {
		return token, fmt.Errorf("last update time missing")
	}
	token.Status.LastUpdateTime = lastUpdateTime

	// The principal id is the object name of the virtual v3.Principal
	// resource and is therefore a required data element. display and login
	// name on the other hand are optional.
	principalID := string(secret.Data[FieldPrincipalID])
	if principalID == "" {
		return token, fmt.Errorf("principal id missing")
	}
	token.Status.PrincipalID = principalID

	kubeUID := string(secret.Data[FieldUID])
	if kubeUID == "" {
		return token, fmt.Errorf("kube uid missing")
	}
	token.ObjectMeta.UID = types.UID(kubeUID)

	var lastUsedAt *metav1.Time
	lastUsedAsString := string(secret.Data[FieldLastUsedAt])
	if lastUsedAsString != "" {
		lastUsed, err := time.Parse(time.RFC3339, lastUsedAsString)
		if err != nil {
			return token, fmt.Errorf("failed to parse lastUsed data: %w", err)
		}
		lastUsedTime := metav1.NewTime(lastUsed)
		lastUsedAt = &lastUsedTime
	} // else: empty => lastUsedAt == nil
	token.Status.LastUsedAt = lastUsedAt

	if err := setExpired(token); err != nil {
		return token, fmt.Errorf("failed to set expiration information: %w", err)
	}

	// annotations, labels - decode the respective fields and place them into the token

	if err := json.Unmarshal(secret.Data[FieldLabels], &token.Labels); err != nil {
		return token, err
	}

	if err := json.Unmarshal(secret.Data[FieldAnnotations], &token.Annotations); err != nil {
		return token, err
	}

	return token, nil
}

// setExpired computes the expiration data (isExpired, expiresAt) from token
// creation time and time to live and places the results into the associated
// token fields.
func setExpired(token *ext.Token) error {
	if token.Spec.TTL < 0 {
		token.Status.Expired = false
		token.Status.ExpiresAt = ""
		return nil
	}

	expiresAt := token.ObjectMeta.CreationTimestamp.Add(time.Duration(token.Spec.TTL) * time.Millisecond)
	isExpired := time.Now().After(expiresAt)

	eAt, err := metav1.NewTime(expiresAt).MarshalJSON()
	if err != nil {
		return err
	}

	// note: The marshalling puts quotes around the string. strip them
	// before handing this to the token and yaml adding another layer
	// of quotes around such a string
	token.Status.ExpiresAt = string(eAt[1 : len(eAt)-1])
	token.Status.Expired = isExpired
	return nil
}

// ensureNameOrGenerateName ensures that the token has either a proper name, or
// a generateName clause. Note, this function does __not generate__ the name if
// the latter is present. That is delegated to the backend store, i.e. the
// secrets holding tokens. See `secretFromToken` above.
func ensureNameOrGenerateName(token *ext.Token) error {
	// NOTE: When both name and generateName are set the generateName has precedence

	if token.ObjectMeta.GenerateName != "" {
		token.ObjectMeta.Name = ""
		return nil
	}
	if token.ObjectMeta.Name != "" {
		return nil
	}

	return apierrors.NewBadRequest(fmt.Sprintf(
		"Token \"%s\" is invalid: metadata.name: Required value: name or generateName is required",
		token.ObjectMeta.Name))
}
