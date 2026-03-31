package cloudcredential

import (
	"context"
	"fmt"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	steveext "github.com/rancher/steve/pkg/ext"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Delete implements [rest.GracefulDeleter]
func (s *Store) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	userInfo, isAdmin, err := s.userFrom(ctx, "delete")
	if err != nil {
		return nil, false, err
	}

	secret, err := s.GetSecret(name, request.NamespaceValue(ctx))
	if err != nil {
		return nil, false, err
	}

	credential, err := fromSecret(secret, s.dynamicSchemaCache)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error converting secret %s to credential: %w", name, err))
	}

	if deleteValidation != nil {
		if err := deleteValidation(ctx, credential); err != nil {
			return nil, false, err
		}
	}

	if !isAdmin && secret.Labels[LabelCloudCredentialOwner] != sanitizeLabelValue(userInfo.GetName()) {
		return nil, false, apierrors.NewForbidden(GVR.GroupResource(), name, fmt.Errorf("insufficient permissions to delete cloud credentials"))
	}

	// If an UID precondition exists and matches the credential UID, replace it with the secret's UID
	if options != nil &&
		options.Preconditions != nil &&
		options.Preconditions.UID != nil &&
		*options.Preconditions.UID == credential.UID {

		options.Preconditions.UID = &secret.UID
	}

	// Delete using the actual secret name, not the CloudCredential name
	if err := s.SystemStore.Delete(secret.Name, options); err != nil {
		return nil, false, err
	}

	return credential, true, nil
}

func (s *SystemStore) Delete(name string, options *metav1.DeleteOptions) error {
	err := s.secretClient.Delete(CredentialNamespace, name, options)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return apierrors.NewInternalError(fmt.Errorf("failed to delete cloud credential %s: %w", name, err))
}

// DeleteCollection implements [rest.CollectionDeleter]
func (s *Store) DeleteCollection(
	ctx context.Context,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
	listOptions *metainternalversion.ListOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, err := s.userFrom(ctx, "delete")
	if err != nil {
		return nil, err
	}

	convertedListOpts, err := steveext.ConvertListOptions(listOptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// Non-admin users are filtered by owner label at the API server level
	localOptions, err := toListOptions(convertedListOpts, userInfo, isAdmin)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w", err))
	}

	credList, err := s.SystemStore.list(localOptions)
	if err != nil {
		return nil, err
	}

	result := &ext.CloudCredentialList{
		ListMeta: credList.ListMeta,
		Items:    make([]ext.CloudCredential, 0, len(credList.Items)),
	}

	for i := range credList.Items {
		cred := &credList.Items[i]

		if !isAdmin && cred.Labels[LabelCloudCredentialOwner] != sanitizeLabelValue(userInfo.GetName()) {
			return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to delete cloud credential: %v", cred.Name))
		}

		if deleteValidation != nil {
			if err := deleteValidation(ctx, cred); err != nil {
				return nil, err
			}
		}

		if cred.Status.Secret != nil {
			if err := s.SystemStore.Delete(cred.Status.Secret.Name, options); err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("error deleting cloud credential %s: %w", cred.Name, err))
			}
		}

		result.Items = append(result.Items, *cred)
	}

	return result, nil
}
