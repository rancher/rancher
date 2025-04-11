// tokens implements the store for the new public API token resources, also
// known as ext tokens.
package tokens

//go::generate mockgen -source tokens.go -destination=zz_token_fakes.go -package=tokens

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	extcore "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/features"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
)

const (
	TokenNamespace       = "cattle-tokens"
	UserIDLabel          = "cattle.io/user-id"
	KindLabel            = "authn.management.cattle.io/kind"
	IsLogin              = "session"
	SecretKindLabel      = "cattle.io/kind"
	SecretKindLabelValue = "token"
	GeneratePrefix       = "token-"

	// names of the data fields used by the backing secrets to store token information
	FieldClusterName      = "cluster"
	FieldDescription      = "description"
	FieldEnabled          = "enabled"
	FieldHash             = "hash"
	FieldKind             = "kind"
	FieldLastActivitySeen = "last-activity-seen"
	FieldLastUpdateTime   = "last-update-time"
	FieldLastUsedAt       = "last-used-at"
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
	namespaceClient v1.NamespaceClient  // access to namespaces.
	namespaceCache  v1.NamespaceCache   // quick access to namespaces.
	secretClient    v1.SecretClient     // direct access to the backing secrets
	secretCache     v1.SecretCache      // cached access to the backing secrets
	userClient      v3.UserCache        // cached access to the v3.Users
	v3TokenClient   v3.TokenCache       // cached access to v3.Tokens. See Fetch.
	clusterCache    v3.ClusterCache     // cached access to cluster for presence checks
	timer           timeHandler         // access to timestamp generation
	hasher          hashHandler         // access to generation and hashing of secret values
	auth            authHandler         // access to user retrieval from context
	tableConverter  rest.TableConvertor // custom column formatting
}

