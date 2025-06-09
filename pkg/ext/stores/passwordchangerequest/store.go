package passwordchangerequest

// tokens implements the store for the new public API token resources, also
// known as ext tokens.
import (
	"context"
	"fmt"
	"unicode/utf8"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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

type PasswordUpdater interface {
	VerifyAndUpdatePassword(userId string, currentPassword, newPassword string) error
	UpdatePassword(userId string, newPassword string) error
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

type Store struct {
	authorizer authorizer.Authorizer
	pwdUpdater PasswordUpdater
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
	err := validatePassword(objPasswordChangeRequest.Spec.NewPassword, settings.PasswordMinLength.GetInt())
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("error validating password: %s", err.Error()))
	}

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
		err := s.pwdUpdater.UpdatePassword(objPasswordChangeRequest.Spec.UserID, objPasswordChangeRequest.Spec.NewPassword)
		if err != nil {
			return nil, apierrors.NewUnauthorized(fmt.Sprintf("error checking permissions %s", err.Error()))
		}

		c := metav1.Condition{
			Type:   "PasswordUpdated",
			Status: "True",
		}
		objPasswordChangeRequest.Status.Conditions = append(objPasswordChangeRequest.Status.Conditions, c)

		return objPasswordChangeRequest, nil
	}

	if userInfo.GetName() == objPasswordChangeRequest.Spec.UserID {
		err := s.pwdUpdater.VerifyAndUpdatePassword(objPasswordChangeRequest.Spec.UserID, objPasswordChangeRequest.Spec.CurrentPassword, objPasswordChangeRequest.Spec.NewPassword)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error updating password: %w", err))
		}
		c := metav1.Condition{
			Type:   "PasswordUpdated",
			Status: "True",
		}
		objPasswordChangeRequest.Status.Conditions = append(objPasswordChangeRequest.Status.Conditions, c)

		return objPasswordChangeRequest, nil
	}

	return objPasswordChangeRequest, apierrors.NewUnauthorized("not authorized to update password")
}

// canUpdateAnyPassword verifies the user can update users and secrets in the cattle-local-user-passwords namespace.
func (s *Store) canUpdateAnyPassword(ctx context.Context, userInfo user.Info) (bool, error) {
	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            "update",
		APIGroup:        v3.GroupName,
		APIVersion:      v3.Version,
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

// validatePassword will ensure a password is at least the minimum required length in runes,
func validatePassword(password string, minPassLen int) error {
	if utf8.RuneCountInString(password) < minPassLen {
		return fmt.Errorf("password must be at least %v characters", minPassLen)
	}

	return nil
}
