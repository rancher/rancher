// tokens implements the store for the new public API token resources, also
// known as ext tokens.
package tokens

//go::generate mockgen -source tokens.go -destination=zz_token_fakes.go -package=tokens

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
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
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
)

const (
	TokenNamespace = "cattle-tokens"
	UserIDLabel    = "authn.management.cattle.io/token-userId"
	KindLabel      = "authn.management.cattle.io/kind"
	IsLogin        = "session"

	// names of the data fields used by the backing secrets to store token information
	FieldAnnotations      = "annotations"
	FieldDescription      = "description"
	FieldEnabled          = "enabled"
	FieldFinalizers       = "finalizers"
	FieldHash             = "hash"
	FieldKind             = "kind"
	FieldLabels           = "labels"
	FieldLastActivitySeen = "last-activity-seen"
	FieldLastUpdateTime   = "last-update-time"
	FieldLastUsedAt       = "last-used-at"
	FieldOwnerReferences  = "owners"
	FieldPrincipal        = "principal"
	FieldTTL              = "ttl"
	FieldUID              = "kube-uid"
	FieldUserID           = "user-id"

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

// Store is the interface to the token store seen by the extension API and
// users. Wrapped around a SystemStore it performs the necessary checks to
// ensure that Users have only access to the tokens they are permitted to.
type Store struct {
	SystemStore
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemStore is the interface to the token store used internally by other
// parts of Rancher. It does not perform any kind of permission checks, and
// operates with admin authority, except where it is told to not to. In other
// words, it generally has access to all the tokens, in all ways.
type SystemStore struct {
	authorizer      authorizer.Authorizer
	initialized     bool               // flag. set when this store ensured presence of the backing namespace
	namespaceClient v1.NamespaceClient // access to namespaces.
	secretClient    v1.SecretClient    // direct access to the backing secrets
	secretCache     v1.SecretCache     // cached access to the backing secrets
	userClient      v3.UserCache       // cached access to the v3.Users
	v3TokenClient   v3.TokenCache      // cached access to v3.Tokens. See Fetch.
	timer           timeHandler        // access to timestamp generation
	hasher          hashHandler        // access to generation and hashing of secret values
	auth            authHandler        // access to user retrieval from context
}

// NewFromWrangler is a convenience function for creating a token store.
// It initializes the returned store from the provided wrangler context.
func NewFromWrangler(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return New(
		authorizer,
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.User(),
		wranglerContext.Mgmt.Token().Cache(),
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
	authorizer authorizer.Authorizer,
	namespaceClient v1.NamespaceClient,
	secretClient v1.SecretController,
	userClient v3.UserController,
	tokenClient v3.TokenCache,
	timer timeHandler,
	hasher hashHandler,
	auth authHandler,
) *Store {
	tokenStore := Store{
		SystemStore: SystemStore{
			authorizer:      authorizer,
			namespaceClient: namespaceClient,
			secretClient:    secretClient,
			secretCache:     secretClient.Cache(),
			userClient:      userClient.Cache(),
			v3TokenClient:   tokenClient,
			timer:           timer,
			hasher:          hasher,
			auth:            auth,
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
	tokenStore := SystemStore{
		namespaceClient: namespaceClient,
		secretClient:    secretClient,
		secretCache:     secretClient.Cache(),
		userClient:      userClient.Cache(),
		v3TokenClient:   tokenClient,
		timer:           timer,
		hasher:          hasher,
		auth:            auth,
	}
	return &tokenStore
}

// GroupVersionKind implements [rest.GroupVersionKindProvider], a required interface.
func (t *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper], a required interface.
func (t *Store) NamespaceScoped() bool {
	return false
}

// GetSingularName implements [rest.SingularNameProvider], a required interface.
func (t *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage], a required interface.
func (t *Store) New() runtime.Object {
	obj := &ext.Token{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage], a required interface.
func (t *Store) Destroy() {
}

// Create implements [rest.Creator], the interface to support the `create`
// verb. Delegates to the actual store method after some generic boilerplate.
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

// Delete implements [rest.GracefulDeleter], the interface to support the
// `delete` verb. Delegates to the actual store method after some generic
// boilerplate.
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

// Get implements [rest.Getter], the interface to support the `get` verb.
// Simply delegates to the actual store method.
func (t *Store) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions) (runtime.Object, error) {
	return t.get(ctx, name, options)
}

// NewList implements [rest.Lister], the interface to support the `list` verb.
func (t *Store) NewList() runtime.Object {
	objList := &ext.TokenList{}
	objList.GetObjectKind().SetGroupVersionKind(GVK)
	return objList
}

// List implements [rest.Lister], the interface to support the `list` verb.
// Simply delegates to the actual store method.
func (t *Store) List(
	ctx context.Context,
	internaloptions *metainternalversion.ListOptions) (runtime.Object, error) {
	options, err := extcore.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	return t.list(ctx, options)
}

// ConvertToTable implements [rest.Lister], the interface to support the `list` verb.
func (t *Store) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object) (*metav1.Table, error) {

	return extcore.ConvertToTableDefault[*ext.Token](ctx, object, tableOptions,
		GVR.GroupResource())
}

// Update implements [rest.Updater], the interface to support the `update` verb.
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

// Watch implements [rest.Watcher], the interface to support the `watch` verb.
func (t *Store) Watch(
	ctx context.Context,
	internaloptions *metainternalversion.ListOptions) (watch.Interface, error) {
	options, err := extcore.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	return t.watch(ctx, options)
}

// create implements the core resource creation for tokens
func (t *Store) create(ctx context.Context, token *ext.Token, options *metav1.CreateOptions) (*ext.Token, error) {
	userName, _, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "create")
	if err != nil {
		return nil, err
	}
	if !isRancherUser {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "",
			fmt.Errorf("user %s is not a Rancher user", userName))
	}
	if !userMatchOrDefault(userName, token) {
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

	user, err := t.userClient.Get(token.Spec.UserID)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve user %s: %w",
			token.Spec.UserID, err))
	}

	// Reject operation if the user is disabled.
	if user.Enabled != nil && !*user.Enabled {
		return nil, apierrors.NewBadRequest("operation references a disabled user")
	}

	// Get token of the request and use its principal as ours. Any attempt
	// by the user to set their own information for the principal is
	// discarded and written over. No checks are made, no errors are thrown.
	requestToken, err := t.Fetch(t.auth.SessionID(ctx))
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	rtPrincipal := requestToken.GetUserPrincipal()
	token.Spec.UserPrincipal = ext.TokenPrincipal{
		Name:           rtPrincipal.ObjectMeta.Name,
		DisplayName:    rtPrincipal.DisplayName,
		LoginName:      rtPrincipal.LoginName,
		ProfilePicture: rtPrincipal.ProfilePicture,
		ProfileURL:     rtPrincipal.ProfileURL,
		PrincipalType:  rtPrincipal.PrincipalType,
		Me:             rtPrincipal.Me,
		MemberOf:       rtPrincipal.MemberOf,
		Provider:       rtPrincipal.Provider,
		ExtraInfo:      rtPrincipal.ExtraInfo,
	}

	// Generate a secret and its hash
	tokenValue, hashedValue, err := t.hasher.MakeAndHashSecret()
	if err != nil {
		return nil, err
	}

	token.Status = ext.TokenStatus{
		Hash:           hashedValue,
		LastUpdateTime: t.timer.Now(),
	}

	rest.FillObjectMetaSystemFields(token)

	secret, err := secretFromToken(token, nil, nil)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert token %s for storage: %w",
			token.Name, err))
	}

	// Return early as the user does not wish to actually change anything.
	if dryRun {
		return token, nil
	}

	// Save new secret
	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// can happen despite the early check for a pre-existing secret.
			// another request may have raced us while the secret was assembled.
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to store token %s: %w",
			token.Name, err))
	}

	// Read changes back to return what was truly created, not what we thought we created
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		// An error here means that something broken was stored.
		// Do not leave that broken thing behind.
		t.secretClient.Delete(TokenNamespace, secret.Name, &metav1.DeleteOptions{})

		// And report what was broken
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			token.Name, err))
	}

	// The newly created token is not the request token
	newToken.Status.Current = false

	// users don't care about the hashed value, just the secret
	// here is the only place the secret is returned and disclosed.
	newToken.Status.Hash = ""
	newToken.Status.Value = tokenValue

	return newToken, nil
}

