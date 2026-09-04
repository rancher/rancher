package kubeconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
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
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	ctrlv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	kconfig "github.com/rancher/rancher/pkg/kubeconfig"
	v3node "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	extapi "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/features"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

const (
	Kind               = "Kubeconfig"
	Singular           = "kubeconfig"
	UserIDLabel        = "cattle.io/user-id"
	KindLabel          = "cattle.io/kind"
	KindLabelValue     = "kubeconfig"
	UIDAnnotation      = "cattle.io/uid"
	namespace          = exttokens.TokenNamespace
	unknownValue       = "<unknown>"
	defaultClusterName = "rancher"
	namePrefix         = Singular + "-"
)

// List of fields that hold Kubeconfig data.
const (
	ClustersField            = "clusters"
	CurrentContextField      = "current-context"
	DescriptionField         = "description"
	TTLField                 = "ttl"
	IncludeDefaultEntryField = "include-default-entry"
	StatusConditionsField    = "status-conditions"
	StatusSummaryField       = "status-summary"
	StatusTokensField        = "status-tokens"
)

// List of statuses.
const (
	StatusSummaryPending  = "Pending"
	StatusSummaryComplete = "Complete"
	StatusSummaryError    = "Error"
)

// List of conditions types.
const (
	UpdatedCond                  = "Updated"
	TokenCreatedCond             = "TokenCreated"
	FailedToCreateTokenCond      = "FailedToCreateToken"
	FailedToListClusterNodesCond = "FailedToListClusterNodes"
	FailedToGenerateCond         = "FailedToGenerate"
)

var gvr = ext.SchemeGroupVersion.WithResource(ext.KubeconfigResourceName)

// kindRequirement filters ConfigMaps to those that back Kubeconfig resources.
// It is built once at init time rather than per request.
var kindRequirement = mustRequirement(KindLabel, selection.Equals, KindLabelValue)