// NewFromWrangler is a convenience function for creating a token store.
// It initializes the returned store from the provided wrangler context.
func NewFromWrangler(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return New(
		authorizer,
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Namespace().Cache(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.User(),
		wranglerContext.Mgmt.Token().Cache(),
		wranglerContext.Mgmt.Cluster().Cache(),
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
	namespaceCache v1.NamespaceCache,
	secretClient v1.SecretController,
	userClient v3.UserController,
	tokenClient v3.TokenCache,
	clusterClient v3.ClusterCache,
	timer timeHandler,
	hasher hashHandler,
	auth authHandler,
) *Store {
	tokenStore := Store{
		SystemStore: SystemStore{
			authorizer:      authorizer,
			namespaceClient: namespaceClient,
			namespaceCache:  namespaceCache,
			secretClient:    secretClient,
			secretCache:     secretClient.Cache(),
			userClient:      userClient.Cache(),
			v3TokenClient:   tokenClient,
			clusterCache:    clusterClient,
			timer:           timer,
			hasher:          hasher,
			auth:            auth,
			tableConverter: printerstorage.TableConvertor{
				TableGenerator: printers.NewTableGenerator().With(printHandler),
			},
		},
	}
	return &tokenStore
}

// NewSystemFromWrangler is a convenience function for creating a system token
// store. It initializes the returned store from the provided wrangler context.
func NewSystemFromWrangler(wranglerContext *wrangler.Context) *SystemStore {
	return NewSystem(
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Namespace().Cache(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.User(),
		wranglerContext.Mgmt.Token().Cache(),
		wranglerContext.Mgmt.Cluster().Cache(),
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
	namespaceCache v1.NamespaceCache,
	secretClient v1.SecretController,
	userClient v3.UserController,
	tokenClient v3.TokenCache,
	clusterClient v3.ClusterCache,
	timer timeHandler,
	hasher hashHandler,
	auth authHandler,
) *SystemStore {
	tokenStore := SystemStore{
		namespaceClient: namespaceClient,
		namespaceCache:  namespaceCache,
		secretClient:    secretClient,
		secretCache:     secretClient.Cache(),
		userClient:      userClient.Cache(),
		v3TokenClient:   tokenClient,
		clusterCache:    clusterClient,
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

// ensureNamespace ensures that the namespace for storing token secrets exists.
func (t *SystemStore) ensureNamespace() error {
	return extcommon.EnsureNamespace(t.namespaceCache, t.namespaceClient, TokenNamespace)
}

// Create implements [rest.Creator], the interface to support the `create`
// verb. Delegates to the actual store method after some generic boilerplate.
// Note: Name and GenerateName are not respected. A name is generated with a
// predefined prefix instead.
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
	if err := t.delete(ctx, obj, options); err != nil {
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

// ConvertToTable implements [rest.Lister]/[rest.TableConvertor], the interface to support the `list` verb.
func (t *Store) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object) (*metav1.Table, error) {
	return t.tableConverter.ConvertToTable(ctx, object, tableOptions)
}

// printHandler registers the column definitions and actual formatter functions
func printHandler(h printers.PrintHandler) {
	columnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "User", Type: "string", Priority: 1, Description: "User is the owner of the token"},
		{Name: "Kind", Type: "string", Description: "Kind/purpose of the token"},
		{Name: "TTL", Type: "string", Description: "The time-to-live for the token"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "Description", Type: "string", Priority: 1, Description: "Human readable description of the token"},
	}
	_ = h.TableHandler(columnDefinitions, printTokenList)
	_ = h.TableHandler(columnDefinitions, printToken)
}

// printToken formats a single Token for table printing
func printToken(token *ext.Token, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	return []metav1.TableRow{{
		Object: runtime.RawExtension{Object: token},
		Cells: []any{
			token.Name,
			token.Spec.UserID,
			token.Spec.Kind,
			duration.HumanDuration(time.Duration(token.Spec.TTL) * time.Millisecond),
			translateTimestampSince(token.CreationTimestamp),
			token.Spec.Description,
		},
	}}, nil
}

// printTokenList formats a set of Tokens for table printing
func printTokenList(tokenList *ext.TokenList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(tokenList.Items))
	for i := range tokenList.Items {
		r, err := printToken(&tokenList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

// translateTimestampSince returns a human-readable approximation of the elapsed
// time since timestamp
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// Update implements [rest.Updater], the interface to support the `update` verb.
// Note: Create on update is not supported because names are always auto-generated.
func (t *Store) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions) (runtime.Object, bool, error) {

	userInfo, fullAccess, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "update")
	if err != nil {
		return nil, false, err
	}

	oldSecret, err := t.secretCache.Get(TokenNamespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Rethrow the NotFound error with the correct group and resource information.
			return nil, false, apierrors.NewNotFound(GVR.GroupResource(), name)
		}
		return nil, false, fmt.Errorf("error getting secret for token %s: %w", name, err)
	}

	// validate that secret is indeed holding an ext token
	if oldSecret.Labels[SecretKindLabel] != SecretKindLabelValue {
		return nil, false, apierrors.NewNotFound(GVR.GroupResource(), name)
	}

	oldToken, err := fromSecret(oldSecret)
	if err != nil {
		return nil, false, apierrors.NewInternalError(
			fmt.Errorf("error converting secret %s to token: %w", name, err))
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldToken)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting updated object: %w", err))
	}

	newToken, ok := newObj.(*ext.Token)
	if !ok {
		return nil, false, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", newObj))
	}

	if updateValidation != nil {
		err = updateValidation(ctx, newObj, oldToken)
		if err != nil {
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("error validating update: %s", err))
		}
	}

	if !fullAccess && (!isRancherUser || !userMatch(userInfo.GetName(), oldToken)) {
		return nil, false, apierrors.NewNotFound(GVR.GroupResource(), oldToken.Name)
	}

	sessionID := t.auth.SessionID(ctx)

	resultToken, err := t.SystemStore.update(sessionID, false, oldToken, newToken, options)

	return resultToken, false, err
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
	userInfo, _, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "create")
	if err != nil {
		return nil, err
	}
	if !isRancherUser {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "",
			fmt.Errorf("user %s is not a Rancher user", userInfo.GetName()))
	}
	if !userMatchOrDefault(userInfo.GetName(), token) {
		return nil, apierrors.NewBadRequest("unable to create token for other user")
	}
	return t.SystemStore.Create(ctx, GVR.GroupResource(), token, options, userInfo)
}