// delete implements the core resource destruction for tokens
func (t *Store) delete(ctx context.Context, token *ext.Token, options *metav1.DeleteOptions) error {
	user, fullAccess, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "delete")
	if err != nil {
		return err
	}
	if !fullAccess && (!isRancherUser || !userMatch(user, token)) {
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

// get implements the core resource retrieval for tokens
func (t *Store) get(ctx context.Context, name string, options *metav1.GetOptions) (*ext.Token, error) {
	userName, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "get")
	if err != nil {
		return nil, err
	}

	// note: have to get token first before we can check for a user mismatch
	token, err := t.SystemStore.Get(name, t.auth.SessionID(ctx), options)
	if err != nil {
		return nil, err
	}

	if fullAccess {
		return token, nil
	}
	if !userMatch(userName, token) {
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
			return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", name, err))
	}
	token, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", name, err))
	}

	token.Status.Current = token.Name == sessionID
	token.Status.Value = ""
	return token, nil
}

// list implements the core resource listing of tokens
func (t *Store) list(ctx context.Context, options *metav1.ListOptions) (*ext.TokenList, error) {
	userName, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "list")
	if err != nil {
		return nil, err
	}

	return t.SystemStore.list(fullAccess, userName, t.auth.SessionID(ctx), options)
}

// ListForUser returns the set of token owned by the named user. It is an
// internal call invoked by other parts of Rancher
func (t *SystemStore) ListForUser(userName string) (*ext.TokenList, error) {
	// As internal call this method can use the cache of secrets.
	// Query the cache using a proper label selector
	secrets, err := t.secretCache.List(TokenNamespace, labels.Set(map[string]string{
		UserIDLabel: userName,
	}).AsSelector())
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list tokens for user %s: %w", userName, err))
	}

	var tokens []ext.Token
	for _, secret := range secrets {
		token, err := tokenFromSecret(secret)
		// ignore broken tokens
		if err != nil {
			continue
		}

		tokens = append(tokens, *token)
	}

	return &ext.TokenList{
		Items: tokens,
	}, nil
}