// tokenFetcher abstracts the ext token store operations needed by the kubeconfig store.
type tokenFetcher interface {
	Fetch(tokenID string) (accessor.TokenAccessor, error)
	GetSecret(name string, options *metav1.GetOptions, useCache bool) (*corev1.Secret, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

// tokenCreator abstracts ext.Token creation for the kubeconfig store.
type tokenCreator interface {
	CreateToken(ctx context.Context, token *ext.Token, userInfo k8suser.Info) (*ext.Token, error)
}

type extTokenCreator struct{ store *exttokens.SystemStore }

func (e *extTokenCreator) CreateToken(ctx context.Context, token *ext.Token, userInfo k8suser.Info) (*ext.Token, error) {
	return e.store.Create(ctx, gvr.GroupResource(), token, &metav1.CreateOptions{}, userInfo)
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
// Store implements storage for [ext.Kubeconfig].
type Store struct {
	mcmEnabled          bool
	authorizer          authorizer.Authorizer
	configMapCache      v1.ConfigMapCache
	configMapClient     v1.ConfigMapClient
	clusterCache        ctrlv3.ClusterCache
	nsCache             v1.NamespaceCache
	nsClient            v1.NamespaceClient
	nodeCache           ctrlv3.NodeCache
	tokenStore          tokenFetcher
	v3Tokens            ctrlv3.TokenClient
	userCache           ctrlv3.UserCache
	tokenMgr            tokenCreator
	getCACert           func() string
	getDefaultTTL       func() (*int64, error)
	getServerURL        func() string
	shouldGenerateToken func() bool
	tableConverter      rest.TableConvertor
}

// New creates a new instance of [Store].
func New(mcmEnabled bool, wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	extTokenStore := exttokens.NewSystemFromWrangler(wranglerContext, authorizer)
	store := &Store{
		mcmEnabled:      mcmEnabled,
		configMapCache:  wranglerContext.Core.ConfigMap().Cache(),
		configMapClient: wranglerContext.Core.ConfigMap(),
		clusterCache:    wranglerContext.Mgmt.Cluster().Cache(),
		nsCache:         wranglerContext.Core.Namespace().Cache(),
		nsClient:        wranglerContext.Core.Namespace(),
		tokenStore:      extTokenStore,
		v3Tokens:        wranglerContext.Mgmt.Token(),
		userCache:       wranglerContext.Mgmt.User().Cache(),
		tokenMgr:        &extTokenCreator{store: extTokenStore},
		authorizer:      authorizer,
		getCACert:       settings.CACerts.Get,
		getDefaultTTL:   tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds,
		getServerURL:    settings.ServerURL.Get,
		shouldGenerateToken: func() bool {
			return strings.EqualFold(settings.KubeconfigGenerateToken.Get(), "true")
		},
		tableConverter: printerstorage.TableConvertor{
			TableGenerator: printers.NewTableGenerator().With(printHandler),
		},
	}

	if mcmEnabled {
		store.nodeCache = wranglerContext.Mgmt.Node().Cache()
	}

	return store
}

// ensureNamespace ensures that the namespace for storing kubeconfig configMaps exists.
func (s *Store) ensureNamespace() error {
	return extcommon.EnsureNamespace(s.nsCache, s.nsClient, namespace)
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

// New implements [rest.Creater].
func (s *Store) New() runtime.Object {
	return &ext.Kubeconfig{}
}

// userFrom is a helper that extracts and validates the user info from the request's context.
func (s *Store) userFrom(ctx context.Context, verb string) (k8suser.Info, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, false, fmt.Errorf("missing user info")
	}

	// Resource: "*" is a deliberate superuser heuristic, matching the pattern
	// used by the sibling token and useractivity stores. Scoping this check to
	// ext.KubeconfigResourceName would make every default Rancher user an admin
	// because the built-in User/User Base GlobalRoles grant all verbs on
	// ext.cattle.io/kubeconfigs, which would bypass per-user scoping in
	// Get/List/Watch/Delete/Update.
	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
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

// Create implements [rest.Creater].
// Note: Name and GenerateName are not respected. A name is generated with a predefined prefix instead.
func (s *Store) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	userInfo, _, isRancherUser, err := s.userFrom(ctx, "create")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	if !isRancherUser {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userInfo.GetName()))
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

	authToken, err := s.tokenStore.Fetch(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("error getting request token %s: %v", authTokenID, err))
	}

	kubeconfig, ok := obj.(*ext.Kubeconfig)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", obj))
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				return nil, err
			}
			return nil, apierrors.NewBadRequest(fmt.Sprintf("create validation failed for kubeconfig: %s", err))
		}
	}

	if !isUnique(kubeconfig.Spec.Clusters) {
		return nil, apierrors.NewBadRequest("spec.clusters must be unique")
	}

	if !includeDefaultEntry(&kubeconfig.Spec) && len(kubeconfig.Spec.Clusters) == 0 {
		return nil, apierrors.NewBadRequest("at least one cluster is required when includeDefaultEntry is false")
	}

	defaultTTL, err := s.getDefaultTTL()
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting default token TTL: %v", err))
	}
	defaultTTLSeconds := *defaultTTL / 1000

	ttlMilliseconds := kubeconfig.Spec.TTL * 1000
	switch {
	case ttlMilliseconds < 0:
		return nil, apierrors.NewBadRequest("spec.ttl can't be negative")
	case ttlMilliseconds == 0:
		ttlMilliseconds = *defaultTTL
		kubeconfig.Spec.TTL = defaultTTLSeconds
	case ttlMilliseconds > *defaultTTL:
		return nil, apierrors.NewBadRequest(fmt.Sprintf("spec.ttl %d exceeds max ttl %d", kubeconfig.Spec.TTL, defaultTTLSeconds))
	default: // Valid TTL.
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
			return nil, apierrors.NewInternalError(errors.New("can't determine the server URL"))
		}
	}

	var (
		conditions    []metav1.Condition
		tokenIDs      []string
		clusters      []*apiv3.Cluster
		isAllClusters bool // User requested the kubeconfig for all clusters with "*".
	)

	localCluster, err := s.clusterCache.Get("local")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting local cluster: %v", err))
	}

	if len(kubeconfig.Spec.Clusters) > 0 {
		if isAllClusters = kubeconfig.Spec.Clusters[0] == "*"; isAllClusters {
			// The first id in the spec.clusters "*" means all clusters.
			clusters, err = s.clusterCache.List(labels.Everything())
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error listing clusters: %v", err))
			}
		} else {
			// Individually listed clusters.
			for _, clusterID := range kubeconfig.Spec.Clusters {
				if clusterID == "local" { // Shortcut for the local cluster.
					clusters = append(clusters, localCluster)
					continue
				}

				cluster, err := s.clusterCache.Get(clusterID)
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil, apierrors.NewBadRequest(fmt.Sprintf("cluster %s not found", clusterID))
					}
					return nil, apierrors.NewInternalError(fmt.Errorf("error getting cluster %s: %v", clusterID, err))
				}

				clusters = append(clusters, cluster)
			}
		}
	}

	// The name of the cluster to use as the current context.
	// Note that the actual context is set later to the display name of the cluster.
	var currentContext string

	// Check if the user has access to requested clusters before generating tokens.
	// If the user requested all clusters, figure out which clusters they have access to and adjust the list.
	for i := 0; i < len(clusters); i++ {
		decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
			User:            userInfo,
			Verb:            "get",
			APIGroup:        mgmt.GroupName,
			Resource:        apiv3.ClusterResourceName,
			ResourceRequest: true,
			Name:            clusters[i].Name,
		})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error checking if user %s has access to cluster %s: %v", userInfo.GetName(), clusters[i].Name, err))
		}

		if decision != authorizer.DecisionAllow {
			if isAllClusters {
				// Delete the cluster the user doesn't have access to from the list in-place.
				copy(clusters[i:], clusters[i+1:])
				clusters[len(clusters)-1] = nil
				clusters = clusters[:len(clusters)-1]
				i--
				continue
			}

			return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is not allowed to access cluster %s", userInfo.GetName(), clusters[i].Name))
		}

		if currentContext == "" && kubeconfig.Spec.CurrentContext == clusters[i].Name {
			currentContext = clusters[i].Name
		}
	}

	// The current context was requested but wasn't found.
	if currentContext == "" && kubeconfig.Spec.CurrentContext != "" {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid currentContext %s", kubeconfig.Spec.CurrentContext))
	}

	includeDefault := includeDefaultEntry(&kubeconfig.Spec)

	needsSharedToken := includeDefault
	if !needsSharedToken {
		for _, c := range clusters {
			if !c.Spec.LocalClusterAuthEndpoint.Enabled {
				needsSharedToken = true
				break
			}
		}
	}

	dryRun := options != nil && len(options.DryRun) > 0
	generateToken := s.shouldGenerateToken()

	kubeconfigToStore := kubeconfig.DeepCopy()
	kubeconfigToStore.Name = ""         // We generate the kubeconfig's name automatically.
	kubeconfigToStore.GenerateName = "" // We generate the kubeconfig's name automatically.
	if kubeconfigToStore.Labels == nil {
		kubeconfigToStore.Labels = make(map[string]string)
	}
	kubeconfigToStore.Labels[UserIDLabel] = userInfo.GetName()
	kubeconfigToStore.UID = uuid.NewUUID() // Generate a UID for the kubeconfig, which is then stored as an annotation in the corresponding ConfigMap.
	kubeconfigToStore.Status.Summary = StatusSummaryPending

	configMap, err := s.toConfigMap(kubeconfigToStore)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig to configmap: %v", err))
	}
	configMap.GenerateName = namePrefix

	var kubeConfigID string
	if dryRun {
		kubeConfigID = names.SimpleNameGenerator.GenerateName(namePrefix)
		configMap.Name = kubeConfigID
	} else {
		if err = s.ensureNamespace(); err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error ensuring namespace %s: %v", namespace, err))
		}

		configMap, err = s.configMapClient.Create(configMap)
		if err != nil {
			return nil, mapBackingError(err, namePrefix)
		}
		kubeConfigID = configMap.Name
	}

	kubeconfigToStore, err = s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", kubeConfigID, err))
	}

	var (
		sharedTokenKey string
		ownerRefs      []metav1.OwnerReference
		v1Config       string
	)

	caCert := kconfig.FormatCertString(base64.StdEncoding.EncodeToString([]byte(s.getCACert())))
	data := kconfig.KubeConfig{
		Meta: kconfig.Meta{
			Name:              kubeConfigID,
			CreationTimestamp: configMap.CreationTimestamp.Format(time.RFC3339),
			TTL:               strconv.FormatInt(kubeconfig.Spec.TTL, 10),
		},
		CurrentContext: func() string {
			if includeDefault {
				return defaultClusterName
			}
			return ""
		}(),
	}

	err = func() error { // Deliberately use an anonymous function to capture the status and error conditions.
		// Generate a shared token for the default and non-ACE clusters.
		if !dryRun && generateToken && needsSharedToken {
			extToken := s.buildExtToken(userInfo.GetName(), authToken, ttlMilliseconds, "", kubeConfigID)
			sharedToken, err := s.tokenMgr.CreateToken(ctx, extToken, userInfo)
			if err != nil {
				conditions = append(conditions, metav1.Condition{
					Type:               FailedToCreateTokenCond,
					Status:             metav1.ConditionTrue,
					Reason:             FailedToCreateTokenCond,
					Message:            fmt.Sprintf("error creating kubeconfig token: %s", err),
					LastTransitionTime: metav1.NewTime(time.Now()),
				})

				return apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token: %v", err))
			}

			sharedTokenKey = sharedToken.Status.BearerToken
			// Note the token down before anything else can go wrong. Both the
			// stored record and the cleanup below work off tokenIDs, so a token
			// that never gets added to it can no longer be deleted.
			tokenIDs = append(tokenIDs, sharedToken.Name)

			ownerRef, err := s.secretOwnerRef(sharedToken.Name)
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("error getting owner reference for shared token: %v", err))
			}
			ownerRefs = append(ownerRefs, ownerRef)

			conditions = append(conditions, metav1.Condition{
				Type:               TokenCreatedCond,
				Status:             metav1.ConditionTrue,
				Reason:             TokenCreatedCond,
				Message:            sharedToken.Name,
				LastTransitionTime: metav1.NewTime(time.Now()),
			})
		}

		if includeDefault {
			data.Clusters = append(data.Clusters, kconfig.Cluster{
				Name:   defaultClusterName,
				Server: "https://" + host,
				Cert:   caCert,
			})
			data.Users = append(data.Users, kconfig.User{
				Name:  defaultClusterName,
				Token: sharedTokenKey,
				Host:  host,
			})
			data.Contexts = append(data.Contexts, kconfig.Context{
				Name:    defaultClusterName,
				Cluster: defaultClusterName,
				User:    defaultClusterName,
			})
		} else if needsSharedToken {
			data.Users = append(data.Users, kconfig.User{
				Name:  defaultClusterName,
				Token: sharedTokenKey,
				Host:  host,
			})
		}

		for _, cluster := range clusters {
			var tokenKey string

			clusterName := cluster.Name
			if name := cluster.Spec.DisplayName; name != "" {
				// Use cluster display name if available.
				clusterName = name
			}

			// Both ACE and non-ACE clusters should have an entry that points to the Rancher proxy.
			data.Clusters = append(data.Clusters, kconfig.Cluster{
				Name:   clusterName,
				Server: "https://" + host + "/k8s/clusters/" + cluster.Name,
				Cert:   caCert,
			})

			if currentContext == "" {
				currentContext = cluster.Name // Set the first cluster as the current context.
			}
			if currentContext == cluster.Name {
				data.CurrentContext = clusterName // Use the display name as the context name.
				kubeconfigToStore.Spec.CurrentContext = currentContext
			}

			if !cluster.Spec.LocalClusterAuthEndpoint.Enabled {
				data.Contexts = append(data.Contexts, kconfig.Context{
					Name:    clusterName,
					Cluster: clusterName,
					User:    defaultClusterName, // Reuse the auth info with the shared token.
				})

				continue
			}

			// Generate a cluster-scoped token for the ACE cluster.
			if !dryRun && generateToken {
				extToken := s.buildExtToken(userInfo.GetName(), authToken, ttlMilliseconds, cluster.Name, kubeConfigID)
				clusterToken, err := s.tokenMgr.CreateToken(ctx, extToken, userInfo)
				if err != nil {
					conditions = append(conditions, metav1.Condition{
						Type:               FailedToCreateTokenCond,
						Status:             metav1.ConditionTrue,
						Reason:             FailedToCreateTokenCond,
						Message:            fmt.Sprintf("error creating kubeconfig token for cluster %s: %s", cluster.Name, err),
						LastTransitionTime: metav1.NewTime(time.Now()),
					})

					return apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token for cluster %s: %v", cluster.Name, err))
				}

				tokenKey = clusterToken.Status.BearerToken
				tokenIDs = append(tokenIDs, clusterToken.Name) // See the shared token above.

				ownerRef, err := s.secretOwnerRef(clusterToken.Name)
				if err != nil {
					return apierrors.NewInternalError(fmt.Errorf("error getting owner reference for token: %v", err))
				}
				ownerRefs = append(ownerRefs, ownerRef)

				conditions = append(conditions, metav1.Condition{
					Type:               TokenCreatedCond,
					Status:             metav1.ConditionTrue,
					Reason:             TokenCreatedCond,
					Message:            clusterToken.Name,
					LastTransitionTime: metav1.NewTime(time.Now()),
				})
			}

			data.Contexts = append(data.Contexts, kconfig.Context{
				Name:    clusterName,
				Cluster: clusterName,
				User:    clusterName,
			})
			data.Users = append(data.Users, kconfig.User{
				Name:      clusterName,
				Token:     tokenKey,
				Host:      host,
				ClusterID: cluster.Name,
			})

			if s.mcmEnabled { // Nodes are only available if MCM is enabled.
				// If the ACE cluster has a FQDN, add a single entry for it.
				if authEndpoint := cluster.Spec.LocalClusterAuthEndpoint; authEndpoint.FQDN != "" {
					fqdnName := clusterName + "-fqdn"
					data.Clusters = append(data.Clusters, kconfig.Cluster{
						Name:   fqdnName,
						Server: "https://" + authEndpoint.FQDN,
						Cert:   kconfig.FormatCertString(base64.StdEncoding.EncodeToString([]byte(authEndpoint.CACerts))),
					})
					data.Contexts = append(data.Contexts, kconfig.Context{
						Name:    fqdnName,
						Cluster: fqdnName,
						User:    clusterName,
					})

					if currentContext == cluster.Name {
						data.CurrentContext = fqdnName
					}

					continue
				}

				// Otherwise produce entries for each control plane node.
				nodes, err := s.nodeCache.List(cluster.Name, labels.Everything())
				if err != nil {
					conditions = append(conditions, metav1.Condition{
						Type:               FailedToListClusterNodesCond,
						Status:             metav1.ConditionTrue,
						Reason:             FailedToListClusterNodesCond,
						Message:            fmt.Sprintf("error listing nodes for cluster %s: %s", cluster.Name, err),
						LastTransitionTime: metav1.NewTime(time.Now()),
					})

					return apierrors.NewInternalError(fmt.Errorf("error listing nodes for cluster %s: %v", cluster.Name, err))
				}

				clusterCerts := kconfig.FormatCertString(cluster.Status.CACert) // Already base64 encoded.
				var isCurrentContextSet bool
				for _, node := range nodes {
					if !node.Spec.ControlPlane {
						continue
					}

					nodeName := clusterName + "-" + strings.TrimPrefix(node.Spec.RequestedHostname, clusterName+"-")
					data.Clusters = append(data.Clusters, kconfig.Cluster{
						Name:   nodeName,
						Server: "https://" + v3node.GetEndpointNodeIP(node) + ":6443",
						Cert:   clusterCerts,
					})
					data.Contexts = append(data.Contexts, kconfig.Context{
						Name:    nodeName,
						Cluster: nodeName,
						User:    clusterName,
					})

					if !isCurrentContextSet && currentContext == cluster.Name && v3node.IsMachineReady(node) {
						data.CurrentContext = nodeName // Set the current context to the first ready control plane node.
						isCurrentContextSet = true
					}
				}
			}
		}

		v1Config, err = kconfig.Generate(data)
		if err != nil {
			conditions = []metav1.Condition{{
				Type:               FailedToGenerateCond,
				Status:             metav1.ConditionTrue,
				Reason:             FailedToGenerateCond,
				Message:            fmt.Sprintf("error generating kubeconfig content: %s", err),
				LastTransitionTime: metav1.NewTime(time.Now()),
			}}

			return apierrors.NewInternalError(fmt.Errorf("error generating kubeconfig content: %v", err))
		}

		return nil
	}()

	statusSummary := StatusSummaryComplete
	if err != nil {
		statusSummary = StatusSummaryError
	}

	kubeconfigToStore.Status.Summary = statusSummary
	kubeconfigToStore.Status.Conditions = append(kubeconfigToStore.Status.Conditions, conditions...)
	kubeconfigToStore.Status.Tokens = tokenIDs
	kubeconfigToStore.OwnerReferences = append(kubeconfigToStore.OwnerReferences, ownerRefs...)

	// What the kubeconfig ended up as, a failed attempt included, is saved back
	// to the ConfigMap created above. Keeping a failed one is on purpose: it
	// lists the tokens that were created, so the user can still delete the
	// kubeconfig and have those tokens deleted with it. That only works if this
	// last save succeeds, which is what recorded tracks.
	recorded := dryRun

	desiredConfigMap, convertErr := s.toConfigMap(kubeconfigToStore)
	switch {
	case convertErr != nil:
		if err == nil {
			err = apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig %s to configmap: %v", kubeConfigID, convertErr))
		} // else preserve the original error.
	case dryRun:
		configMap = desiredConfigMap
	default:
		finalized, finalizeErr := s.finalizeConfigMap(configMap, desiredConfigMap)
		if finalizeErr != nil {
			if err == nil {
				err = mapBackingError(finalizeErr, kubeConfigID)
			} // else preserve the original error.
			break
		}
		configMap, recorded = finalized, true
	}

	if err != nil {
		if !recorded {
			// Nothing points at the tokens that were created: their IDs never
			// reached the ConfigMap, so deleting the kubeconfig later won't
			// delete them either. Remove them here along with the unfinished
			// record, so that a create which reports a failure doesn't leave
			// working credentials behind.
			s.cleanupIncompleteKubeconfig(kubeConfigID, tokenIDs)
		}
		return nil, err
	}

	kubeconfig, err = s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig after saving: %v", kubeConfigID, err))
	}

	// Note: Status.Value contains tokens' secret keys and mustn't be persisted.
	kubeconfig.Status.Value = v1Config

	return kubeconfig, nil
}

