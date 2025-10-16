// groupmembershiprefreshrequest implements the store for the imperative groupmembershiprefreshrequest resource.
package groupmembershiprefreshrequest

import (
	"context"
	"fmt"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/controllers/status"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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
	SingularName = "groupmembershiprefreshrequest"
	kind         = "GroupMembershipRefreshRequest"
	allUsers     = "*"
)

var (
	_ rest.Creater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)

var GVK = ext.SchemeGroupVersion.WithKind(kind)

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

	userManager, err := common.NewUserManagerNoBindings(wranglerContext)
	if err != nil {
		return nil, err
	}
	scaledContext.UserManager = userManager

	userAuthRefresher := providerrefresh.NewUserAuthRefresher(scaledContext)
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
	return &ext.GroupMembershipRefreshRequest{}
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

	objGroupMembershipRefreshRequest, ok := obj.(*ext.GroupMembershipRefreshRequest)
	if !ok {
		var zeroT *ext.GroupMembershipRefreshRequest
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T",
			zeroT, obj))
	}
	if objGroupMembershipRefreshRequest.Spec.UserID == "" {
		return nil, apierrors.NewBadRequest("user ID must be set")
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

	if objGroupMembershipRefreshRequest.Spec.UserID == allUsers {
		s.userAuthRefresher.TriggerAllUserRefresh()
	} else {
		s.userAuthRefresher.TriggerUserRefresh(objGroupMembershipRefreshRequest.Spec.UserID, true)
	}

	objGroupMembershipRefreshRequest.Status = ext.GroupMembershipRefreshRequestStatus{
		Conditions: []metav1.Condition{{
			LastTransitionTime: metav1.Now(),
			Type:               "UserRefreshInitiated",
			Status:             metav1.ConditionTrue,
		}},
		Summary: status.SummaryCompleted,
	}

	return objGroupMembershipRefreshRequest, nil
}