func (t *SystemStore) Create(ctx context.Context, group schema.GroupResource, token *ext.Token, options *metav1.CreateOptions, userInfo user.Info) (*ext.Token, error) {
	// check if the user does not wish to actually change anything
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

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

	if token.Spec.ClusterName != "" {
		// Verify existence of cluster
		cluster, err := t.clusterCache.Get(token.Spec.ClusterName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, apierrors.NewBadRequest(fmt.Sprintf("cluster %s not found",
					token.Spec.ClusterName))
			}
			return nil, apierrors.NewInternalError(fmt.Errorf("error getting cluster %s: %w",
				token.Spec.ClusterName, err))
		}

		// Verify that user is authorized to access cluster
		decision, _, err := t.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
			User:            userInfo,
			Verb:            "get",
			APIGroup:        mgmt.GroupName,
			Resource:        apiv3.ClusterResourceName,
			ResourceRequest: true,
			Name:            cluster.Name,
		})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error authorizing user %s to access cluster %s: %w",
				userInfo.GetName(), cluster.Name, err))
		}

		if decision != authorizer.DecisionAllow {
			return nil, apierrors.NewForbidden(GVR.GroupResource(), "",
				fmt.Errorf("user %s is not allowed to access cluster %s",
					userInfo.GetName(), cluster.Name))
		}
	}

	// Generate a secret and its hash
	tokenValue, hashedValue, err := t.hasher.MakeAndHashSecret()
	if err != nil {
		return nil, err
	}

	// ignore incoming status, persist new fields
	token.Status = ext.TokenStatus{
		Hash:           hashedValue,
		LastUpdateTime: t.timer.Now(),
	}

	rest.FillObjectMetaSystemFields(token)

	// Return early as the user does not wish to actually change anything.
	if dryRun {
		// enforce our choice of name
		token.ObjectMeta.Name, err = t.generateName(GeneratePrefix)
		token.ObjectMeta.GenerateName = ""
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	secret, err := toSecret(token)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert token %s for storage: %w",
			token.Name, err))
	}

	// enforce our choice of name, without racing create
	secret.ObjectMeta.Name = ""
	secret.ObjectMeta.GenerateName = GeneratePrefix

	if err = t.ensureNamespace(); err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error ensuring namespace %s: %w", TokenNamespace, err))
	}

	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// note: should not be possible due to the forced use of generateName
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to store token: %w", err))
	}

	// Read changes back to return what was truly created, not what we thought we created
	newToken, err := fromSecret(newSecret)
	if err != nil {
		// An error here means that something broken was stored.
		// Do not leave that broken thing behind.
		t.secretClient.Delete(TokenNamespace, newSecret.Name, &metav1.DeleteOptions{})

		// And report what was broken
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			newSecret.Name, err))
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
	userInfo, fullAccess, isRancherUser, err := t.auth.UserName(ctx, &t.SystemStore, "delete")
	if err != nil {
		return err
	}
	if !fullAccess && (!isRancherUser || !userMatch(userInfo.GetName(), token)) {
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
	userInfo, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "get")
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
	if !userMatch(userInfo.GetName(), token) {
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
	token, err := fromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", name, err))
	}

	token.Status.Current = token.Name == sessionID
	token.Status.Value = ""
	return token, nil
}

// list implements the core resource listing of tokens
func (t *Store) list(ctx context.Context, options *metav1.ListOptions) (*ext.TokenList, error) {
	userInfo, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "list")
	if err != nil {
		return nil, err
	}

	return t.SystemStore.list(fullAccess, userInfo.GetName(), t.auth.SessionID(ctx), options)
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
		token, err := fromSecret(secret)
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

	// Core token listing from backing secrets
	secrets, err := t.secretClient.List(TokenNamespace, localOptions)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) { // Continue token expired.
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list tokens: %w", err))
	}

	tokens := make([]ext.Token, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		token, err := fromSecret(&secret)
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
			ResourceVersion:    secrets.ResourceVersion,
			Continue:           secrets.Continue,
			RemainingItemCount: secrets.RemainingItemCount,
		},
		Items: tokens,
	}, nil
}

