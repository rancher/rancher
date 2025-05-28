package kubeconfig

import (
	"context"
	"encoding/json"
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
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
)

const (
	Kind               = "Kubeconfig"
	Singular           = "kubeconfig"
	Namespace          = exttokens.TokenNamespace
	UserIDLabel        = "cattle.io/userId"
	KindLabel          = "cattle.io/kind"
	KindLabelValue     = "kubeconfig"
	UIDAnnotation      = "cattle.io/uid"
	unknownValue       = "<unknown>"
	defaultClusterName = "rancher"
)

// Names of the ConfigMap fields that to persist the Kubeconfig data.
const (
	ClustersField         = "clusters"
	CurrentContextField   = "current-context"
	DescriptionField      = "description"
	TTLField              = "ttl"
	StatusConditionsField = "status-conditions"
	StatusSummaryField    = "status-summary"
	StatusTokensField     = "status-tokens"
)

const (
	StatusSummaryComplete = "Complete"
	StatusSummaryError    = "Error"
)

const (
	CreatedCond          = "Created"
	UpdatedCond          = "Updated"
	TokenCreatedCond     = "TokenCreated"
	ContentGeneratedCond = "ContentGenerated"
	FailedToGenerateCond = "FailedToGenerate"
)

var gvr = ext.SchemeGroupVersion.WithResource(ext.KubeconfigResourceName)

type userManager interface {
	EnsureClusterToken(clusterID string, input user.TokenInput) (string, runtime.Object, error)
	EnsureToken(input user.TokenInput) (string, runtime.Object, error)
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
// Store implements storage for [ext.Kubeconfig].
type Store struct {
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
	userMgr             userManager
	getDefaultTTL       func() (*int64, error)
	getServerURL        func() string
	shouldGenerateToken func() bool
	tableConverter      rest.TableConvertor
}

// New creates a new instance of [Store].
func New(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer, userMgr user.Manager) *Store {
	return &Store{
		configMapCache:  wranglerContext.Core.ConfigMap().Cache(),
		configMapClient: wranglerContext.Core.ConfigMap(),
		clusterCache:    wranglerContext.Mgmt.Cluster().Cache(),
		nsCache:         wranglerContext.Core.Namespace().Cache(),
		nsClient:        wranglerContext.Core.Namespace(),
		nodeCache:       wranglerContext.Mgmt.Node().Cache(),
		tokenCache:      wranglerContext.Mgmt.Token().Cache(),
		tokens:          wranglerContext.Mgmt.Token(),
		userCache:       wranglerContext.Mgmt.User().Cache(),
		userMgr:         userMgr,
		authorizer:      authorizer,
		getServerURL:    settings.ServerURL.Get,
		getDefaultTTL:   tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds,
		shouldGenerateToken: func() bool {
			return strings.EqualFold(settings.KubeconfigGenerateToken.Get(), "true")
		},
		tableConverter: printerstorage.TableConvertor{
			TableGenerator: printers.NewTableGenerator().With(printHandler),
		},
	}
}

// EnsureNamespace ensures that the namespace for storing kubeconfig configMaps exists.
func (s *Store) EnsureNamespace() error {
	_, err := s.nsCache.Get(Namespace)
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting namespace %s: %w", Namespace, err)
	}

	_, err = s.nsClient.Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: Namespace,
		},
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating namespace %s: %w", Namespace, err)
	}

	return nil
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