// toConfigMap converts a Kubeconfig object to a ConfigMap object.
func (s *Store) toConfigMap(kubeconfig *ext.Kubeconfig) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: *kubeconfig.ObjectMeta.DeepCopy(),
		Data:       make(map[string]string),
	}
	configMap.Namespace = namespace
	configMap.UID = ""

	if configMap.Annotations == nil {
		configMap.Annotations = make(map[string]string)
	}
	configMap.Annotations[UIDAnnotation] = string(kubeconfig.UID)

	if configMap.Labels == nil {
		configMap.Labels = make(map[string]string)
	}
	configMap.Labels[KindLabel] = KindLabelValue

	if len(kubeconfig.Spec.Clusters) > 0 {
		serialized, err := json.Marshal(kubeconfig.Spec.Clusters)
		if err != nil {
			return nil, fmt.Errorf("error serializing spec.clusters: %w", err)
		}
		configMap.Data[ClustersField] = string(serialized)
	}

	configMap.Data[CurrentContextField] = kubeconfig.Spec.CurrentContext
	configMap.Data[DescriptionField] = kubeconfig.Spec.Description
	configMap.Data[TTLField] = strconv.FormatInt(kubeconfig.Spec.TTL, 10)

	if kubeconfig.Spec.IncludeDefaultEntry != nil {
		configMap.Data[IncludeDefaultEntryField] = strconv.FormatBool(*kubeconfig.Spec.IncludeDefaultEntry)
	}

	// Note: Value should never be persisted!
	configMap.Data[StatusSummaryField] = kubeconfig.Status.Summary
	if len(kubeconfig.Status.Conditions) > 0 {
		serialized, err := json.Marshal(kubeconfig.Status.Conditions)
		if err != nil {
			return nil, fmt.Errorf("error serializing status.conditions: %w", err)
		}
		configMap.Data[StatusConditionsField] = string(serialized)
	}
	if len(kubeconfig.Status.Tokens) > 0 {
		serialized, err := json.Marshal(kubeconfig.Status.Tokens)
		if err != nil {
			return nil, fmt.Errorf("error serializing status.tokens: %w", err)
		}
		configMap.Data[StatusTokensField] = string(serialized)
	}

	var err error
	configMap.ObjectMeta.ManagedFields, err = extcommon.MapManagedFields(mapFromKubeconfig,
		kubeconfig.ObjectMeta.ManagedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to map kubeconfig managed-fields: %w", err)
	}

	return configMap, nil
}