func (t *SystemStore) Update(oldToken, token *ext.Token, options *metav1.UpdateOptions) (*ext.Token, error) {
	return t.update("", true, oldToken, token, options)
}

func (t *SystemStore) update(sessionID string, fullPermission bool, oldToken, token *ext.Token,
	options *metav1.UpdateOptions) (*ext.Token, error) {
	// check if the user does not wish to actually change anything
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	if token.ObjectMeta.UID != oldToken.ObjectMeta.UID {
		return nil, apierrors.NewBadRequest("meta.UID is immutable")
	}

	if token.Spec.UserID != oldToken.Spec.UserID {
		return nil, apierrors.NewBadRequest("spec.userID is immutable")
	}

	if token.Spec.Kind != oldToken.Spec.Kind {
		return nil, apierrors.NewBadRequest("spec.kind is immutable")
	}

	if token.Spec.UserPrincipal.Name != oldToken.Spec.UserPrincipal.Name ||
		token.Spec.UserPrincipal.DisplayName != oldToken.Spec.UserPrincipal.DisplayName ||
		token.Spec.UserPrincipal.LoginName != oldToken.Spec.UserPrincipal.LoginName ||
		token.Spec.UserPrincipal.ProfilePicture != oldToken.Spec.UserPrincipal.ProfilePicture ||
		token.Spec.UserPrincipal.PrincipalType != oldToken.Spec.UserPrincipal.PrincipalType ||
		token.Spec.UserPrincipal.Me != oldToken.Spec.UserPrincipal.Me ||
		token.Spec.UserPrincipal.MemberOf != oldToken.Spec.UserPrincipal.MemberOf ||
		token.Spec.UserPrincipal.Provider != oldToken.Spec.UserPrincipal.Provider ||
		!reflect.DeepEqual(token.Spec.UserPrincipal.ExtraInfo, oldToken.Spec.UserPrincipal.ExtraInfo) {
		return nil, apierrors.NewBadRequest("spec.userprincipal is immutable")
	}

	if token.Spec.ClusterName != oldToken.Spec.ClusterName {
		return nil, apierrors.NewBadRequest("spec.clusterName is immutable")
	}

	// Regular users are not allowed to extend the TTL.
	if !fullPermission {
		ttl, err := clampMaxTTL(token.Spec.TTL)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to clamp token time-to-live: %w", err))
		}
		token.Spec.TTL = ttl
		if ttlGreater(ttl, oldToken.Spec.TTL) {
			return nil, apierrors.NewBadRequest("forbidden to extend time-to-live")
		}
	}

	// Keep the status of the resource unchanged, never store a token value, etc.
	// IOW changes to hash, value, etc. are all ignored without a peep.
	token.Status = oldToken.Status
	token.Status.Value = ""
	// Refresh time of last update to current.
	token.Status.LastUpdateTime = t.timer.Now()

	secret, err := toSecret(token)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert token for storage: %w", err))
	}

	// Abort, user does not wish to actually change anything.
	if dryRun {
		return token, nil
	}

	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to save updated token: %w", err))
	}

	// Read changes back to return what was truly saved, not what we thought we saved
	newToken, err := fromSecret(newSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token: %w", err))
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
	userInfo, fullAccess, _, err := t.auth.UserName(ctx, &t.SystemStore, "watch")
	if err != nil {
		return nil, err
	}

	// the channel to the consumer is given a bit of slack, allowing the
	// producer (the go routine below) to run a bit ahead of the consumer
	// for a burst of events.
	consumer := &watcher{
		ch: make(chan watch.Event, 100),
	}

	localOptions, err := ListOptionMerge(fullAccess, userInfo.GetName(), options)
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

	if !features.FeatureGates().Enabled(features.WatchListClient) {
		localOptions.SendInitialEvents = nil
		localOptions.ResourceVersionMatch = ""
	}

	producer, err := t.secretClient.Watch(TokenNamespace, localOptions)
	if err != nil {
		logrus.Errorf("tokens: watch: error starting watch: %s", err)
		return nil, apierrors.NewInternalError(fmt.Errorf("tokens: watch: error starting watch: %w", err))
	}

	sessionID := t.auth.SessionID(ctx)

	// watch the backend secrets for changes and transform their events into
	// the appropriate token events.
	go func() {
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

				var token *ext.Token
				switch event.Type {
				case watch.Bookmark:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("tokens: watch: expected secret got %T", event.Object)
						continue
					}

					token = &ext.Token{
						ObjectMeta: metav1.ObjectMeta{
							ResourceVersion: secret.ResourceVersion,
						},
					}
				case watch.Error:
					status, ok := event.Object.(*metav1.Status)
					if ok {
						logrus.Warnf("tokens: watch: received error event: %s", status.String())
					} else {
						logrus.Warnf("tokens: watch: received error event: %s", event.Object.GetObjectKind().GroupVersionKind().String())
					}
					continue
				case watch.Added, watch.Modified, watch.Deleted:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("tokens: watch: expected secret got %T", event.Object)
						continue
					}

					token, err = fromSecret(secret)
					if err != nil {
						logrus.Errorf("tokens: watch: error converting secret '%s' to token: %s", secret.Name, err)
						continue
					}

					// skipping tokens not owned by the watching
					// user is not required. The watch filter (see
					// ListOptionMerge above) takes care of only
					// asking for owned tokens
					token.Status.Current = token.Name == sessionID
				default:
					logrus.Warnf("tokens: watch: received and ignored unknown event: '%s'", event.Type)
					continue
				}

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

