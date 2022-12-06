package projectresources

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/steve/pkg/attributes"
	steveschema "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	apiregcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apiregistration.k8s.io/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
	clientauthzv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	// Group is the APIGroup for this APIService.
	Group = "resources.project.cattle.io"
	// ParentLabel is the label assigned to namespaces that represent Projects.
	ParentLabel = "cattle.io/parent"
	// RoleName is the name of the ClusterRole and RoleBindings used to grant access to this API.
	RoleName = "cattle-project-resources"
	// AuthzAnnotation is the annotation designated for RBAC resources.
	AuthzAnnotation = "resources.project.cattle.io/authz"
	// NamespaceAnnotation is the annotation designated for namespaces.
	NamespaceAnnotation = "resources.project.cattle.io/namespace"

	apiServiceName            = version + "." + Group
	groupVersion              = Group + "/" + version
	version                   = "v1alpha1"
	queryKey                  = "fieldSelector"
	projectsOrNamespacesKey   = "projectsornamespaces"
	projectsOrNamespacesVar   = "projectsOrNamespaces"
	cattleClusterAgentService = "cattle-cluster-agent"
	rancherService            = "rancher"
	priority                  = 1 // low priority
	unscopedNamespace         = "cattle-unscoped"
)

var (
	paramScheme                = runtime.NewScheme()
	paramCodec                 = runtime.NewParameterCodec(paramScheme)
	queryOpReg                 = regexp.MustCompile("[=!]?=")
	queryValue                 = fmt.Sprintf("%s{op:[=!]?}={%s}", projectsOrNamespacesKey, projectsOrNamespacesVar)
	orphanNamespaceRequirement *labels.Requirement
	groupResourceReg           = regexp.MustCompile(`^.*\.([a-z]*)$`)
)

func init() {
	metav1.AddToGroupVersion(paramScheme, metav1.SchemeGroupVersion)

	var err error
	orphanNamespaceRequirement, err = labels.NewRequirement(nslabels.ProjectIDFieldLabel, selection.DoesNotExist, nil)
	if err != nil {
		panic("programmer error: invalid selector")
	}
}

type configError struct {
	msg string
}

func (c configError) Error() string {
	if c.msg == "" {
		return "invalid extension config"
	}
	return c.msg
}

type authError struct {
	msg string
}

func (c authError) Error() string {
	return fmt.Sprintf(c.msg)
}

type apiServiceHandler struct {
	secretCache       corecontrollers.SecretCache
	apiServiceCache   apiregcontrollers.APIServiceCache
	apiServiceClient  apiregcontrollers.APIServiceClient
	namespaceCache    corecontrollers.NamespaceCache
	namespaceClient   corecontrollers.NamespaceClient
	clusterRoleCache  rbaccontrollers.ClusterRoleCache
	clusterRoleClient rbaccontrollers.ClusterRoleClient
	roleBindingCache  rbaccontrollers.RoleBindingCache
	roleBindingClient rbaccontrollers.RoleBindingClient
}

type handler struct {
	restConfig     *rest.Config
	sarClient      clientauthzv1.SubjectAccessReviewInterface
	namespaceCache corecontrollers.NamespaceCache
	apis           APIResourceWatcher
	clientGetter   clientGetter
}

type authHandler struct {
	configMapCache corecontrollers.ConfigMapCache
}

type clientGetter func(*http.Request, *rest.Config, schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error)