func (t *SystemStore) list(fullAccess bool, userName, sessionID string, options *metav1.ListOptions) (*ext.TokenList, error) {
	// Non-system requests always filter the tokens down to those of the current user.
	// Merge our own selection request (user match!) into the caller's demands
	localOptions, err := ListOptionMerge(fullAccess, userName, options)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w", err))
	}
	empty := metav1.ListOptions{}
	if localOptions == empty {
		// The setup indicated that we can bail out. I.e the
		// options ask for something which cannot match.
		return &ext.TokenList{}, nil
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

	return &ext.TokenList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: secrets.ResourceVersion,
		},
		Items: tokens,
	}, nil
}

// update implements the core resource updating/modification of tokens
func (t *Store) update(ctx context.Context, token *ext.Token, options *metav1.UpdateOptions) (*ext.Token, error) {
	user, _, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "update")
	if err != nil {
		return nil, err
	}
	if !isRancherUser || !userMatch(user, token) {
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
			return nil, apierrors.NewNotFound(GVR.GroupResource(), token.Name)
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
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit kubernetes UID",
			token.Name))
	}

	if token.Spec.UserID != currentToken.Spec.UserID {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit user id",
			token.Name))
	}

	if token.Spec.Kind != currentToken.Spec.Kind {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit kind",
			token.Name))
	}

	// Regular users are not allowed to extend the TTL.
	if !fullPermission {
		ttl, err := clampMaxTTL(token.Spec.TTL)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to clamp token %s ttl: %w",
				token.Name, err))
		}
		token.Spec.TTL = ttl
		if ttlGreater(ttl, currentToken.Spec.TTL) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to extend time-to-live",
				token.Name))
		}
	}

	// Keep the status of the resource unchanged, never store a token value, etc.
	// IOW changes to display name, login name, etc. are all ignored without a peep.
	token.Status = currentToken.Status
	token.Status.Value = ""
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
	newToken.Status.Value = ""
	return newToken, nil
}