// generateName computes a unique name for a new token, from a fixed prefix.
func (t *SystemStore) generateName(prefix string) (string, error) {
	var tokenID string

	err := retry.OnError(retry.DefaultRetry, func(_ error) bool {
		return true // Retry all errors.
	}, func() error {
		tokenID = names.SimpleNameGenerator.GenerateName(prefix)
		_, err := t.secretCache.Get(TokenNamespace, tokenID)
		if err == nil {
			return fmt.Errorf("token %s already exists", tokenID)
		}

		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("error getting token %s: %w", tokenID, err)
	})

	if err != nil {
		return "", apierrors.NewInternalError(
			fmt.Errorf("error checking if token %s exists: %w", tokenID, err))
	}

	return tokenID, nil
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
	UserName(ctx context.Context, store *SystemStore, verb string) (user.Info, bool, bool, error)
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
func (tp *tokenAuth) UserName(ctx context.Context, store *SystemStore, verb string) (user.Info, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		logrus.Errorf("ext token store (%s request) no user information in request context", verb)
		return nil, false, false, apierrors.NewInternalError(fmt.Errorf("context has no user info"))
	}

	decision, _, err := store.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
		Resource:        "*",
		ResourceRequest: true,
	})
	if err != nil {
		logrus.Errorf("ext token store (%s request) by user %q: auth error: %v", verb, userInfo.GetName(), err)
		return nil, false, false, err
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
			logrus.Errorf("ext token store (%s request) by user %q: general error: %v", verb, userName, err)
			return nil, false, false,
				apierrors.NewInternalError(fmt.Errorf("error getting user %s: %w", userName, err))
		} // else: not a rancher user, may still be an admin
	} // else: some system user, not a rancher user, may still be an admin

	logrus.Debugf("ext token store (%s request) by user %q (full-access=%v, rancher-user=%v)", verb, userName, fullAccess, isRancherUser)
	return userInfo, fullAccess, isRancherUser, nil
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

