package passwordchangerequest

// tokens implements the store for the new public API token resources, also
// known as ext tokens.
import (
	"context"
	"fmt"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	SingularName = "passwordchangerequest"
	PluralName   = SingularName + "s"
)

var GV = schema.GroupVersion{
	Group:   "ext.cattle.io",
	Version: "v1",
}

var GVK = schema.GroupVersionKind{
	Group:   GV.Group,
	Version: GV.Version,
	Kind:    "PasswordChangeRequest",
}
var GVR = schema.GroupVersionResource{
	Group:    GV.Group,
	Version:  GV.Version,
	Resource: "passwordchangerequests",
}

type PasswordUpdater interface {
	UpdatePassword(userId string, currentPassword, newPassword string) error
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

type Store struct {
	secretClient v1.SecretClient
	secretCache  v1.SecretCache
	authorizer   authorizer.Authorizer
	pwdUpdater   PasswordUpdater
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// NewSystemFromWrangler is a convenience function for creating a system token
// store. It initializes the returned store from the provided wrangler context.
func New(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	pwdManager := pbkdf2.New(wranglerContext.Core.Secret().Cache(), wranglerContext.Core.Secret())

	store := Store{
		pwdUpdater: pwdManager,
		authorizer: authorizer,
	}
	return &store
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
	obj := &ext.PasswordChangeRequest{}
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

	objPasswordChangeRequest, ok := obj.(*ext.PasswordChangeRequest)
	if !ok {
		var zeroT *ext.PasswordChangeRequest
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T",
			zeroT, obj))
	}

	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("can't get user info from context"))
	}
	if userInfo.GetName() != objPasswordChangeRequest.Spec.UserID {
		decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
			User:            userInfo,
			Verb:            "update",
			APIGroup:        v3.GroupName,
			APIVersion:      v3.Version,
			Resource:        "users",
			ResourceRequest: true,
		})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error checking permissions %w", err))
		}
		if decision != authorizer.DecisionAllow {
			return nil, apierrors.NewUnauthorized("not authorized to update password")
		}
		decision, _, err = s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
			User:            userInfo,
			Verb:            "update",
			Namespace:       "cattle-local-user-passwords", //TODO const
			APIVersion:      "v1",
			Resource:        "secrets",
			ResourceRequest: true,
		})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error checking permissions %w", err))
		}
		if decision != authorizer.DecisionAllow {
			return nil, apierrors.NewUnauthorized("not authorized to update password")
		}
	}

	if dryRun {
		return obj, nil
	}

	if err := s.pwdUpdater.UpdatePassword(objPasswordChangeRequest.Spec.UserID, objPasswordChangeRequest.Spec.CurrentPassword, objPasswordChangeRequest.Spec.NewPassword); err != nil {
		c := metav1.Condition{
			Type:    "PasswordUpdated",
			Status:  "False",
			Reason:  err.Error(),
			Message: "Failed to update password",
		}
		objPasswordChangeRequest.Status.Conditions = append(objPasswordChangeRequest.Status.Conditions, c)

		return objPasswordChangeRequest, nil
	}

	c := metav1.Condition{
		Type:   "PasswordUpdated",
		Status: "True",
	}
	objPasswordChangeRequest.Status.Conditions = append(objPasswordChangeRequest.Status.Conditions, c)

	return objPasswordChangeRequest, nil
}