// finalizeConfigMap saves the finished kubeconfig into the ConfigMap that
// [Store.Create] already made for it.
//
// Create is a two-phase write. The first phase stores an empty record and gets
// back the generated name, which the tokens need. The second phase, done here,
// saves the tokens and the rest of the finished kubeconfig. Only the store knows
// about the first phase: the client never sees the resourceVersion it produced,
// and retrying the request would make a whole new ConfigMap and a whole new set
// of tokens rather than repeat this one write. So when something else changes
// the ConfigMap in between - another controller adding a label to it, for
// example - the request must not fail. Read the ConfigMap again instead and
// write the fields the store owns onto the version that is now stored.
func (s *Store) finalizeConfigMap(created, desired *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	var (
		latest    = created
		finalized *corev1.ConfigMap
	)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if latest == nil {
			refreshed, err := s.configMapClient.Get(namespace, created.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			latest = refreshed
		}

		updated, err := s.configMapClient.Update(mergeConfigMap(latest, desired))
		if err != nil {
			latest = nil // Read the ConfigMap again before the next attempt.
			return err
		}

		finalized = updated
		return nil
	})
	if err != nil {
		return nil, err
	}

	return finalized, nil
}

// mergeConfigMap copies the fields the store owns onto the ConfigMap as it is
// currently stored, leaving whatever anything else wrote there in place. The
// result has the current resourceVersion and can be sent as an update.
//
// Data and metadata are handled differently on purpose. Every key under data
// belongs to the store, so the whole map is replaced and a key the store has
// stopped writing goes away. Labels and annotations are shared with anything
// else that touches the ConfigMap, so a key the store doesn't write is far more
// likely to belong to someone else than to be one of the store's own left over
// from an earlier version. Those are only added or overwritten, never removed,
// which means dropping a label or an annotation the store used to set needs an
// explicit delete here.
func mergeConfigMap(latest, desired *corev1.ConfigMap) *corev1.ConfigMap {
	merged := latest.DeepCopy()
	// Cloned rather than shared: desired is built once and reused for every
	// attempt, so it must not end up pointing at a map that was handed to the
	// client.
	merged.Data = maps.Clone(desired.Data)

	if merged.Labels == nil && len(desired.Labels) > 0 {
		merged.Labels = make(map[string]string, len(desired.Labels))
	}
	maps.Copy(merged.Labels, desired.Labels)

	if merged.Annotations == nil && len(desired.Annotations) > 0 {
		merged.Annotations = make(map[string]string, len(desired.Annotations))
	}
	maps.Copy(merged.Annotations, desired.Annotations)

	merged.OwnerReferences = mergeOwnerReferences(merged.OwnerReferences, desired.OwnerReferences)

	// ManagedFields are left as the API server reported them: it works them out
	// again on every update and ignores whatever the request carries.

	return merged
}

// mergeOwnerReferences returns the two lists of owner references combined,
// matching entries up by UID. Where the same UID is in both lists, the entry
// from desired is the one kept.
//
// Both lists have to survive, which is why this isn't a plain replace like
// [mergeConfigMap] does for data. The references the store writes are what makes
// Kubernetes delete the ConfigMap once the tokens it points at are gone, so
// losing them would leave the record behind forever. A reference that only the
// stored ConfigMap has was put there by something else between the two writes,
// and dropping it would break whatever that something else is relying on.
//
// It isn't a plain concatenation either. The desired list is built from the
// ConfigMap the first write returned, so the references that were already on it
// are in both lists, and appending would list them twice.
func mergeOwnerReferences(existing, desired []metav1.OwnerReference) []metav1.OwnerReference {
	if len(desired) == 0 {
		return existing
	}

	merged := make([]metav1.OwnerReference, 0, len(existing)+len(desired))
	indexByUID := make(map[types.UID]int, len(existing)+len(desired))

	for _, refs := range [][]metav1.OwnerReference{existing, desired} {
		for _, ref := range refs {
			if i, ok := indexByUID[ref.UID]; ok {
				merged[i] = ref
				continue
			}
			indexByUID[ref.UID] = len(merged)
			merged = append(merged, ref)
		}
	}

	return merged
}

// cleanupIncompleteKubeconfig removes what a failed [Store.Create] left behind:
// the tokens it had already created and the ConfigMap still holding the
// unfinished record. Create is already returning an error, so anything that
// goes wrong here is logged rather than reported.
func (s *Store) cleanupIncompleteKubeconfig(name string, tokenIDs []string) {
	for _, tokenID := range tokenIDs {
		if err := s.deleteToken(tokenID, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("kubeconfig: error deleting token %s of incomplete kubeconfig %s: %v", tokenID, name, err)
		}
	}

	if err := s.configMapClient.Delete(namespace, name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("kubeconfig: error deleting configmap of incomplete kubeconfig %s: %v", name, err)
	}
}

