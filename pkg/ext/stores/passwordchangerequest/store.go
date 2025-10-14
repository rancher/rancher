// passwordchangerequest implements the store for the imperative passwordchangerequest resource.
package passwordchangerequest

import (
	"context"
	"fmt"
	"unicode/utf8"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	SingularName = "passwordchangerequest"
	kind         = "PasswordChangeRequest"
)

var (
	_ rest.Creater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)

var GVK = ext.SchemeGroupVersion.WithKind(kind)

type PasswordUpdater interface {
	VerifyAndUpdatePassword(userId string, currentPassword, newPassword string) error
	UpdatePassword(userId string, newPassword string) error
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

type Store struct {
	authorizer           authorizer.Authorizer
	pwdUpdater           PasswordUpdater
	userCache            mgmtv3.UserCache
	getPasswordMinLength func() int
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// New is a convenience function for creating a password change request
// store. It initializes the returned store from the provided wrangler context.
func New(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	pwdManager := pbkdf2.New(wranglerContext.Core.Secret().Cache(), wranglerContext.Core.Secret())

	return &Store{
		pwdUpdater:           pwdManager,
		authorizer:           authorizer,
		userCache:            wranglerContext.Mgmt.User().Cache(),
		getPasswordMinLength: settings.PasswordMinLength.GetInt,
	}
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
	return &ext.PasswordChangeRequest{}
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
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	if createValidation != nil {
		err := createValidation(ctx, obj)
		if err != nil {
			return obj, err
		}
	}

	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("can't get user info from context"))
	}

	req, ok := obj.(*ext.PasswordChangeRequest)
	if !ok {
		var zeroT *ext.PasswordChangeRequest
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T", zeroT, obj))
	}

	// UserID is required.
	if req.Spec.UserID == "" {
		return nil, apierrors.NewBadRequest("userID is required")
	}

	// Password must be at least the minimum required length.
	if minLength := s.getPasswordMinLength(); utf8.RuneCountInString(req.Spec.NewPassword) < minLength {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("password must be at least %d characters", minLength))
	}

	user, err := s.userCache.Get(req.Spec.UserID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("user %s not found", req.Spec.UserID))
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("can't get user %s: %w", req.Spec.UserID, err))
	}

	// Password must not be the same as the username.
	if req.Spec.NewPassword == user.Username {
		return nil, apierrors.NewBadRequest("password cannot be the same as the username")
	}

	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	if dryRun {
		return obj, nil
	}

	canUpdateAnyPassword, err := s.canUpdateAnyPassword(ctx, userInfo)
	if err != nil {
		return nil, err
	}

	// Checking the current password is only required if the user doesn't have permissions on update on users and update
	// secrets in the cattle-local-user-passwords namespace.
	if canUpdateAnyPassword {
		err := s.pwdUpdater.UpdatePassword(req.Spec.UserID, req.Spec.NewPassword)
		if err != nil {
			return nil, apierrors.NewUnauthorized(fmt.Sprintf("error checking permissions %s", err.Error()))
		}

		req.Status = ext.PasswordChangeRequestStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "PasswordUpdated",
					Status: metav1.ConditionTrue,
				},
			},
			Summary: status.SummaryCompleted,
		}

		return req, nil
	}

	// Ordinary users can only change their own password and must provide their current password.
	if userInfo.GetName() == req.Spec.UserID {
		err := s.pwdUpdater.VerifyAndUpdatePassword(req.Spec.UserID, req.Spec.CurrentPassword, req.Spec.NewPassword)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error updating password: %w", err))
		}
		req.Status = ext.PasswordChangeRequestStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "PasswordUpdated",
					Status: metav1.ConditionTrue,
				},
			},
			Summary: status.SummaryCompleted,
		}

		return req, nil
	}

	return req, apierrors.NewUnauthorized("not authorized to update password")
}

// canUpdateAnyPassword verifies the user can update users and secrets in the cattle-local-user-passwords namespace.
func (s *Store) canUpdateAnyPassword(ctx context.Context, userInfo user.Info) (bool, error) {
	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "update",
		APIGroup:        mgmt.GroupName,
		APIVersion:      "v3",
		Resource:        "users",
		ResourceRequest: true,
	})
	if err != nil {
		return false, apierrors.NewInternalError(fmt.Errorf("error checking permissions %w", err))
	}
	if decision != authorizer.DecisionAllow {
		return false, nil
	}
	decision, _, err = s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "update",
		Namespace:       pbkdf2.LocalUserPasswordsNamespace,
		APIVersion:      "v1",
		Resource:        "secrets",
		ResourceRequest: true,
	})
	if err != nil {
		return false, apierrors.NewInternalError(fmt.Errorf("error checking permissions %w", err))
	}

	return decision == authorizer.DecisionAllow, nil
}