// Create implements [rest.Creater]
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
		return nil, apierrors.NewBadRequest(fmt.Sprintf("spec.ttl %d exceeds max tll %d", kubeconfig.Spec.TTL, defaultTTLSeconds))
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
			return nil, apierrors.NewBadRequest("can't determine the server URL")
		}
	}

	var (
		conditions []metav1.Condition
		tokenIDs   []string
	)

	var kubeConfigID string
	err = retry.OnError(retry.DefaultRetry, func(_ error) bool {
		return true // Retry all errors.
	}, func() error {
		kubeConfigID = names.SimpleNameGenerator.GenerateName("kubeconfig-")
		_, err := s.configMapCache.Get(Namespace, kubeConfigID)
		if err == nil {
			return fmt.Errorf("kubeconfig %s already exists", kubeConfigID)
		}

		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("error getting kubeconfig %s: %w", kubeConfigID, err)
	})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error checking if kubeconfig %s exists: %w", kubeConfigID, err))
	}

	localCluster, err := s.clusterCache.Get("local")
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting local cluster: %w", err))
	}

	var clusters []*apiv3.Cluster
	if len(kubeconfig.Spec.Clusters) > 0 {
		if kubeconfig.Spec.Clusters[0] == "*" {
			// The first id in the spec.clusters "*" means all clusters.
			clusters, err = s.clusterCache.List(labels.Everything())
			if err != nil {
				return nil, fmt.Errorf("error listing clusters: %w", err)
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

	var foundCurrentContext bool
	if kubeconfig.Spec.CurrentContext == "" {
		kubeconfig.Spec.CurrentContext = "rancher"
		foundCurrentContext = true
	}

	// Check if the user has access to requested clusters before generating tokens.
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

		if !foundCurrentContext && kubeconfig.Spec.CurrentContext == cluster.Name {
			foundCurrentContext = true
		}
	}

	if !foundCurrentContext {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid currentContext %s", kubeconfig.Spec.CurrentContext))
	}

	dryRun := options != nil && len(options.DryRun) > 0
	generateToken := s.shouldGenerateToken()

	var (
		sharedTokenKey string
		sharedToken    runtime.Object
		ownerRefs      []metav1.OwnerReference
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

	// Generate a shared token for the default and non-ACE clusters.
	if !dryRun && generateToken {
		input := s.createTokenInput(kubeConfigID, userInfo.GetName(), authToken, &ttlMilliseconds)
		sharedTokenKey, sharedToken, err = s.userMgr.EnsureToken(input)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token: %w", err))
		}

		ownerRef, err := ownerReferenceFrom(sharedToken)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error getting owner reference for shared token: %w", err))
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

	caCert := kconfig.FormatCert(settings.CACerts.Get())
	data := kconfig.KubeConfig{
		Meta: kconfig.Meta{
			Name:              kubeConfigID,
			CreationTimestamp: time.Now().Format(time.RFC3339), // TODO: Use the timestamp from the backing ConfigMap.
			TTL:               strconv.FormatInt(kubeconfig.Spec.TTL, 10),
		},
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

		if !cluster.Spec.LocalClusterAuthEndpoint.Enabled {
			data.Contexts = append(data.Contexts, kconfig.Context{
				Name:    clusterName,
				Cluster: clusterName,
				User:    defaultClusterName, // Reuse the auth info with the shared token.
			})

			continue
		}

		// For the ACE cluster we generate a cluster-scoped token.
		if !dryRun && generateToken {
			input := s.createTokenInput(kubeConfigID, userInfo.GetName(), authToken, &ttlMilliseconds)
			tokenKey, token, err = s.userMgr.EnsureClusterToken(cluster.Name, input)
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error creating kubeconfig token for cluster %s: %w", cluster.Name, err))
			}

			ownerRef, err := ownerReferenceFrom(token)
			if err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error getting owner reference for token: %w", err))
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

		// If the ACE cluster has a FQDN, add a single entry for it.
		if authEndpoint := cluster.Spec.LocalClusterAuthEndpoint; authEndpoint.FQDN != "" {
			fqdnName := clusterName + "-fqdn"
			data.Clusters = append(data.Clusters, kconfig.Cluster{
				Name:   fqdnName,
				Server: "https://" + authEndpoint.FQDN,
				Cert:   kconfig.FormatCert(authEndpoint.CACerts),
			})
			data.Contexts = append(data.Contexts, kconfig.Context{
				Name:    fqdnName,
				Cluster: fqdnName,
				User:    clusterName,
			})

			continue
		}

		// Otherwise produce entries for each control plane node.
		nodes, err := s.nodeCache.List(cluster.Name, labels.Everything())
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error listing nodes for cluster %s: %w", cluster.Name, err))
		}

		clusterCerts := kconfig.FormatCert(cluster.Status.CACert)
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
		}
	}

	v1Config, err := kconfig.Generate(data)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error generating kubeconfig content: %w", err))
	}

	conditions = append(conditions, metav1.Condition{
		Type:               ContentGeneratedCond,
		Status:             metav1.ConditionTrue,
		Reason:             ContentGeneratedCond,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

	kubeconfigToStore := kubeconfig.DeepCopy()
	kubeconfigToStore.Name = kubeConfigID // Overwrite the name.

	if kubeconfigToStore.Labels == nil {
		kubeconfigToStore.Labels = make(map[string]string)
	}
	kubeconfigToStore.Labels[UserIDLabel] = userInfo.GetName()
	kubeconfigToStore.OwnerReferences = append(kubeconfigToStore.OwnerReferences, ownerRefs...)
	kubeconfigToStore.Annotations = map[string]string{
		UIDAnnotation: string(uuid.NewUUID()),
	}

	kubeconfigToStore.Status = &ext.KubeconfigStatus{
		Summary:    StatusSummaryComplete,
		Conditions: conditions,
		Tokens:     tokenIDs,
	}

	configMap, err := s.toConfigMap(kubeconfigToStore)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting kubeconfig to configmap: %w", err))
	}
	if !dryRun {
		_, err = s.configMapClient.Create(configMap)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error creating configmap for kubeconfig %s: %w", kubeConfigID, err))
		}
	}

	kubeconfig, err = s.fromConfigMap(configMap)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error converting configmap %s to kubeconfig after saving: %w", kubeConfigID, err))
	}

	kubeconfig.Status.Value = v1Config

	return kubeconfig, nil
}