// deleteToken removes one of a kubeconfig's tokens, falling back to the v3 token
// client for tokens created before the ext token store existed. A token that is
// already gone is not an error.
func (s *Store) deleteToken(name string, options *metav1.DeleteOptions) error {
	err := s.tokenStore.Delete(name, options)
	if apierrors.IsNotFound(err) {
		err = s.v3Tokens.Delete(name, options)
	}
	if apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

// fromConfigMap converts a ConfigMap object to a Kubeconfig object.
func (s *Store) fromConfigMap(configMap *corev1.ConfigMap) (*ext.Kubeconfig, error) {
	kubeconfig := &ext.Kubeconfig{
		ObjectMeta: *configMap.ObjectMeta.DeepCopy(),
		Spec: ext.KubeconfigSpec{
			Description:    configMap.Data[DescriptionField],
			CurrentContext: configMap.Data[CurrentContextField],
		},
	}
	kubeconfig.Namespace = ""            // Kubeconfig is not namespaced.
	delete(kubeconfig.Labels, KindLabel) // Remove an internal label.

	if kubeconfig.Annotations != nil {
		uid, ok := kubeconfig.Annotations[UIDAnnotation]

		if ok {
			kubeconfig.UID = types.UID(uid)
			delete(kubeconfig.Annotations, UIDAnnotation) // Remove an internal annotation.
		}
	}

	ttl, err := strconv.ParseInt(configMap.Data[TTLField], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing TTL for %s: %w", configMap.Name, err)
	}
	kubeconfig.Spec.TTL = ttl

	if serialized := configMap.Data[ClustersField]; serialized != "" {
		err = json.Unmarshal([]byte(serialized), &kubeconfig.Spec.Clusters)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling spec.clusters for %s: %w", configMap.Name, err)
		}
	}

	if v, ok := configMap.Data[IncludeDefaultEntryField]; ok {
		boolVal, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("error parsing includeDefaultEntry for %s: %w", configMap.Name, err)
		}
		kubeconfig.Spec.IncludeDefaultEntry = &boolVal
	}

	kubeconfig.Status.Summary = configMap.Data[StatusSummaryField]

	if serialized := configMap.Data[StatusConditionsField]; serialized != "" {
		err = json.Unmarshal([]byte(serialized), &kubeconfig.Status.Conditions)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling status.conditions for %s: %w", configMap.Name, err)
		}
	}

	if serialized := configMap.Data[StatusTokensField]; serialized != "" {
		err = json.Unmarshal([]byte(serialized), &kubeconfig.Status.Tokens)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling status.tokens for %s: %w", configMap.Name, err)
		}
	}

	kubeconfig.ObjectMeta.ManagedFields, err = extcommon.MapManagedFields(mapFromConfigMap,
		kubeconfig.ObjectMeta.ManagedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to map configmap managed-fields: %w", err)
	}

	return kubeconfig, nil
}

func includeDefaultEntry(spec *ext.KubeconfigSpec) bool {
	return spec.IncludeDefaultEntry == nil || *spec.IncludeDefaultEntry
}

// first returns the first element of a slice of strings, or an empty string if the slice is empty.
func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// buildExtToken builds an [ext.Token] for a kubeconfig token.
func (s *Store) buildExtToken(userName string, authToken accessor.TokenAccessor, ttl int64, clusterID, kubeConfigID string) *ext.Token {
	return &ext.Token{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				tokens.TokenKubeconfigIDLabel: kubeConfigID,
			},
		},
		Spec: ext.TokenSpec{
			UserID:        userName,
			UserPrincipal: toExtTokenPrincipal(authToken.GetUserPrincipal()),
			Kind:          KindLabelValue,
			Description:   "Kubeconfig token",
			TTL:           ttl,
			ClusterName:   clusterID,
		},
	}
}

// toExtTokenPrincipal converts a [apiv3.Principal] to an [ext.TokenPrincipal].
func toExtTokenPrincipal(p apiv3.Principal) ext.TokenPrincipal {
	return ext.TokenPrincipal{
		Name:           p.Name,
		DisplayName:    p.DisplayName,
		LoginName:      p.LoginName,
		ProfilePicture: p.ProfilePicture,
		ProfileURL:     p.ProfileURL,
		PrincipalType:  p.PrincipalType,
		Provider:       p.Provider,
		ExtraInfo:      p.ExtraInfo,
	}
}

// secretOwnerRef returns a [metav1.OwnerReference] pointing to the backing v1/Secret for an ext token.
// Kubernetes GC only follows native resource owner references; using the Secret ensures proper cleanup.
func (s *Store) secretOwnerRef(tokenName string) (metav1.OwnerReference, error) {
	secret, err := s.tokenStore.GetSecret(tokenName, &metav1.GetOptions{}, false)
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("error getting backing secret for token %s: %w", tokenName, err)
	}
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       secret.Name,
		UID:        secret.UID,
	}, nil
}

// getConfigMap retrieves a ConfigMap by name, optionally using the cache.
func (s *Store) getConfigMap(name string, options *metav1.GetOptions, useCache bool) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
	)

	if useCache {
		configMap, err = s.configMapCache.Get(namespace, name)
	} else {
		configMap, err = s.configMapClient.Get(namespace, name, *options)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
		return nil, fmt.Errorf("error getting configmap %s: %w", name, err)
	}

	if configMap.Labels[KindLabel] != KindLabelValue {
		return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	return configMap, nil
}

// Get implements [rest.Getter].
func (s *Store) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "get")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	var emptyGetOptions metav1.GetOptions
	useCache := options == nil || *options == emptyGetOptions
	configMap, err := s.getConfigMap(name, options, useCache)
	if err != nil {
		return nil, err
	}

	if configMap.Labels[UserIDLabel] != userInfo.GetName() && !isAdmin {
		// An ordinary user can only access their own kubeconfigs.
		// We return a NotFound error to avoid leaking information about other users' kubeconfigs.
		return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	kubeconfig, err := s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", name, err))
	}

	return kubeconfig, nil
}

// List implements [rest.Lister].
func (s *Store) NewList() runtime.Object {
	return &ext.KubeconfigList{}
}

func mustRequirement(key string, op selection.Operator, val string) labels.Requirement {
	r, err := labels.NewRequirement(key, op, []string{val})
	if err != nil {
		panic(fmt.Sprintf("kubeconfig: invalid label requirement %s %s %s: %v", key, op, val, err))
	}
	return *r
}

func toListOptions(options *metainternalversion.ListOptions, userInfo k8suser.Info, isAdmin bool) (*metav1.ListOptions, error) {
	listOptions, err := extapi.ConvertListOptions(options)
	if err != nil {
		return nil, fmt.Errorf("error converting list options: %w", err)
	}

	selector := labels.Everything()
	if listOptions.LabelSelector != "" {
		parsed, err := labels.Parse(listOptions.LabelSelector)
		if err != nil {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid label selector: %s", err))
		}
		selector = parsed
	}

	reqs := []labels.Requirement{kindRequirement}
	if !isAdmin {
		userReq, err := labels.NewRequirement(UserIDLabel, selection.Equals, []string{userInfo.GetName()})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("user ID %q is not a valid label value: %v", userInfo.GetName(), err))
		}
		reqs = append(reqs, *userReq)
	}
	selector = selector.Add(reqs...)
	listOptions.LabelSelector = selector.String()

	return listOptions, nil
}

