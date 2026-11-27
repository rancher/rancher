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
	"k8s.io/apimachinery/pkg/types"
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

	if !isAdmin && secret.Labels[CloudCredentialOwnerLabel] != sanitizeLabelValue(userInfo.GetName()) {
		return nil, false, apierrors.NewNotFound(GVR.GroupResource(), name)
	}

	if deleteValidation != nil {
		if err := deleteValidation(ctx, credential); err != nil {
			return nil, false, err
		}
	}

	if err := checkPreconditions(optionsPreconditions(options), name, secret.ResourceVersion, credential.UID, secret.UID); err != nil {
		return nil, false, err
	}

	if options != nil && options.Preconditions != nil && options.Preconditions.UID != nil {
		preconditions := *options.Preconditions
		preconditions.UID = &secret.UID
		optionsCopy := *options
		optionsCopy.Preconditions = &preconditions
		options = &optionsCopy
	}

	// Delete using the actual secret name, not the CloudCredential name
	if err := s.deleteBackingSecret(secret.Name, credential.Name, options); err != nil {
		return nil, false, err
	}

	return credential, true, nil
}

func (s *SystemStore) Delete(name string, options *metav1.DeleteOptions) error {
	return s.deleteBackingSecret(name, name, options)
}

func (s *SystemStore) deleteBackingSecret(name, resource string, options *metav1.DeleteOptions) error {
	err := s.secretClient.Delete(CredentialNamespace, name, options)
	if err == nil {
		return nil
	}
	return mapBackingError(err, resource)
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
		return nil, apiStatusOrInternalError(err)
	}

	if namespace := request.NamespaceValue(ctx); namespace != metav1.NamespaceAll {
		if convertedListOpts.LabelSelector == "" {
			convertedListOpts.LabelSelector = fmt.Sprintf("%s=%s", CloudCredentialNamespaceLabel, namespace)
		} else {
			convertedListOpts.LabelSelector = fmt.Sprintf("%s,%s=%s", convertedListOpts.LabelSelector, CloudCredentialNamespaceLabel, namespace)
		}
	}

	// Non-admin users are filtered by owner label at the API server level
	localOptions, err := toListOptions(convertedListOpts, userInfo, isAdmin)
	if err != nil {
		return nil, apiStatusOrInternalError(err)
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

		if !isAdmin && cred.Labels[CloudCredentialOwnerLabel] != sanitizeLabelValue(userInfo.GetName()) {
			return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("insufficient permissions to delete cloud credential: %v", cred.Name))
		}

		if deleteValidation != nil {
			if err := deleteValidation(ctx, cred); err != nil {
				return nil, err
			}
		}

		if cred.Status.Secret != nil {
			if err := checkPreconditions(optionsPreconditions(options), cred.Name, cred.ResourceVersion, cred.UID, cred.Status.Secret.UID); err != nil {
				return nil, err
			}
			deleteOptions := copyDeleteOptionsForSecret(options, cred.Status.Secret.UID)
			if err := s.SystemStore.deleteBackingSecret(cred.Status.Secret.Name, cred.Name, deleteOptions); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return nil, err
			}
		}

		result.Items = append(result.Items, *cred)
	}

	return result, nil
}

func optionsPreconditions(options *metav1.DeleteOptions) *metav1.Preconditions {
	if options == nil {
		return nil
	}
	return options.Preconditions
}

func copyDeleteOptionsForSecret(options *metav1.DeleteOptions, uid types.UID) *metav1.DeleteOptions {
	if options == nil || options.Preconditions == nil || options.Preconditions.UID == nil {
		return options
	}
	preconditions := *options.Preconditions
	preconditions.UID = &uid
	optionsCopy := *options
	optionsCopy.Preconditions = &preconditions
	return &optionsCopy
}