// toConfigMap converts a Kubeconfig object to a ConfigMap object.
func (s *Store) toConfigMap(kubeconfig *ext.Kubeconfig) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfig.Name,
			Namespace: Namespace,
		},
		Data: make(map[string]string),
	}

	if len(kubeconfig.Annotations) > 0 {
		configMap.Annotations = make(map[string]string)
		for k, v := range kubeconfig.Annotations {
			configMap.Annotations[k] = v
		}
	}

	configMap.Labels = make(map[string]string)
	for k, v := range kubeconfig.Labels {
		configMap.Labels[k] = v
	}
	configMap.Labels[KindLabel] = KindLabelValue

	configMap.Finalizers = append(configMap.Finalizers, kubeconfig.Finalizers...)
	configMap.OwnerReferences = append(configMap.OwnerReferences, kubeconfig.OwnerReferences...)

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

	if kubeconfig.Status != nil { // Note: Value should never be persisted!
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

	if seriliazed := configMap.Data[ClustersField]; seriliazed != "" {
		err = json.Unmarshal([]byte(configMap.Data[ClustersField]), &kubeconfig.Spec.Clusters)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling spec.clusters for %s: %w", configMap.Name, err)
		}
	}

	status := &ext.KubeconfigStatus{
		Summary: configMap.Data[StatusSummaryField],
	}

	if serialized := configMap.Data[StatusConditionsField]; serialized != "" {
		err = json.Unmarshal([]byte(serialized), &status.Conditions)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling status.conditions for %s: %w", configMap.Name, err)
		}
	}

	if serialized := configMap.Data[StatusTokensField]; serialized != "" {
		err = json.Unmarshal([]byte(serialized), &status.Tokens)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling status.tokens for %s: %w", configMap.Name, err)
		}
	}

	if status.Summary != "" || len(status.Conditions) > 0 || len(status.Tokens) > 0 {
		kubeconfig.Status = status
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

func (s *Store) getConfigMap(name string, options *metav1.GetOptions, useCache bool) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
	)

	if useCache {
		configMap, err = s.configMapCache.Get(Namespace, name)
	} else {
		configMap, err = s.configMapClient.Get(Namespace, name, *options)
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

// Get implements [rest.Getter]
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

// List implements [rest.Lister]
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

// List implements [rest.Lister]
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

	configMapList, err := s.configMapClient.List(Namespace, *listOptions)
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

// Watch implements [rest.Watcher], the interface to support the `watch` verb.
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

	kubeconfigWatch := &watcher{
		ch: make(chan watch.Event, 100),
	}

	go func() {
		configMapWatch, err := s.configMapClient.Watch(Namespace, *listOptions)
		if err != nil {
			logrus.Errorf("kubeconfig: watch: error starting watch: %s", err)
			return
		}
		defer configMapWatch.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, more := <-configMapWatch.ResultChan():
				if !more {
					return
				}

				var kubeconfig *ext.Kubeconfig
				switch event.Type {
				case watch.Bookmark:
					configMap, ok := event.Object.(*corev1.ConfigMap)
					if !ok {
						logrus.Warnf("kubeconfig: watch: expected configmap got %T", event.Object)
						continue
					}

					kubeconfig = &ext.Kubeconfig{
						ObjectMeta: metav1.ObjectMeta{
							ResourceVersion: configMap.ResourceVersion,
						},
					}
				case watch.Error:
					status, ok := event.Object.(*metav1.Status)
					if ok {
						logrus.Warnf("kubeconfig: watch: received error event: %s", status.String())
					} else {
						logrus.Warnf("kubeconfig: watch: received error event: %s", event.Object.GetObjectKind().GroupVersionKind().String())
					}
					continue
				case watch.Added, watch.Modified, watch.Deleted:
					configMap, ok := event.Object.(*corev1.ConfigMap)
					if !ok {
						logrus.Warnf("kubeconfig: watch: expected configmap got %T", event.Object)
						continue
					}

					kubeconfig, err = s.fromConfigMap(configMap)
					if err != nil {
						logrus.Errorf("kubeconfig: watch: error converting configmap %s to kubeconfig: %s", configMap.Name, err)
						continue
					}
				default:
					logrus.Warnf("kubeconfig: watch: unknown event type %s", event.Type)
				}

				if !kubeconfigWatch.add(watch.Event{
					Type:   event.Type,
					Object: kubeconfig,
				}) {
					return
				}
			}
		}
	}()

	return kubeconfigWatch, nil
}