// List implements [rest.Lister].
func (s *Store) List(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "list")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, apiStatusOrInternalError(err)
	}

	configMapList, err := s.configMapClient.List(namespace, *listOptions)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) { // Continue token expired.
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("error listing configmaps for kubeconfigs: %v", err))
	}

	list := &ext.KubeconfigList{
		ListMeta: metav1.ListMeta{
			Continue:           configMapList.Continue,
			ResourceVersion:    configMapList.ResourceVersion,
			RemainingItemCount: configMapList.RemainingItemCount,
		},
		Items: make([]ext.Kubeconfig, 0, len(configMapList.Items)),
	}
	for _, configMap := range configMapList.Items {
		kubeconfig, err := s.fromConfigMap(&configMap)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", configMap.Name, err))
		}

		list.Items = append(list.Items, *kubeconfig)
	}

	return list, nil
}

// Watch implements [rest.Watcher].
func (s *Store) Watch(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (watch.Interface, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "watch")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, apiStatusOrInternalError(err)
	}

	if !features.FeatureGates().Enabled(features.WatchListClient) {
		listOptions.SendInitialEvents = nil
		listOptions.ResourceVersionMatch = ""
	}

	configMapWatch, err := s.configMapClient.Watch(namespace, *listOptions)
	if err != nil {
		logrus.Errorf("kubeconfig: watch: error starting watch: %s", err)
		return nil, apierrors.NewInternalError(fmt.Errorf("kubeconfig: watch: error starting watch: %v", err))
	}

	kubeconfigWatch := &watcher{
		ch:   make(chan watch.Event, 100),
		done: make(chan struct{}),
	}

	go func() {
		defer configMapWatch.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, more := <-configMapWatch.ResultChan():
				if !more {
					return
				}

				var obj runtime.Object
				switch event.Type {
				case watch.Bookmark:
					configMap, ok := event.Object.(*corev1.ConfigMap)
					if !ok {
						logrus.Warnf("kubeconfig: watch: expected configmap got %T", event.Object)
						continue
					}

					// Rebuild the bookmark's metadata as an allowlist instead of
					// copying the backing ConfigMap's annotations: passing them
					// through leaked internal bookkeeping (the cattle.io/uid
					// annotation) that every other event type strips, and a
					// copy-and-delete approach would silently leak any internal
					// annotation added later. Only two things are justified on a
					// bookmark: the resourceVersion resume point, and the
					// k8s.io/initial-events-end marker that tells a WatchList
					// (sendInitialEvents) client the initial-state replay is
					// complete — dropping the marker would leave such a client
					// waiting forever.
					anns := map[string]string{}
					if v, ok := configMap.Annotations["k8s.io/initial-events-end"]; ok {
						anns["k8s.io/initial-events-end"] = v
					}
					obj = &ext.Kubeconfig{ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: configMap.ResourceVersion,
						Annotations:     anns,
					}}
				case watch.Error:
					// Pass through the errors e.g. 410 Expired.
					obj = event.Object
				case watch.Added, watch.Modified, watch.Deleted:
					configMap, ok := event.Object.(*corev1.ConfigMap)
					if !ok {
						logrus.Warnf("kubeconfig: watch: expected configmap got %T", event.Object)
						continue
					}

					obj, err = s.fromConfigMap(configMap)
					if err != nil {
						logrus.Errorf("kubeconfig: watch: error converting configmap %s to kubeconfig: %s", configMap.Name, err)
						continue
					}
				default:
					// watch.EventType is an open string type; unknown types pass through untranslated.
					obj = event.Object
				}

				if !kubeconfigWatch.add(watch.Event{
					Type:   event.Type,
					Object: obj,
				}) {
					return
				}
			}
		}
	}()

	return kubeconfigWatch, nil
}

// watcher implements [watch.Interface].
type watcher struct {
	mu       sync.RWMutex
	closed   bool
	ch       chan watch.Event
	done     chan struct{}
	stopOnce sync.Once
}

// Stop tells the producer that the consumer is done watching, so the
// producer should stop sending events and close the result channel.
func (w *watcher) Stop() {
	// Close done BEFORE taking the write lock: a blocked add holds RLock
	// until done closes, so taking Lock first would deadlock — the exact
	// bug this change fixes.
	w.stopOnce.Do(func() { close(w.done) })

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	close(w.ch)
	w.closed = true
}

// ResultChan returns a channel which will receive events from the event producer.
func (w *watcher) ResultChan() <-chan watch.Event {
	return w.ch
}

// add pushes a new event to the Result channel.
func (w *watcher) add(event watch.Event) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return false
	}
	select {
	case w.ch <- event:
		return true
	case <-w.done:
		return false
	}
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return unknownValue
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// ConvertToTable implements [rest.TableConvertor].
func (s *Store) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return s.tableConverter.ConvertToTable(ctx, object, tableOptions)
}

// printHandler registers the table printer for Kubeconfig objects.
func printHandler(h printers.PrintHandler) {
	columnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "TTL", Type: "string", Description: "TTL is the time-to-live for the Kubeconfig tokens"},
		{Name: "Tokens", Type: "string", Description: "Tokens is the number of tokens created for the Kubeconfig"},
		{Name: "Status", Type: "string", Description: "Status is the most recently observed status of the Kubeconfig"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "User", Type: "string", Priority: 1, Description: "User is the owner of the Kubeconfig"},
		{Name: "Clusters", Type: "string", Priority: 1, Description: "Clusters is a list of clusters in the Kubeconfig"},
		{Name: "Description", Type: "string", Priority: 1, Description: "Description is a human readable description of the Kubeconfig"},
	}
	_ = h.TableHandler(columnDefinitions, printKubeconfigList)
	_ = h.TableHandler(columnDefinitions, printKubeconfig)
}

// printKubeconfig prints a single Kubeconfig object as a table row.
func printKubeconfig(kubeconfig *ext.Kubeconfig, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	status := unknownValue
	allTokenCount := 0
	if kubeconfig.Status.Summary != "" {
		status = kubeconfig.Status.Summary
	}

	allTokenCount = len(kubeconfig.Status.Tokens)

	var ownedTokenCount int
	for _, ref := range kubeconfig.OwnerReferences {
		if (ref.Kind == "Secret" && ref.APIVersion == "v1") ||
			(ref.Kind == "Token" && ref.APIVersion == "management.cattle.io/v3") {
			ownedTokenCount++
		}
	}
	tokens := strconv.Itoa(ownedTokenCount) + "/" + strconv.Itoa(allTokenCount)

	return []metav1.TableRow{{
		Object: runtime.RawExtension{Object: kubeconfig},
		Cells: []any{
			kubeconfig.Name,
			duration.HumanDuration(time.Duration(kubeconfig.Spec.TTL) * time.Second),
			tokens,
			status,
			translateTimestampSince(kubeconfig.CreationTimestamp),
			kubeconfig.Labels[UserIDLabel],
			strings.Join(kubeconfig.Spec.Clusters, ","),
			kubeconfig.Spec.Description,
		},
	}}, nil
}