// formatter ensures the links in the steve response point back to the original resource,
// since update, view, and delete don't work on this endpoint.
func formatter(apiOp *types.APIRequest, resource *types.RawResource) {
	if !strings.Contains(resource.Type, Group) {
		return
	}
	unst, ok := resource.APIObject.Object.(*unstructured.Unstructured)
	if !ok {
		logrus.Debugf("could not format links for unexpected resource type %v", resource)
		return
	}
	apiVersion, ok := unst.Object["apiVersion"]
	if !ok {
		logrus.Debugf("could not format links for unexpected resource type %v", resource)
		return
	}

	for k, v := range resource.Links {
		v1Pattern := fmt.Sprintf("/v1/%s.", Group)           // self, update, remove links (added by apiserver)
		k8sPattern := fmt.Sprintf("/apis/%s/", groupVersion) // view link (added by steve)
		switch {
		case strings.Contains(v, v1Pattern):
			resource.Links[k] = strings.Replace(v, v1Pattern, "/v1/", 1)
		case strings.Contains(v, k8sPattern):
			// example input: https://rancher/apis/resources.project.cattle.io/v1alpha1/namespaces/default/apps.deployments/mydeployment
			// use the true API version, not what the schema thinks it is
			var link string
			if apiVersion == "v1" {
				link = strings.Replace(v, k8sPattern, fmt.Sprintf("/api/%s/", apiVersion), 1)
			} else {
				link = strings.Replace(v, k8sPattern, fmt.Sprintf("/apis/%s/", apiVersion), 1)
			}
			concatResource := attributes.Resource(resource.Schema)                    // example: apps.deployments, rbac.authorization.io.roles, pods
			resourceOnly := groupResourceReg.ReplaceAllString(concatResource, "${1}") // result: deployments, roles, pods
			link = strings.Replace(link, concatResource, resourceOnly, 1)
			resource.Links[k] = link // example result: https://rancher/apis/apps/v1/namespaces/default/deployments/mydeployment
		}
	}
	selfLink, ok := resource.Links["self"]
	if !ok {
		return
	}
	origType := strings.ReplaceAll(resource.Type, Group+".", "")
	origSchema := apiOp.Schemas.LookupSchema(origType)
	if _, ok := resource.Links["update"]; !ok && origSchema != nil && apiOp.AccessControl.CanUpdate(apiOp, resource.APIObject, origSchema) == nil {
		resource.Links["update"] = selfLink
	}
	if _, ok := resource.Links["delete"]; !ok && origSchema != nil && apiOp.AccessControl.CanDelete(apiOp, resource.APIObject, origSchema) == nil {
		resource.Links["delete"] = selfLink
	}

}

func setUpSteve(server *steve.Server) error {
	server.SchemaFactory.AddTemplate(steveschema.Template{
		Customize: func(schema *types.APISchema) {
			schema.Formatter = types.FormatterChain(schema.Formatter, formatter) // make sure the default formatter runs first so our formatter can apply changes to it
		},
	})
	return nil
}

// Register creates the APIService in kubernetes, starts a watch on APIResources, and registers the endpoint handlers.
func Register(ctx context.Context, router *mux.Router, config *wrangler.Context, server *steve.Server) {
	if !checkServerConfig(config.Core.ConfigMap()) {
		logrus.Infof("[%s] kube-apiserver is not configured for API server aggregation, the %s API will be disabled", apiServiceName, apiServiceName)
		return
	}
	err := setUpSteve(server)
	if err != nil {
		logrus.Fatal(err)
	}
	setUpAPIService(ctx, config)
	restConfig, err := config.ClientConfig.ClientConfig()
	if err != nil {
		logrus.Fatalf("[%s] failed to get rest config: %v", apiServiceName, err)
		return
	}
	mapper, _ := config.RESTClientGetter.ToRESTMapper()
	h := handler{
		restConfig:     restConfig,
		sarClient:      config.K8s.AuthorizationV1().SubjectAccessReviews(),
		namespaceCache: config.Core.Namespace().Cache(),
		apis:           WatchAPIResources(ctx, config.K8s.Discovery(), config.CRD.CustomResourceDefinition(), config.API.APIService(), mapper),
		clientGetter:   clientForRequest,
	}
	subRouter := router.PathPrefix("/apis").Subrouter()
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1", h.discoveryHandler).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/{resource}", h.globalHandler).Queries(queryKey, queryValue).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/{resource}", h.forwarder).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/namespaces/cattle-unscoped/{resource}", h.unscopedHandler).Queries(queryKey, queryValue).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/namespaces/cattle-unscoped/{resource}", h.unscopedHandler).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/namespaces/{project}/{resource}", h.scopedHandler).Queries(queryKey, queryValue).Methods(http.MethodGet)
	subRouter.HandleFunc("/resources.project.cattle.io/v1alpha1/namespaces/{project}/{resource}", h.scopedHandler).Methods(http.MethodGet)
	a := authHandler{
		configMapCache: config.Core.ConfigMap().Cache(),
	}
	subRouter.Use(a.authenticateAPIServer)
}

