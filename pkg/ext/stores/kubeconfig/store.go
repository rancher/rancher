package kubeconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	extcommon "github.com/rancher/rancher/pkg/ext/common"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	ctrlv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	kconfig "github.com/rancher/rancher/pkg/kubeconfig"
	v3node "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	extapi "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/features"
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
	ClustersField         = "clusters"
	CurrentContextField   = "current-context"
	DescriptionField      = "description"
	TTLField              = "ttl"
	StatusConditionsField = "status-conditions"
	StatusSummaryField    = "status-summary"
	StatusTokensField     = "status-tokens"
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

// tokenCreator abstracts [tokens.Manager].
type tokenCreator interface {
	EnsureClusterToken(clusterID string, input user.TokenInput) (string, runtime.Object, error)
	EnsureToken(input user.TokenInput) (string, runtime.Object, error)
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
	tokenCache          ctrlv3.TokenCache
	tokens              ctrlv3.TokenClient
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
	store := &Store{
		mcmEnabled:      mcmEnabled,
		configMapCache:  wranglerContext.Core.ConfigMap().Cache(),
		configMapClient: wranglerContext.Core.ConfigMap(),
		clusterCache:    wranglerContext.Mgmt.Cluster().Cache(),
		nsCache:         wranglerContext.Core.Namespace().Cache(),
		nsClient:        wranglerContext.Core.Namespace(),
		tokenCache:      wranglerContext.Mgmt.Token().Cache(),
		tokens:          wranglerContext.Mgmt.Token(),
		userCache:       wranglerContext.Mgmt.User().Cache(),
		tokenMgr:        tokens.NewManager(wranglerContext),
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

	defaultTTL, err := s.getDefaultTTL()
	if err != nil {
		return nil, fmt.Errorf("error getting default token TTL: %w", err)
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting local cluster: %w", err))
	}

	if len(kubeconfig.Spec.Clusters) > 0 {
		if isAllClusters = kubeconfig.Spec.Clusters[0] == "*"; isAllClusters {
			// The first id in the spec.clusters "*" means all clusters.
			clusters, err = s.clusterCache.List(labels.Everything())
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error listing clusters: %w", err))
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
					return nil, apierrors.NewInternalError(fmt.Errorf("error getting cluster %s: %w", clusterID, err))
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
			return nil, apierrors.NewInternalError(fmt.Errorf("error checking if user %s has access to cluster %s: %w", userInfo.GetName(), clusters[i].Name, err))
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig to configmap: %w", err))
	}
	configMap.GenerateName = namePrefix

	var kubeConfigID string
	if dryRun {
		kubeConfigID = names.SimpleNameGenerator.GenerateName(namePrefix)
		configMap.Name = kubeConfigID
	} else {
		if err = s.ensureNamespace(); err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error ensuring namespace %s: %w", namespace, err))
		}

		configMap, err = s.configMapClient.Create(configMap)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error creating configmap for kubeconfig: %w", err))
		}
		kubeConfigID = configMap.Name
	}

	kubeconfigToStore, err = s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", kubeConfigID, err))
	}

	var (
		sharedTokenKey string
		sharedToken    runtime.Object
		ownerRefs      []metav1.OwnerReference
		v1Config       string
	)

	ownerReferenceFrom := func(obj runtime.Object) (metav1.OwnerReference, error) {
		objMeta, err := meta.Accessor(obj)
		if err != nil {
			return metav1.OwnerReference{}, err
		}

		ref := metav1.OwnerReference{
			APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			Name:       objMeta.GetName(),
			UID:        objMeta.GetUID(),
		}
		return ref, nil
	}

	caCert := kconfig.FormatCertString(base64.StdEncoding.EncodeToString([]byte(s.getCACert())))
	data := kconfig.KubeConfig{
		Meta: kconfig.Meta{
			Name:              kubeConfigID,
			CreationTimestamp: configMap.CreationTimestamp.Format(time.RFC3339),
			TTL:               strconv.FormatInt(kubeconfig.Spec.TTL, 10),
		},
		CurrentContext: defaultClusterName,
	}

	err = func() error { // Deliberately use an anonymous function to capture the status and error conditions.
		// Generate a shared token for the default and non-ACE clusters.
		if !dryRun && generateToken {
			input := s.createTokenInput(kubeConfigID, userInfo.GetName(), authToken, &ttlMilliseconds)
			sharedTokenKey, sharedToken, err = s.tokenMgr.EnsureToken(input)
			if err != nil {
				conditions = append(conditions, metav1.Condition{
					Type:               FailedToCreateTokenCond,
					Status:             metav1.ConditionTrue,
					Reason:             FailedToCreateTokenCond,
					Message:            fmt.Sprintf("error creating kubeconfig token: %s", err),
					LastTransitionTime: metav1.NewTime(time.Now()),
				})

				return apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token: %w", err))
			}

			ownerRef, err := ownerReferenceFrom(sharedToken)
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("error getting owner reference for shared token: %w", err))
			}
			ownerRefs = append(ownerRefs, ownerRef)
			tokenIDs = append(tokenIDs, ownerRef.Name)

			conditions = append(conditions, metav1.Condition{
				Type:               TokenCreatedCond,
				Status:             metav1.ConditionTrue,
				Reason:             TokenCreatedCond,
				Message:            ownerRef.Name,
				LastTransitionTime: metav1.NewTime(time.Now()),
			})
		}

		// The default entry that points to the Rancher URL.
		// Even a base user without access to any cluster should be able to use a kubeconfig
		// to interact with Rancher via Public API.
		data.Clusters = append(data.Clusters, kconfig.Cluster{
			Name:   defaultClusterName,
			Server: "https://" + host,
			Cert:   caCert,
		})
		data.Users = append(data.Users, kconfig.User{
			Name:  defaultClusterName,
			Token: sharedTokenKey,
		})
		data.Contexts = append(data.Contexts, kconfig.Context{
			Name:    defaultClusterName,
			Cluster: defaultClusterName,
			User:    defaultClusterName,
		})

		for _, cluster := range clusters {
			var (
				tokenKey string
				token    runtime.Object
			)

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
				input := s.createTokenInput(kubeConfigID, userInfo.GetName(), authToken, &ttlMilliseconds)
				tokenKey, token, err = s.tokenMgr.EnsureClusterToken(cluster.Name, input)
				if err != nil {
					conditions = append(conditions, metav1.Condition{
						Type:               FailedToCreateTokenCond,
						Status:             metav1.ConditionTrue,
						Reason:             FailedToCreateTokenCond,
						Message:            fmt.Sprintf("error creating kubeconfig token for cluster %s: %s", cluster.Name, err),
						LastTransitionTime: metav1.NewTime(time.Now()),
					})

					return apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token for cluster %s: %w", cluster.Name, err))
				}

				ownerRef, err := ownerReferenceFrom(token)
				if err != nil {
					return apierrors.NewInternalError(fmt.Errorf("error getting owner reference for token: %w", err))
				}
				ownerRefs = append(ownerRefs, ownerRef)
				tokenIDs = append(tokenIDs, ownerRef.Name)

				conditions = append(conditions, metav1.Condition{
					Type:               TokenCreatedCond,
					Status:             metav1.ConditionTrue,
					Reason:             TokenCreatedCond,
					Message:            ownerRef.Name,
					LastTransitionTime: metav1.NewTime(time.Now()),
				})
			}

			data.Contexts = append(data.Contexts, kconfig.Context{
				Name:    clusterName,
				Cluster: clusterName,
				User:    clusterName,
			})
			data.Users = append(data.Users, kconfig.User{
				Name:  clusterName,
				Token: tokenKey,
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

					return apierrors.NewInternalError(fmt.Errorf("error listing nodes for cluster %s: %w", cluster.Name, err))
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

					if !isCurrentContextSet && currentContext == cluster.Name {
						data.CurrentContext = nodeName // Set the current context to the first control plane node.
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

			return apierrors.NewInternalError(fmt.Errorf("error generating kubeconfig content: %w", err))
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

	var convertErr error
	configMap, convertErr = s.toConfigMap(kubeconfigToStore)
	if convertErr == nil {
		if !dryRun {
			var updateErr error
			configMap, updateErr = s.configMapClient.Update(configMap)
			if updateErr != nil {
				if err == nil {
					err = apierrors.NewInternalError(fmt.Errorf("error updating configmap for kubeconfig %s: %w", kubeConfigID, updateErr))
				} // else preserve the original error.
			}
		}
	} else {
		if err == nil {
			err = apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig %s to configmap: %w", kubeConfigID, convertErr))
		} // else preserve the original error.
	}

	if err != nil {
		return nil, err
	}

	kubeconfig, err = s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig after saving: %w", kubeConfigID, err))
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

// first returns the first element of a slice of strings, or an empty string if the slice is empty.
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

// tokenSelector builds a label selector for kubeconfig tokens.
func tokenSelector(isAdmin bool, userID, kubeconfigID string) (labels.Selector, error) {
	set := labels.Set{
		tokens.TokenKindLabel: KindLabelValue,
	}

	if kubeconfigID != "" {
		set = labels.Merge(set, labels.Set{tokens.TokenKubeconfigIDLabel: kubeconfigID})
	}

	if !isAdmin {
		set = labels.Merge(set, labels.Set{tokens.UserIDLabel: userID})
	}

	return set.AsSelector(), nil
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", name, err))
	}

	return kubeconfig, nil
}

// List implements [rest.Lister].
func (s *Store) NewList() runtime.Object {
	return &ext.KubeconfigList{}
}

func toListOptions(options *metainternalversion.ListOptions, userInfo k8suser.Info, isAdmin bool) (*metav1.ListOptions, error) {
	listOptions, err := extapi.ConvertListOptions(options)
	if err != nil {
		return nil, fmt.Errorf("error converting list options: %w", err)
	}

	labelSet, err := labels.ConvertSelectorToLabelsMap(listOptions.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}

	configMapLabels := labels.Set{
		KindLabel: KindLabelValue,
	}

	if !isAdmin {
		configMapLabels[UserIDLabel] = userInfo.GetName()
	}

	labelSet = labels.Merge(labelSet, configMapLabels)
	listOptions.LabelSelector = labelSet.AsSelector().String()

	return listOptions, nil
}

// List implements [rest.Lister].
func (s *Store) List(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, _, err := s.userFrom(ctx, "list")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	configMapList, err := s.configMapClient.List(namespace, *listOptions)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) { // Continue token expired.
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("error listing configmaps for kubeconfigs: %w", err))
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
			return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", configMap.Name, err))
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if !features.FeatureGates().Enabled(features.WatchListClient) {
		listOptions.SendInitialEvents = nil
		listOptions.ResourceVersionMatch = ""
	}

	configMapWatch, err := s.configMapClient.Watch(namespace, *listOptions)
	if err != nil {
		logrus.Errorf("kubeconfig: watch: error starting watch: %s", err)
		return nil, apierrors.NewInternalError(fmt.Errorf("kubeconfig: watch: error starting watch: %w", err))
	}

	kubeconfigWatch := &watcher{
		ch: make(chan watch.Event, 100),
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

					obj = &ext.Kubeconfig{
						ObjectMeta: metav1.ObjectMeta{
							ResourceVersion: configMap.ResourceVersion,
						},
					}
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
				default: // watch.Error
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
	mu     sync.RWMutex
	closed bool
	ch     chan watch.Event
}

// Stop tells the producer that the consumer is done watching, so the
// producer should stop sending events and close the result channel.
func (w *watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// no operation if called multiple times.
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

	w.ch <- event
	return true
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
		if ref.Kind == "Token" && ref.APIVersion == "management.cattle.io/v3" {
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
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	lOptions, err := toListOptions(listOptions, userInfo, isAdmin)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	configMapList, err := s.configMapClient.List(namespace, *lOptions)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) { // Continue token expired.
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("error listing configmaps for kubeconfigs: %w", err))
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
		tokenSelector, err := tokenSelector(isAdmin, userInfo.GetName(), configMap.Name)
		if err != nil {
			return nil, apierrors.NewInternalError(err)
		}

		obj, _, err := s.delete(ctx, &configMap, tokenSelector, deleteValidation, options)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error deleting kubeconfig %s: %w", configMap.Name, err))
		}

		kubeconfig, ok := obj.(*ext.Kubeconfig)
		if !ok { // Sanity check.
			return nil, apierrors.NewInternalError(fmt.Errorf("expected kubeconfig object, got %T", obj))
		}

		list.Items = append(list.Items, *kubeconfig)
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
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
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

	tokenSelector, err := tokenSelector(isAdmin, userInfo.GetName(), name)
	if err != nil {
		return nil, false, apierrors.NewInternalError(err)
	}

	return s.delete(ctx, configMap, tokenSelector, deleteValidation, options)
}

// delete a kubeconfig's configmap and associated tokens.
func (s *Store) delete(
	ctx context.Context,
	configMap *corev1.ConfigMap,
	tokenSelector labels.Selector,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	kubeconfig, err := s.fromConfigMap(configMap)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", configMap.Name, err))
	}

	if deleteValidation != nil {
		err := deleteValidation(ctx, kubeconfig)
		if err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				return nil, false, err
			}
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("delete validation for kubeconfig %s failed: %s", configMap.Name, err))
		}
	}

	tokenList, err := s.tokenCache.List(tokenSelector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, apierrors.NewNotFound(gvr.GroupResource(), configMap.Name)
		}
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error listing tokens for kubeconfig %s: %w", configMap.Name, err))
	}

	if options.Preconditions != nil {
		// If the UID precondition matches the kubeconfig's UID, we need
		// to replace it with the corresponding configmap's UID.
		if options.Preconditions.UID != nil && *options.Preconditions.UID == kubeconfig.UID {
			options.Preconditions.UID = &configMap.UID
		}
	}

	err = s.configMapClient.Delete(namespace, configMap.Name, options)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return nil, false, apierrors.NewNotFound(gvr.GroupResource(), configMap.Name)
	case apierrors.IsConflict(err):
		// Massage the err details to refer to Kubeconfigs instead of ConfigMaps.
		var errMessage string
		conflictErr, ok := err.(*apierrors.StatusError)
		if ok {
			_, errMessage, ok = strings.Cut(conflictErr.ErrStatus.Message, `ConfigMap "`+configMap.Name+`": `)
			if !ok {
				errMessage = ""
			}
		}
		return nil, false, apierrors.NewConflict(gvr.GroupResource(), configMap.Name, errors.New(errMessage))
	default:
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error deleting configmap for kubeconfig %s: %w", configMap.Name, err))
	}

	var tokenNames []string
	for _, token := range tokenList {
		tokenNames = append(tokenNames, token.Name)
	}

	for _, tokenName := range tokenNames {
		delOptions := &metav1.DeleteOptions{
			GracePeriodSeconds: options.GracePeriodSeconds,
			PropagationPolicy:  options.PropagationPolicy,
			DryRun:             options.DryRun, // Pass through the dry run flag instead of handling it explicitly.
		}

		err := s.tokens.Delete(tokenName, delOptions)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, false, apierrors.NewInternalError(fmt.Errorf("error deleting token %s for kubeconfig %s: %w", tokenName, configMap.Name, err))
		}
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
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
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
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", name, err))
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldKubeconfig)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting updated object for kubeconfig %s: %w", name, err))
	}

	newKubeconfig, ok := newObj.(*ext.Kubeconfig)
	if !ok {
		return nil, false, apierrors.NewBadRequest(fmt.Sprintf("invalid object type %T", newObj))
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

	newKubeconfig.UID = oldKubeconfig.UID // Make sure UID is preserved.

	if newKubeconfig.Labels == nil {
		newKubeconfig.Labels = make(map[string]string)
	}
	newKubeconfig.Labels[UserIDLabel] = oldKubeconfig.Labels[UserIDLabel]
	newKubeconfig.Status = oldKubeconfig.Status // Carry over the status.

	newKubeconfig.Status.Conditions = append(newKubeconfig.Status.Conditions, metav1.Condition{
		Type:               UpdatedCond,
		Status:             metav1.ConditionTrue,
		Reason:             UpdatedCond,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

	// Note: [Store.toConfigMap] takes care of enforcing [KindLabel] label and [UIDAnnotation] annotation.
	newConfigMap, err := s.toConfigMap(newKubeconfig)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig %s to configmap: %w", name, err))
	}

	dryRun := options != nil && len(options.DryRun) > 0
	if !dryRun {
		newConfigMap, err = s.configMapClient.Update(newConfigMap)
		if err != nil {
			return nil, false, apierrors.NewInternalError(fmt.Errorf("error updating configmap for kubeconfig %s: %w", name, err))
		}
	}

	newKubeconfig, err = s.fromConfigMap(newConfigMap)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig: %w", name, err))
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

	pathKConfigClustersField       = fieldpath.MakePathOrDie("spec", "clusters")
	pathKConfigCurrentContextField = fieldpath.MakePathOrDie("spec", "currentContext")
	pathKConfigDescriptionField    = fieldpath.MakePathOrDie("spec", "description")
	pathKConfigTTLField            = fieldpath.MakePathOrDie("spec", "ttl")

	mapFromConfigMap = extcommon.MapSpec{
		pathCMData.String():                  nil,
		pathCMClustersField.String():         pathKConfigClustersField,
		pathCMCurrentContextField.String():   pathKConfigCurrentContextField,
		pathCMDescriptionField.String():      pathKConfigDescriptionField,
		pathCMTTLField.String():              pathKConfigTTLField,
		pathCMStatusConditionsField.String(): nil,
		pathCMStatusSummaryField.String():    nil,
		pathCMStatusTokensField.String():     nil,
		pathCMLabelKind.String():             nil,
	}

	mapFromKubeconfig = extcommon.MapSpec{
		pathKConfigClustersField.String():       pathCMClustersField,
		pathKConfigCurrentContextField.String(): pathCMCurrentContextField,
		pathKConfigDescriptionField.String():    pathCMDescriptionField,
		pathKConfigTTLField.String():            pathCMTTLField,
	}
)