// printKubeconfigList prints a list of Kubeconfig objects as table rows.
func printKubeconfigList(kubeconfigList *ext.KubeconfigList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(kubeconfigList.Items))
	for i := range kubeconfigList.Items {
		r, err := printKubeconfig(&kubeconfigList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

// DeleteCollection implements [rest.CollectionDeleter]
func (s *Store) DeleteCollection(
	ctx context.Context,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
	listOptions *metainternalversion.ListOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "delete")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	lOptions, err := toListOptions(listOptions, userInfo, isAdmin)
	if err != nil {
		return nil, apiStatusOrInternalError(err)
	}

	configMapList, err := s.configMapClient.List(namespace, *lOptions)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) { // Continue token expired.
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("error listing configmaps for kubeconfigs: %v", err))
	}

	list := &ext.KubeconfigList{
		ListMeta: metav1.ListMeta{
			Continue:           configMapList.Continue,
			ResourceVersion:    configMapList.ResourceVersion,
			RemainingItemCount: configMapList.RemainingItemCount,
		},
		Items: make([]ext.Kubeconfig, 0, len(configMapList.Items)),
	}

	for _, configMap := range configMapList.Items {
		// Convert to Kubeconfig first and run deleteValidation against it directly.
		// Any error from deleteValidation (including a NotFound-classified one) is
		// provably from the caller's validation logic, not from a concurrent delete,
		// so it aborts the whole collection and is surfaced verbatim.
		kubeconfig, err := s.fromConfigMap(&configMap)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", configMap.Name, err))
		}
		if deleteValidation != nil {
			if err := deleteValidation(ctx, kubeconfig); err != nil {
				if _, ok := err.(apierrors.APIStatus); ok {
					return nil, err
				}
				return nil, apierrors.NewBadRequest(fmt.Sprintf("delete validation for kubeconfig %s failed: %s", configMap.Name, err))
			}
		}
		// Pass nil deleteValidation: validation already ran above, so an IsNotFound
		// from s.delete is provably the backing concurrent-delete race.
		obj, _, err := s.delete(ctx, kubeconfig, &configMap, nil, options)
		if apierrors.IsNotFound(err) {
			continue // backing ConfigMap was concurrently deleted — skip
		}
		if err != nil {
			return nil, err // already Kubeconfig-scoped via mapBackingError
		}
		kc, ok := obj.(*ext.Kubeconfig)
		if !ok {
			return nil, apierrors.NewInternalError(fmt.Errorf("expected kubeconfig object, got %T", obj))
		}
		list.Items = append(list.Items, *kc)
	}

	return list, nil
}

// Delete implements [rest.GracefulDeleter].
func (s *Store) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "delete")
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	useCache := false
	configMap, err := s.getConfigMap(name, &metav1.GetOptions{}, useCache)
	if err != nil {
		return nil, false, err // The err is already an [apierrors.APIStatus].
	}

	if configMap.Labels[UserIDLabel] != userInfo.GetName() && !isAdmin {
		// An ordinary user can only access their own kubeconfigs.
		// We return a NotFound error to avoid leaking information about other users' kubeconfigs.
		return nil, false, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	kubeconfig, err := s.fromConfigMap(configMap)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", configMap.Name, err))
	}
	return s.delete(ctx, kubeconfig, configMap, deleteValidation, options)
}

// checkPreconditions verifies the caller's UID and ResourceVersion preconditions
// against the stored values. It returns a Conflict if a check fails and nil when
// all checks pass or preconditions is nil.
//
// The UID precondition passes if the submitted UID equals ANY of the accepted
// UIDs: callers may pass both the Kubeconfig's virtual UID and the backing
// ConfigMap's real UID so a client holding either identity can enforce uniqueness.
//
// Note: the default apiserver UpdatedObjectInfo only ever populates the UID
// precondition; the ResourceVersion branch is defensive for non-default
// rest.UpdatedObjectInfo implementations, but kept for the standard
// Preconditions contract.
func checkPreconditions(preconditions *metav1.Preconditions, name, rv string, uids ...types.UID) error {
	if preconditions == nil {
		return nil
	}
	if preconditions.UID != nil {
		var matched bool
		for _, uid := range uids {
			if *preconditions.UID == uid {
				matched = true
				break
			}
		}
		if !matched {
			var recordUID types.UID
			if len(uids) > 0 {
				recordUID = uids[0]
			}
			return apierrors.NewConflict(gvr.GroupResource(), name,
				fmt.Errorf("the UID in the precondition (%s) does not match the UID in record (%s)",
					*preconditions.UID, recordUID))
		}
	}
	if preconditions.ResourceVersion != nil && *preconditions.ResourceVersion != rv {
		return apierrors.NewConflict(gvr.GroupResource(), name,
			fmt.Errorf("the ResourceVersion in the precondition (%s) does not match the ResourceVersion in record (%s)",
				*preconditions.ResourceVersion, rv))
	}
	return nil
}

// delete a kubeconfig's configmap and associated tokens.
// kubeconfig must be the already-converted form of configMap (callers convert once).
func (s *Store) delete(
	ctx context.Context,
	kubeconfig *ext.Kubeconfig,
	configMap *corev1.ConfigMap,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	if deleteValidation != nil {
		err := deleteValidation(ctx, kubeconfig)
		if err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				return nil, false, err
			}
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("delete validation for kubeconfig %s failed: %s", configMap.Name, err))
		}
	}

	// Enforce the client's preconditions before any destructive step, so a
	// failing precondition leaves both the tokens and the ConfigMap untouched.
	// The backing ConfigMap delete below re-enforces them authoritatively; the
	// window between the token loop and that delete is the inherent
	// non-transactional gap of this multi-object design. On this gap: if the
	// backing delete returns a Conflict after tokens have been deleted, the
	// caller receives a Conflict that explicitly names this state — signaling
	// that the delete is safe to retry (the tokens are gone; only the ConfigMap
	// record remains to be removed).
	if err := checkPreconditions(options.Preconditions, configMap.Name, configMap.ResourceVersion, kubeconfig.UID, configMap.UID); err != nil {
		return nil, false, err
	}
	if options.Preconditions != nil && options.Preconditions.UID != nil {
		// Rewrite to the ConfigMap UID so the backing delete can enforce
		// it — on a copy, so a caller retrying with the same DeleteOptions
		// still submits its original precondition.
		preconditions := *options.Preconditions
		preconditions.UID = &configMap.UID
		optionsCopy := *options
		optionsCopy.Preconditions = &preconditions
		options = &optionsCopy
	}

	// Delete tokens first so the ConfigMap (which holds the token ID list)
	// survives a partial failure and the whole delete stays retryable.
	tokensDeleteAttempted := len(options.DryRun) == 0 && len(kubeconfig.Status.Tokens) > 0
	for _, tokenName := range kubeconfig.Status.Tokens {
		delOptions := &metav1.DeleteOptions{
			GracePeriodSeconds: options.GracePeriodSeconds,
			PropagationPolicy:  options.PropagationPolicy,
			DryRun:             options.DryRun,
		}
		if err := s.deleteToken(tokenName, delOptions); err != nil {
			return nil, false, mapBackingError(err, configMap.Name)
		}
	}

	err := s.configMapClient.Delete(namespace, configMap.Name, options)
	if err != nil {
		mapped := mapBackingError(err, configMap.Name)
		if apierrors.IsConflict(mapped) && tokensDeleteAttempted {
			return nil, false, apierrors.NewConflict(gvr.GroupResource(), configMap.Name,
				fmt.Errorf("%s; this attempt has already revoked the kubeconfig's tokens and the delete should be retried to remove the record", genericregistry.OptimisticLockErrorMsg))
		}
		return nil, false, mapped
	}
	return kubeconfig, true, nil
}