// UpdateLastUsedAt patches the last-used-at information of the token.
// Called during authentication.
func (t *SystemStore) UpdateLastUsedAt(name string, now time.Time) error {
	// Operate directly on the backend secret holding the token
	nowEncoded := base64.StdEncoding.EncodeToString([]byte(now.Format(time.RFC3339)))
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

// UpdateLastActivitySeen patches the last-activity-seen information of the token.
// Called from the ext user activity store.
func (t *SystemStore) UpdateLastActivitySeen(name string, now time.Time) error {
	// Operate directly on the backend secret holding the token
	nowEncoded := base64.StdEncoding.EncodeToString([]byte(now.Format(time.RFC3339)))
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "replace",
		Path:  "/data/" + FieldLastActivitySeen,
		Value: nowEncoded,
	}})
	if err != nil {
		return err
	}

	_, err = t.secretClient.Patch(TokenNamespace, name, types.JSONPatchType, patch)
	return err
}

// Disable patches the enabled flag of the token.
// Called by refreshAttributes.
func (t *SystemStore) Disable(name string) error {
	// Operate directly on the backend secret holding the token
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "replace",
		Path:  "/data/" + FieldEnabled,
		Value: base64.StdEncoding.EncodeToString([]byte("false")),
	}})
	if err != nil {
		return err
	}

	_, err = t.secretClient.Patch(TokenNamespace, name, types.JSONPatchType, patch)
	return err
}

// watch implements the core resource watcher for tokens
func (t *Store) watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	userName, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "watch")
	if err != nil {
		return nil, err
	}

	// the channel to the consumer is given a bit of slack, allowing the
	// producer (the go routine below) to run a bit ahead of the consumer
	// for a burst of events.
	consumer := &watcher{
		ch: make(chan watch.Event, 100),
	}

	localOptions, err := ListOptionMerge(fullAccess, userName, options)
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

// userMatchOrDefault hides the details of matching a user name against an ext
// token, and may set the default if nothing is specified in the token.
func userMatchOrDefault(name string, token *ext.Token) bool {
	if token.Spec.UserID == "" {
		token.Spec.UserID = name
		return true
	}
	return name == token.Spec.UserID
}

// Fetch is a convenience function for retrieving a token by name, regardless of
// type. I.e. this function auto-detects if the referenced token is a v3 or ext
// token, and returns a proper interface hiding the differences from the caller.
// It is public because it is of use to other parts of rancher, not just here.
func (t *SystemStore) Fetch(tokenID string) (accessor.TokenAccessor, error) {
	// checking for a v3 Token first, as it is the currently more common
	// type of tokens. in other words, high probability that we are done
	// with a single request. or even none, if the token is found in the
	// cache.
	if v3token, err := t.v3TokenClient.Get(tokenID); err == nil {
		return v3token, nil
	}

	// not a v3 Token, now check for ext token
	if ext, err := t.Get(tokenID, "", &metav1.GetOptions{}); err == nil {
		return ext, nil
	}

	return nil, fmt.Errorf("unable to fetch unknown token %s", tokenID)
}

// timeHandler is a helper interface hiding the details of timestamp generation from
// the store. This makes the operation mockable for store testing.
type timeHandler interface {
	Now() string
}

