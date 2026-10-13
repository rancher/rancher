package clusters

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/handlers"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/api/steve/norman"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/features"
	normanv3 "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/steve/pkg/podimpersonation"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8srequest "k8s.io/apiserver/pkg/endpoints/request"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

func Register(ctx context.Context, server *steve.Server, wrangler *wrangler.Context, userManager user.Manager) error {
	log := &log{
		cg: server.ClientFactory,
	}

	sc, err := config.NewScaledContext(*wrangler.RESTConfig, nil)
	if err != nil {
		return err
	}

	sc.Wrangler = wrangler
	sc.UserManager = userManager

	kubeconfig := kubeconfigDownload{
		tokenMgr:  tokens.NewManager(sc.Wrangler),
		authToken: requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, sc),
	}

	var shellHandler *shell
	if features.ClusterShell.Enabled() {
		shellHandler = &shell{
			cg:              server.ClientFactory,
			namespace:       "cattle-system",
			impersonator:    podimpersonation.New("shell", server.ClientFactory, time.Hour, settings.FullShellImage),
			clusterRegistry: server.ClusterRegistry,
		}

		server.ClusterCache.OnAdd(ctx, shellHandler.impersonator.PurgeOldRoles)
		server.ClusterCache.OnChange(ctx, func(gvk schema.GroupVersionKind, key string, obj, oldObj runtime.Object) error {
			return shellHandler.impersonator.PurgeOldRoles(gvk, key, obj)
		})
	}

	authorizationClient, err := authorizationv1.NewForConfig(wrangler.RESTConfig)
	if err != nil {
		return err
	}
	subjectAccessReviews := authorizationClient.SubjectAccessReviews()

	server.BaseSchemas.MustImportAndCustomize(GenerateKubeconfigOutput{}, nil)
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group:     "management.cattle.io",
		Kind:      "Cluster",
		Formatter: norman.NewLinksAndActionsFormatter(wrangler.MultiClusterManager, normanv3.Version, "cluster"),
		Customize: func(schema *types.APISchema) {
			if schema.LinkHandlers == nil {
				schema.LinkHandlers = map[string]http.Handler{}
			}
			if shellHandler != nil {
				schema.LinkHandlers["shell"] = shellHandler
			}
			schema.LinkHandlers["log"] = log
			if schema.ActionHandlers == nil {
				schema.ActionHandlers = map[string]http.Handler{}
			}
			schema.ActionHandlers["generateKubeconfig"] = kubeconfig
			if schema.ResourceActions == nil {
				schema.ResourceActions = map[string]schemas.Action{}
			}
			schema.ResourceActions["generateKubeconfig"] = schemas.Action{
				Output: "generateKubeconfigOutput",
			}
			schema.ByIDHandler = func(request *types.APIRequest) (types.APIObject, error) {
				if request.Name == "local" && request.Link == "shell" && shellHandler != nil {
					requestUser, ok := k8srequest.UserFrom(request.Request.Context())
					if !ok || requestUser == nil || requestUser.GetName() == "" {
						return types.APIObject{}, validation.Unauthorized
					}

					extra := make(map[string]authzv1.ExtraValue, len(requestUser.GetExtra()))
					for key, values := range requestUser.GetExtra() {
						extra[key] = authzv1.ExtraValue(values)
					}

					review, err := subjectAccessReviews.Create(
						request.Request.Context(),
						&authzv1.SubjectAccessReview{
							Spec: authzv1.SubjectAccessReviewSpec{
								User:   requestUser.GetName(),
								UID:    requestUser.GetUID(),
								Groups: requestUser.GetGroups(),
								Extra:  extra,
								ResourceAttributes: &authzv1.ResourceAttributes{
									Verb:      "get",
									Group:     "management.cattle.io",
									Version:   "v3",
									Resource:  "clusters",
									Name:      "local",
									Namespace: "",
								},
							},
						},
						metav1.CreateOptions{},
					)
					if err != nil {
						return types.APIObject{}, err
					}
					if !review.Status.Allowed || review.Status.Denied ||
						review.Status.EvaluationError != "" {
						return types.APIObject{}, apierror.NewAPIError(
							validation.PermissionDenied,
							"not authorized to access the local cluster shell",
						)
					}
					shellHandler.ServeHTTP(request.Response, request.Request)
					return types.APIObject{}, validation.ErrComplete
				}
				return handlers.ByIDHandler(request)
			}
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Project",
		Customize: func(schema *types.APISchema) {
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "",
		Kind:  "Namespace",
		Customize: func(schema *types.APISchema) {
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})

	return nil
}
