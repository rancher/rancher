// userrefreshrequest implements the store for the new public API userrefreshrequest resources
package userrefreshrequest

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/types/config"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/wrangler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	SingularName = "userrefreshrequest"
	PluralName   = SingularName + "s"
)

var GV = schema.GroupVersion{
	Group:   "ext.cattle.io",
	Version: "v1",
}

var GVK = schema.GroupVersionKind{
	Group:   GV.Group,
	Version: GV.Version,
	Kind:    "UserRefreshRequest",
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

type Store struct {
	authorizer        authorizer.Authorizer
	userAuthRefresher providerrefresh.UserAuthRefresher
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

func New(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) (*Store, error) {
	scaledContext, err := config.NewScaledContext(*wranglerContext.RESTConfig, &config.ScaleContextOptions{
		ControllerFactory: wranglerContext.ControllerFactory,
	})
	scaledContext.Wrangler = wranglerContext
	if err != nil {
		return nil, err
	}
	userAuthRefresher := providerrefresh.NewUserAuthRefresher(context.TODO(), scaledContext)
	store := Store{
		userAuthRefresher: userAuthRefresher,
		authorizer:        authorizer,
	}
	return &store, nil
}

// GroupVersionKind implements [rest.GroupVersionKindProvider], a required interface.
func (s *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper], a required interface.
func (s *Store) NamespaceScoped() bool {
	return false
}

// GetSingularName implements [rest.SingularNameProvider], a required interface.
func (s *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage], a required interface.
func (s *Store) New() runtime.Object {
	obj := &ext.UserRefreshRequest{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage], a required interface.
func (s *Store) Destroy() {
}

// Create implements [rest.Creator], the interface to support the `create`
// verb. Delegates to the actual store method after some generic boilerplate.
func (s *Store) Create(
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
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	objUserRefreshRequest, ok := obj.(*ext.UserRefreshRequest)
	if !ok {
		var zeroT *ext.UserRefreshRequest
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T",
			zeroT, obj))
	}
	if !objUserRefreshRequest.Spec.All && objUserRefreshRequest.Spec.UserID == "" {
		return nil, apierrors.NewBadRequest("user ID or 'all' must be set")
	}

	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("can't get user info from context"))
	}
	// Only users that can create users are allowed to refresh UserAttributes. This is the same as the current Norman validation.
	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "create",
		APIGroup:        v3.UserGroupVersionKind.Group,
		APIVersion:      v3.Version,
		Resource:        v3.UserResource.Name,
		ResourceRequest: true,
	})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error checking permissions %w", err))
	}
	if decision != authorizer.DecisionAllow {
		return nil, apierrors.NewUnauthorized("not authorized to refresh user attributes")
	}

	if dryRun {
		return obj, nil
	}

	if objUserRefreshRequest.Spec.All {
		s.userAuthRefresher.TriggerAllUserRefresh()
	} else if objUserRefreshRequest.Spec.UserID != "" {
		s.userAuthRefresher.TriggerUserRefresh(objUserRefreshRequest.Spec.UserID, true)
	}

	c := metav1.Condition{
		Type:   "UserRefreshInitiated",
		Status: "True",
	}
	objUserRefreshRequest.Status.Conditions = append(objUserRefreshRequest.Status.Conditions, c)

	return objUserRefreshRequest, nil
}