// hashHandler is a helper interface hiding the details of secret generation and
// hashing from the store. This makes these operations mockable for store
// testing.
type hashHandler interface {
	MakeAndHashSecret() (string, string, error)
}

// authHandler is a helper interface hiding the details of retrieving token auth
// information (user name, principal id, auth provider) from the store. This
// makes these operations mockable for store testing.
type authHandler interface {
	SessionID(ctx context.Context) string
	UserName(ctx context.Context, store *SystemStore, verb string) (string, bool, bool, error)
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

// UserName hides the details of extracting a user name and its permission
// status from the request context
func (tp *tokenAuth) UserName(ctx context.Context, store *SystemStore, verb string) (string, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return "", false, false, apierrors.NewInternalError(fmt.Errorf("context has no user info"))
	}

	decision, _, err := store.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
		Resource:        "*",
		ResourceRequest: true,
	})
	if err != nil {
		return "", false, false, err
	}

	fullAccess := decision == authorizer.DecisionAllow

	isRancherUser := false
	userName := userInfo.GetName()

	if !strings.Contains(userName, ":") { // E.g. system:admin
		// potentially a rancher user
		_, err := store.userClient.Get(userName)
		if err == nil {
			// definitely a rancher user
			isRancherUser = true
		} else if !apierrors.IsNotFound(err) {
			// some general error
			return "", false, false,
				apierrors.NewInternalError(fmt.Errorf("error getting user %s: %w", userName, err))
		} // else: not a rancher user, may still be an admin
	} // else: some system user, not a rancher user, may still be an admin

	return userName, fullAccess, isRancherUser, nil
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