// Update implements [rest.Updater]
// Note: Create on update is not supported because names are always auto-generated.
func (s *Store) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "update")
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %v", err))
	}

	useCache := false
	oldConfigMap, err := s.getConfigMap(name, &metav1.GetOptions{}, useCache)
	if err != nil {
		return nil, false, err // The err is already an [apierrors.APIStatus].
	}

	if oldConfigMap.Labels[UserIDLabel] != userInfo.GetName() && !isAdmin {
		// An ordinary user can only access their own kubeconfigs.
		// We return a NotFound error to avoid leaking information about other users' kubeconfigs.
		return nil, false, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	oldKubeconfig, err := s.fromConfigMap(oldConfigMap)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", name, err))
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldKubeconfig)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting updated object for kubeconfig %s: %v", name, err))
	}

	newKubeconfig, ok := newObj.(*ext.Kubeconfig)
	if !ok {
		return nil, false, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", newObj))
	}

	// Enforce a UID or ResourceVersion precondition supplied by the caller via
	// objInfo.Preconditions(). This mirrors the Delete path and must run before
	// any write so dry-run also predicts the conflict outcome.
	if prec := objInfo.Preconditions(); prec != nil {
		if err := checkPreconditions(prec, name, oldKubeconfig.ResourceVersion, oldKubeconfig.UID, oldConfigMap.UID); err != nil {
			return nil, false, err
		}
	}

	if updateValidation != nil {
		err = updateValidation(ctx, newKubeconfig, oldKubeconfig)
		if err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				return nil, false, err
			}
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("update validation for kubeconfig %s failed: %s", name, err))
		}
	}

	if !reflect.DeepEqual(oldKubeconfig.Spec.Clusters, newKubeconfig.Spec.Clusters) {
		return nil, false, apierrors.NewBadRequest("spec.clusters is immutable")
	}
	if oldKubeconfig.Spec.CurrentContext != newKubeconfig.Spec.CurrentContext {
		return nil, false, apierrors.NewBadRequest("spec.currentContext is immutable")
	}
	if oldKubeconfig.Spec.TTL != newKubeconfig.Spec.TTL {
		return nil, false, apierrors.NewBadRequest("spec.ttl is immutable")
	}
	if !reflect.DeepEqual(oldKubeconfig.Spec.IncludeDefaultEntry, newKubeconfig.Spec.IncludeDefaultEntry) {
		return nil, false, apierrors.NewBadRequest("spec.includeDefaultEntry is immutable")
	}

	newKubeconfig.UID = oldKubeconfig.UID // Make sure UID is preserved.

	if newKubeconfig.Labels == nil {
		newKubeconfig.Labels = make(map[string]string)
	}
	newKubeconfig.Labels[UserIDLabel] = oldKubeconfig.Labels[UserIDLabel]
	newKubeconfig.Status = oldKubeconfig.Status // Carry over the status.

	meta.SetStatusCondition(&newKubeconfig.Status.Conditions, metav1.Condition{
		Type:   UpdatedCond,
		Status: metav1.ConditionTrue,
		Reason: UpdatedCond,
	})
	// Updated records the time of the latest modification, not a status transition.
	// Advance LastTransitionTime unconditionally so clients can use it as a
	// last-modified timestamp — restoring the observable behavior from before the
	// single-condition upsert was introduced.
	if c := meta.FindStatusCondition(newKubeconfig.Status.Conditions, UpdatedCond); c != nil {
		c.LastTransitionTime = metav1.NewTime(time.Now())
	}

	// Note: [Store.toConfigMap] takes care of enforcing [KindLabel] label and [UIDAnnotation] annotation.
	newConfigMap, err := s.toConfigMap(newKubeconfig)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig %s to configmap: %v", name, err))
	}

	dryRun := options != nil && len(options.DryRun) > 0

	// Enforce the client's resourceVersion precondition even on dry-run, so a
	// preview predicts the real conflict outcome.
	if newKubeconfig.ResourceVersion != "" &&
		newKubeconfig.ResourceVersion != oldKubeconfig.ResourceVersion {
		return nil, false, apierrors.NewConflict(gvr.GroupResource(), name,
			errors.New(genericregistry.OptimisticLockErrorMsg))
	}

	if !dryRun {
		newConfigMap, err = s.configMapClient.Update(newConfigMap)
		if err != nil {
			return nil, false, mapBackingError(err, name)
		}
	}

	newKubeconfig, err = s.fromConfigMap(newConfigMap)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %v", name, err))
	}

	return newKubeconfig, false, nil
}

// GetSingularName implements [rest.SingularNameProvider].
func (s *Store) GetSingularName() string {
	return Singular
}

// GroupVersionKind implements [rest.GroupVersionKindProvider].
func (s *Store) GroupVersionKind(gv schema.GroupVersion) schema.GroupVersionKind {
	return gv.WithKind(Kind)
}

// Destroy implements [rest.Storage].
func (s *Store) Destroy() {}

// NamespaceScoped implements [rest.Scoper].
func (s *Store) NamespaceScoped() bool {
	return false
}

var (
	_ rest.Creater                  = &Store{}
	_ rest.Getter                   = &Store{}
	_ rest.Lister                   = &Store{}
	_ rest.Watcher                  = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.CollectionDeleter        = &Store{}
	_ rest.Updater                  = &Store{}
	_ rest.Patcher                  = &Store{}
	_ rest.TableConvertor           = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)

var (
	pathCMData                  = fieldpath.MakePathOrDie("data")
	pathCMClustersField         = fieldpath.MakePathOrDie("data", "clusters")
	pathCMCurrentContextField   = fieldpath.MakePathOrDie("data", "current-context")
	pathCMDescriptionField      = fieldpath.MakePathOrDie("data", "description")
	pathCMTTLField              = fieldpath.MakePathOrDie("data", "ttl")
	pathCMStatusConditionsField = fieldpath.MakePathOrDie("data", "status-conditions")
	pathCMStatusSummaryField    = fieldpath.MakePathOrDie("data", "status-summary")
	pathCMStatusTokensField     = fieldpath.MakePathOrDie("data", "status-tokens")

	pathCMLabelKind = fieldpath.MakePathOrDie("metadata", "labels", KindLabel)

	pathKConfigClustersField            = fieldpath.MakePathOrDie("spec", "clusters")
	pathKConfigCurrentContextField      = fieldpath.MakePathOrDie("spec", "currentContext")
	pathKConfigDescriptionField         = fieldpath.MakePathOrDie("spec", "description")
	pathKConfigTTLField                 = fieldpath.MakePathOrDie("spec", "ttl")
	pathKConfigIncludeDefaultEntryField = fieldpath.MakePathOrDie("spec", "includeDefaultEntry")

	pathCMIncludeDefaultEntryField = fieldpath.MakePathOrDie("data", "include-default-entry")

	mapFromConfigMap = extcommon.MapSpec{
		pathCMData.String():                     nil,
		pathCMClustersField.String():            pathKConfigClustersField,
		pathCMCurrentContextField.String():      pathKConfigCurrentContextField,
		pathCMDescriptionField.String():         pathKConfigDescriptionField,
		pathCMTTLField.String():                 pathKConfigTTLField,
		pathCMIncludeDefaultEntryField.String(): pathKConfigIncludeDefaultEntryField,
		pathCMStatusConditionsField.String():    nil,
		pathCMStatusSummaryField.String():       nil,
		pathCMStatusTokensField.String():        nil,
		pathCMLabelKind.String():                nil,
	}

	mapFromKubeconfig = extcommon.MapSpec{
		pathKConfigClustersField.String():            pathCMClustersField,
		pathKConfigCurrentContextField.String():      pathCMCurrentContextField,
		pathKConfigDescriptionField.String():         pathCMDescriptionField,
		pathKConfigTTLField.String():                 pathCMTTLField,
		pathKConfigIncludeDefaultEntryField.String(): pathCMIncludeDefaultEntryField,
	}
)