// toSecret converts a Token object into the equivalent Secret resource.
func toSecret(token *ext.Token) (*corev1.Secret, error) {
	// base structure
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: TokenNamespace,
			Name:      token.Name,
		},
		StringData: make(map[string]string),
	}

	if len(token.Annotations) > 0 {
		secret.Annotations = make(map[string]string)
		for k, v := range token.Annotations {
			secret.Annotations[k] = v
		}
	}

	secret.Labels = make(map[string]string)
	for k, v := range token.Labels {
		secret.Labels[k] = v
	}
	secret.Labels[SecretKindLabel] = SecretKindLabelValue
	secret.Labels[UserIDLabel] = token.Spec.UserID
	secret.Labels[KindLabel] = token.Spec.Kind

	secret.Finalizers = append(secret.Finalizers, token.Finalizers...)
	secret.OwnerReferences = append(secret.OwnerReferences, token.OwnerReferences...)

	// user principal
	principalBytes, err := json.Marshal(token.Spec.UserPrincipal)
	if err != nil {
		return nil, err
	}

	// system information. remainder is handled through secret's ObjectMeta
	secret.StringData[FieldUID] = string(token.ObjectMeta.UID)

	// spec values
	// injects default on creation and update
	ttl, err := clampMaxTTL(token.Spec.TTL)
	if err != nil {
		return nil, err
	}
	// pass back to caller (Create, Update)
	token.Spec.TTL = ttl

	secret.StringData[FieldClusterName] = token.Spec.ClusterName
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

// fromSecret converts a Secret object into the equivalent Token resource.
func fromSecret(secret *corev1.Secret) (*ext.Token, error) {
	// Basic result. This will be incrementally filled as data is extracted from the secret.
	// On error a partially filled token is returned.
	// See the token store `Delete` (marker **) for where this is important.
	token := &ext.Token{
		TypeMeta: metav1.TypeMeta{
			Kind:       GVK.Kind,
			APIVersion: GV.String(),
		},
		ObjectMeta: *secret.ObjectMeta.DeepCopy(),
	}
	token.Namespace = ""                  // token is not namespaced.
	delete(token.Labels, SecretKindLabel) // Remove an internal label.

	// system - kubernetes uid
	if token.ObjectMeta.UID = types.UID(string(secret.Data[FieldUID])); token.ObjectMeta.UID == "" {
		return nil, fmt.Errorf("kube uid missing")
	}

	// spec - user id, required
	if token.Spec.UserID = string(secret.Data[FieldUserID]); token.Spec.UserID == "" {
		return nil, fmt.Errorf("user id missing")
	}

	// spec - user principal, required
	if err := json.Unmarshal(secret.Data[FieldPrincipal], &token.Spec.UserPrincipal); err != nil {
		return nil, err
	}
	if token.Spec.UserPrincipal.Name == "" {
		return nil, fmt.Errorf("principal id missing")
	}
	if token.Spec.UserPrincipal.Provider == "" {
		return nil, fmt.Errorf("auth provider missing")
	}

	// spec - optional elements
	token.Spec.ClusterName = string(secret.Data[FieldClusterName])
	token.Spec.Description = string(secret.Data[FieldDescription])
	token.Spec.Kind = string(secret.Data[FieldKind])

	enabled, err := strconv.ParseBool(string(secret.Data[FieldEnabled]))
	if err != nil {
		return nil, err
	}
	token.Spec.Enabled = &enabled

	ttl, err := strconv.ParseInt(string(secret.Data[FieldTTL]), 10, 64)
	if err != nil {
		return nil, err
	}
	token.Spec.TTL = ttl

	// status information
	if token.Status.Hash = string(secret.Data[FieldHash]); token.Status.Hash == "" {
		return nil, fmt.Errorf("token hash missing")
	}

	if token.Status.LastUpdateTime = string(secret.Data[FieldLastUpdateTime]); token.Status.LastUpdateTime == "" {
		return nil, fmt.Errorf("last update time missing")
	}

	lastUsedAt, err := decodeTime("lastUsedAt", secret.Data[FieldLastUsedAt])
	if err != nil {
		return nil, err
	}
	token.Status.LastUsedAt = lastUsedAt

	lastActivitySeen, err := decodeTime("lastActivitySeen", secret.Data[FieldLastActivitySeen])
	if err != nil {
		return nil, err
	}
	token.Status.LastActivitySeen = lastActivitySeen

	if err := setExpired(token); err != nil {
		return nil, fmt.Errorf("failed to set expiration information: %w", err)
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

var (
	_ rest.Creater                  = &Store{}
	_ rest.Getter                   = &Store{}
	_ rest.Lister                   = &Store{}
	_ rest.Watcher                  = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.Updater                  = &Store{}
	_ rest.Patcher                  = &Store{}
	_ rest.TableConvertor           = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)