// ListOptionMerge merges any external filter options with the internal filter
// (for the current user).  A non-error empty result indicates that the options
// specified a filter which cannot match anything.  I.e. the calling user
// requests a filter for a different user than itself.
func ListOptionMerge(fullAccess bool, userName string, options *metav1.ListOptions) (metav1.ListOptions, error) {
	var localOptions metav1.ListOptions

	// for admins we do not impose any additional restrictions over the requested
	if fullAccess {
		return *options, nil
	}

	// for non-admins we additionally filter the result for their own tokens
	userIDSelector := labels.Set(map[string]string{
		UserIDLabel: userName,
	})
	empty := metav1.ListOptions{}
	if options == nil || *options == empty {
		// No external filter to contend with, just set the internal filter.
		localOptions = metav1.ListOptions{
			LabelSelector: userIDSelector.AsSelector().String(),
		}
	} else {
		// We have to contend with an external filter, and merge ours into it.
		localOptions = *options
		callerSelector, err := labels.ConvertSelectorToLabelsMap(localOptions.LabelSelector)
		if err != nil {
			return localOptions, err
		}
		if callerSelector.Has(UserIDLabel) {
			// The external filter does filter for a user, possible conflict.
			if callerSelector[UserIDLabel] != userName {
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
			localOptions.LabelSelector = labels.Merge(callerSelector, userIDSelector).AsSelector().String()
		}
	}

	return localOptions, nil
}

// secretFromToken converts the token argument into the equivalent secrets to
// store in k8s.
func secretFromToken(token *ext.Token, oldBackendLabels, oldBackendAnnotations map[string]string) (*corev1.Secret, error) {
	// user principal
	principalBytes, err := json.Marshal(token.Spec.UserPrincipal)
	if err != nil {
		return nil, err
	}

	// finalizers -- encode and store into a data field. on reading decode
	// that field.  this fully separates the finalizers on the tokens from
	// finalizers on the backing secrets.
	finalizerBytes, err := json.Marshal(token.Finalizers)
	if err != nil {
		return nil, err
	}

	// ownerReferences -- encode and store into a data field. on reading
	// decode that field.  this fully separates the ownerReferences on the
	// tokens from ownerReferences on the backing secrets.
	ownerBytes, err := json.Marshal(token.OwnerReferences)
	if err != nil {
		return nil, err
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

	// labels again -- for proper filtering and searching both referenced
	// user id and kind of the token are placed into labels of the backing
	// secret -- the keys for these labels are part of the public API. This
	// may have to be merged with labels set on the secrets by other apps
	// with access to the secrets.
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

	// base structure
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

	// system information
	secret.StringData[FieldUID] = string(token.ObjectMeta.UID)
	secret.StringData[FieldLabels] = string(labelBytes)
	secret.StringData[FieldAnnotations] = string(annotationBytes)
	secret.StringData[FieldFinalizers] = string(finalizerBytes)
	secret.StringData[FieldOwnerReferences] = string(ownerBytes)

	// spec values

	// injects default on creation
	ttl, err := clampMaxTTL(token.Spec.TTL)
	if err != nil {
		return nil, err
	}
	// pass back to caller (Create)
	token.Spec.TTL = ttl

	secret.StringData[FieldDescription] = token.Spec.Description
	secret.StringData[FieldEnabled] = fmt.Sprintf("%t", token.Spec.Enabled == nil || *token.Spec.Enabled)
	secret.StringData[FieldKind] = token.Spec.Kind
	secret.StringData[FieldPrincipal] = string(principalBytes)
	secret.StringData[FieldTTL] = fmt.Sprintf("%d", ttl)
	secret.StringData[FieldUserID] = token.Spec.UserID

	// status elements
	lastUsedAtAsString := ""
	if token.Status.LastUsedAt != nil {
		lastUsedAtAsString = token.Status.LastUsedAt.Format(time.RFC3339)
	}
	secret.StringData[FieldLastUsedAt] = lastUsedAtAsString
	secret.StringData[FieldHash] = token.Status.Hash
	secret.StringData[FieldLastUpdateTime] = token.Status.LastUpdateTime
	secret.StringData[FieldLastActivitySeen] = ""

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
			Kind:       GVK.Kind,
			APIVersion: GV.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			CreationTimestamp: secret.CreationTimestamp,
		},
	}

	// system - kubernetes uid
	if token.ObjectMeta.UID = types.UID(string(secret.Data[FieldUID])); token.ObjectMeta.UID == "" {
		return token, fmt.Errorf("kube uid missing")
	}

	// system - finalizers - decode the field and place into the token
	if err := json.Unmarshal(secret.Data[FieldFinalizers], &token.Finalizers); err != nil {
		return token, err
	}

	// system - owner references - decode the field and place into the token
	if err := json.Unmarshal(secret.Data[FieldOwnerReferences], &token.OwnerReferences); err != nil {
		return token, err
	}

	// system - labels - decode the field and place into the token
	if err := json.Unmarshal(secret.Data[FieldLabels], &token.Labels); err != nil {
		return token, err
	}

	// system - annotations - decode the fields and place into the token
	if err := json.Unmarshal(secret.Data[FieldAnnotations], &token.Annotations); err != nil {
		return token, err
	}

	// spec - user id, required
	if token.Spec.UserID = string(secret.Data[FieldUserID]); token.Spec.UserID == "" {
		return token, fmt.Errorf("user id missing")
	}

	// spec - user principal, required
	if err := json.Unmarshal(secret.Data[FieldPrincipal], &token.Spec.UserPrincipal); err != nil {
		return token, err
	}
	if token.Spec.UserPrincipal.Name == "" {
		return token, fmt.Errorf("principal id missing")
	}
	if token.Spec.UserPrincipal.Provider == "" {
		return token, fmt.Errorf("auth provider missing")
	}

	// spec - optional elements
	token.Spec.Description = string(secret.Data[FieldDescription])
	token.Spec.Kind = string(secret.Data[FieldKind])

	enabled, err := strconv.ParseBool(string(secret.Data[FieldEnabled]))
	if err != nil {
		return token, err
	}
	token.Spec.Enabled = &enabled

	ttl, err := strconv.ParseInt(string(secret.Data[FieldTTL]), 10, 64)
	if err != nil {
		return token, err
	}

	// clamp and inject default on retrieval
	ttl, err = clampMaxTTL(ttl)
	if err != nil {
		return token, err
	}

	token.Spec.TTL = ttl

	// status information
	if token.Status.Hash = string(secret.Data[FieldHash]); token.Status.Hash == "" {
		return token, fmt.Errorf("token hash missing")
	}

	if token.Status.LastUpdateTime = string(secret.Data[FieldLastUpdateTime]); token.Status.LastUpdateTime == "" {
		return token, fmt.Errorf("last update time missing")
	}

	lastUsedAt, err := decodeTime("lastUsedAt", secret.Data[FieldLastUsedAt])
	if err != nil {
		return token, err
	}
	token.Status.LastUsedAt = lastUsedAt

	lastActivitySeen, err := decodeTime("lastActivitySeen", secret.Data[FieldLastActivitySeen])
	if err != nil {
		return token, err
	}
	token.Status.LastActivitySeen = lastActivitySeen

	if err := setExpired(token); err != nil {
		return token, fmt.Errorf("failed to set expiration information: %w", err)
	}

	return token, nil
}