func checkServerConfig(configMapClient corecontrollers.ConfigMapClient) bool {
	config, err := configMapClient.Get(kubeSystemNamespace, extensionConfigMap, metav1.GetOptions{})
	if err != nil {
		logrus.Debugf("[%s] could not find %s/%s configmap", apiServiceName, kubeSystemNamespace, extensionConfigMap)
		return false
	}
	_, ok := config.Data[clientCAKey]
	if !ok {
		logrus.Debugf("[%s] could not find %s key in %s configmap", apiServiceName, clientCAKey, extensionConfigMap)
	}
	return ok
}

func setUpAPIService(ctx context.Context, config *wrangler.Context) {
	a := apiServiceHandler{
		secretCache:       config.Core.Secret().Cache(),
		apiServiceCache:   config.API.APIService().Cache(),
		apiServiceClient:  config.API.APIService(),
		namespaceCache:    config.Core.Namespace().Cache(),
		namespaceClient:   config.Core.Namespace(),
		clusterRoleCache:  config.RBAC.ClusterRole().Cache(),
		clusterRoleClient: config.RBAC.ClusterRole(),
		roleBindingCache:  config.RBAC.RoleBinding().Cache(),
		roleBindingClient: config.RBAC.RoleBinding(),
	}
	config.Core.Service().OnChange(ctx, "project-resources-apiservice", a.applyAPIService)
}

// applyAPIService creates or updates the APIService and other required resources.
func (a *apiServiceHandler) applyAPIService(key string, service *corev1.Service) (*corev1.Service, error) {
	if service == nil {
		return service, nil
	}
	if service.Namespace != namespace.System || (service.Name != rancherService && service.Name != cattleClusterAgentService) {
		return service, nil
	}
	err := a.ensureAPIService(service)
	if err != nil {
		return service, err
	}
	err = a.ensureNamespace()
	if err != nil {
		return service, err
	}
	err = a.ensureClusterRole()
	if err != nil {
		return service, err
	}
	err = a.ensureRoleBinding()
	return service, err
}

func getDynamicClient(config *wrangler.Context) (dynamic.Interface, error) {
	restConfig, err := config.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(restConfig)
}