// watcher implements [watch.Interface]
type watcher struct {
	mu     sync.RWMutex
	closed bool
	ch     chan watch.Event
}

// Stop implements [watch.Interface]
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

// ResultChan implements [watch.Interface]
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

// ConvertToTable implements [rest.TableConvertor]
func (s *Store) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return s.tableConverter.ConvertToTable(ctx, object, tableOptions)
}

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

func printKubeconfig(kubeconfig *ext.Kubeconfig, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	status := unknownValue
	allTokenCount := 0
	if kubeconfig.Status != nil {
		if kubeconfig.Status.Summary != "" {
			status = kubeconfig.Status.Summary
		}

		allTokenCount = len(kubeconfig.Status.Tokens)
	}

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

	configMapList, err := s.configMapClient.List(Namespace, *lOptions)
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

// Delete implements [rest.GracefulDeleter]
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
			return nil, false, err
		}
	}

	tokenList, err := s.tokenCache.List(tokenSelector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, apierrors.NewNotFound(gvr.GroupResource(), configMap.Name)
		}
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error listing tokens for kubeconfig %s: %w", configMap.Name, err))
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

	err = s.configMapClient.Delete(Namespace, configMap.Name, options) // TODO: Revisit the options. It's not clear if all options we'll play nicely.
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error deleting configmap for kubeconfig %s: %w", configMap.Name, err))
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

	if newKubeconfig.Annotations == nil {
		newKubeconfig.Annotations = make(map[string]string)
	}
	newKubeconfig.Annotations[UIDAnnotation] = string(oldKubeconfig.UID) // Preserve the UID.

	if newKubeconfig.Labels == nil {
		newKubeconfig.Labels = make(map[string]string)
	}
	newKubeconfig.Labels[UserIDLabel] = oldKubeconfig.Labels[UserIDLabel]
	newKubeconfig.Labels[KindLabel] = KindLabelValue
	newKubeconfig.Status = oldKubeconfig.Status // Carry over the status.

	if newKubeconfig.Status == nil {
		newKubeconfig.Status = &ext.KubeconfigStatus{}
	}
	newKubeconfig.Status.Conditions = append(newKubeconfig.Status.Conditions, metav1.Condition{
		Type:               UpdatedCond,
		Status:             metav1.ConditionTrue,
		Reason:             UpdatedCond,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

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