// decodeTime parses the byte-slice of the secret into a proper k8s timestamp.
func decodeTime(label string, timeBytes []byte) (*metav1.Time, error) {
	if timeAsString := string(timeBytes); timeAsString != "" {
		time, err := time.Parse(time.RFC3339, timeAsString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s data: %w", label, err)
		}
		kubeTime := metav1.NewTime(time)
		return &kubeTime, nil
	} // else: empty => time == nil

	return nil, nil
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
	token.Status.Expired = time.Now().After(expiresAt)
	token.Status.ExpiresAt = expiresAt.Format(time.RFC3339)
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

func clampMaxTTL(ttl int64) (int64, error) {
	max, err := maxTTL()
	if err != nil {
		return 0, err
	}

	// decision table
	// max | ttl         | note                                        | result
	// --- + ----------- + ------------------------------------------- + ----------------
	// < 1 | < 0         | max, ttl = +inf, no clamp                   | ttl
	// < 1 | = 0         | max = +inf = default, ttl default requested | -1 = +inf
	// < 1 | > 0         | max = +inf, ttl is regular, less than max   | ttl
	// --- + ----------- + ------------------------------------------- + ----------------
	// > 0 | < 0         | ttl = +inf, clamp to max                    | max
	// > 0 | = 0         | ttl default requested, this is max          | max
	// > 0 | > 0, <= max | less than max                               | ttl
	// > 0 | > max       | clamp to max                                | max

	if max < 1 {
		if ttl == 0 {
			return -1, nil
		}
		return ttl, nil
	}
	if ttl > max || ttl <= 0 {
		return max, nil
	}
	return ttl, nil
}

func maxTTL() (int64, error) {
	maxTTL, err := tokens.ParseTokenTTL(settings.AuthTokenMaxTTLMinutes.Get())

	if err != nil {
		return 0, fmt.Errorf("failed to parse setting '%s': %w", settings.AuthTokenMaxTTLMinutes.Name, err)
	}

	return maxTTL.Milliseconds(), nil
}

// ttlGreater compares the two TTL a and b. It returns true if a is greater than b.
// Important special cases for TTL:
// Any value < 0 represents +infinity.
// A value > 0 is that many milliseconds.
// The default of `0` cannot arrive here. `clampMaxTTL` resolved that already.
func ttlGreater(a, b int64) bool {
	// Decision table
	//
	// a   b   | note                           | result
	// --------+--------------------------------+-------
	// <0  <0  | both infinite, same            | false
	// <0  >=0 | a infinite, b not, greater     | true
	// >=0 <0  | b infinite, a not, less        | false
	// >=0 >=0 | regular compare                | t/f

	if a < 0 && b < 0 {
		return false
	}
	if a < 0 && b >= 0 {
		return true
	}
	if a >= 0 && b < 0 {
		return false
	}

	return a > b
}