func (a *apiServiceHandler) ensureAPIService(service *corev1.Service) error {
	logrus.Tracef("[%s] ensuring apiservice %s", apiServiceName, apiServiceName)
	var tlsSecret *corev1.Secret
	var err error
	if os.Getenv("CATTLE_DEV_MODE") == "" {
		tlsSecret, err = a.secretCache.Get(namespace.System, tls.InternalCA)
		if err != nil {
			return fmt.Errorf("could not get secret %s, %w", tls.InternalCA, err)
		}
		if len(tlsSecret.Data) == 0 || len(tlsSecret.Data[corev1.TLSCertKey]) == 0 {
			return fmt.Errorf("secret %s is not ready yet", tls.InternalCA)
		}
	}
	apiService := &apiregv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiServiceName,
		},
		Spec: apiregv1.APIServiceSpec{
			Group:                Group,
			GroupPriorityMinimum: priority,
			Version:              version,
			VersionPriority:      priority,
			Service: &apiregv1.ServiceReference{
				Namespace: service.Namespace,
				Name:      service.Name,
			},
		},
	}
	if tlsSecret == nil { // in dev mode, there is no CA
		apiService.Spec.InsecureSkipTLSVerify = true
	} else {
		apiService.Spec.CABundle = tlsSecret.Data[corev1.TLSCertKey]
	}
	currentAPIService, err := a.apiServiceCache.Get(apiServiceName)
	if apierrors.IsNotFound(err) {
		_, err = a.apiServiceClient.Create(apiService)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		currentAPIService, err = a.apiServiceClient.Get(apiServiceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(currentAPIService.Spec, apiService.Spec) {
		updatedAPIService := currentAPIService.DeepCopy()
		updatedAPIService.Spec = apiService.Spec
		_, err = a.apiServiceClient.Update(updatedAPIService)
		return err
	}
	return nil
}

func (a *apiServiceHandler) ensureNamespace() error {
	logrus.Tracef("[%s] ensuring namespace %s", apiServiceName, unscopedNamespace)
	// special non-project namespace to "contain" namespaces not in a project
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: unscopedNamespace,
			Annotations: map[string]string{
				NamespaceAnnotation: "true",
			},
		},
	}
	currentNamespace, err := a.namespaceCache.Get(unscopedNamespace)
	if apierrors.IsNotFound(err) {
		_, err = a.namespaceClient.Create(namespace)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		currentNamespace, err = a.namespaceClient.Get(unscopedNamespace, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(currentNamespace.ObjectMeta.Annotations, namespace.ObjectMeta.Annotations) {
		updatedNamespace := currentNamespace.DeepCopy()
		updatedNamespace.ObjectMeta.Annotations = namespace.ObjectMeta.Annotations
		_, err = a.namespaceClient.Update(updatedNamespace)
		return err
	}
	return nil
}

func (a *apiServiceHandler) ensureClusterRole() error {
	logrus.Tracef("[%s] ensuring clusterrole %s", apiServiceName, RoleName)
	// cluster role for listing resources in this APIGroup
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: RoleName,
			Annotations: map[string]string{
				AuthzAnnotation: "unscoped",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"list"},
				APIGroups: []string{Group},
				Resources: []string{"*"},
			},
		},
	}
	currentRole, err := a.clusterRoleCache.Get(RoleName)
	if apierrors.IsNotFound(err) {
		_, err = a.clusterRoleClient.Create(role)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		currentRole, err = a.clusterRoleClient.Get(RoleName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(currentRole.Rules, role.Rules) || !reflect.DeepEqual(currentRole.ObjectMeta.Annotations, role.ObjectMeta.Annotations) {
		updatedClusterRole := currentRole.DeepCopy()
		updatedClusterRole.ObjectMeta.Annotations = role.ObjectMeta.Annotations
		_, err = a.clusterRoleClient.Update(updatedClusterRole)
		return err
	}
	return nil
}

func (a *apiServiceHandler) ensureRoleBinding() error {
	logrus.Tracef("[%s] ensuring rolebinding %s/%s", apiServiceName, unscopedNamespace, RoleName)
	// default rolebinding for the special unscoped namespace, the prtb controller will create similar rolebindings for project namespaces for specific users
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleName,
			Namespace: unscopedNamespace,
			Annotations: map[string]string{
				AuthzAnnotation: "unscoped",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "system:authenticated", // all users can access this endpoint initially, the handler will do a SAR check before returning resources
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     RoleName,
		},
	}
	currentRoleBinding, err := a.roleBindingCache.Get(unscopedNamespace, RoleName)
	if apierrors.IsNotFound(err) {
		_, err = a.roleBindingClient.Create(roleBinding)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		currentRoleBinding, err = a.roleBindingClient.Get(unscopedNamespace, RoleName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(currentRoleBinding.ObjectMeta.Annotations, roleBinding.ObjectMeta.Annotations) {
		updatedRoleBinding := currentRoleBinding.DeepCopy()
		updatedRoleBinding.ObjectMeta.Annotations = currentRoleBinding.ObjectMeta.Annotations
		_, err = a.roleBindingClient.Update(updatedRoleBinding)
		return err
	}
	return nil
}
