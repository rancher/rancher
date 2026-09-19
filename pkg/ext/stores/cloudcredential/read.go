package cloudcredential

import (
	"context"
	"fmt"
	"sync"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	steveext "github.com/rancher/steve/pkg/ext"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/features"
)

// Get implements [rest.Getter]
func (s *Store) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	userInfo, isAdmin, err := s.userFrom(ctx, "get")
	if err != nil {
		return nil, err
	}

	secret, err := s.GetSecret(name, request.NamespaceValue(ctx))
	if err != nil {
		return nil, err
	}

	// Non-admin users can only see their own credentials
	if !isAdmin && secret.Labels[CloudCredentialOwnerLabel] != sanitizeLabelValue(userInfo.GetName()) {
		return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
	}

	credential, err := fromSecret(secret, s.dynamicSchemaCache)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract cloud credential %s: %w", name, err))
	}

	return credential, nil
}

// GetSecret retrieves the backing secret for a cloud credential by name and request namespace.
// The name parameter is the CloudCredential name (stored in the label), not the Secret name.
func (s *SystemStore) GetSecret(name, namespace string) (*corev1.Secret, error) {
	// Find the secret by CloudCredential name label
	ls := labels.SelectorFromSet(map[string]string{CloudCredentialNameLabel: name})
	if namespace != metav1.NamespaceAll {
		req, err := labels.NewRequirement(CloudCredentialNamespaceLabel, "=", []string{namespace})
		if err != nil {
			return nil, err
		}
		ls = ls.Add(*req)
	}

	secrets, err := s.secretCache.List(CredentialNamespace, ls)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list cloud credential secrets: %w", err))
	}

	// Filter for valid cloud credential secrets (type must match our prefix)
	for _, secret := range secrets {
		if namespaceMatches(secret, namespace) && isCloudCredentialSecret(secret) {
			return secret, nil
		}
	}

	return nil, apierrors.NewNotFound(GVR.GroupResource(), name)
}

// NewList implements [rest.Lister]
func (s *Store) NewList() runtime.Object {
	objList := &ext.CloudCredentialList{}
	objList.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   GV.Group,
		Version: GV.Version,
		Kind:    "CloudCredentialList",
	})
	return objList
}

// List implements [rest.Lister]
func (s *Store) List(ctx context.Context, internaloptions *metainternalversion.ListOptions) (runtime.Object, error) {
	options, err := steveext.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// Extract namespace from request context and filter by it
	namespace := request.NamespaceValue(ctx)
	if namespace != metav1.NamespaceAll {
		labelSelector := fmt.Sprintf("%s=%s", CloudCredentialNamespaceLabel, namespace)
		if options.LabelSelector == "" {
			options.LabelSelector = labelSelector
		} else {
			options.LabelSelector = fmt.Sprintf("%s,%s", options.LabelSelector, labelSelector)
		}
	}

	return s.list(ctx, options)
}

func (s *Store) list(ctx context.Context, options *metav1.ListOptions) (*ext.CloudCredentialList, error) {
	userInfo, isAdmin, err := s.userFrom(ctx, "list")
	if err != nil {
		return nil, err
	}

	// Non-admin users are filtered by owner label at the API server level
	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to process list options: %w", err))
	}

	return s.SystemStore.list(listOptions)
}

func (s *SystemStore) list(options *metav1.ListOptions) (*ext.CloudCredentialList, error) {
	// Add label selector to only get cloud credential secrets
	if options.LabelSelector == "" {
		options.LabelSelector = fmt.Sprintf("%s=true", CloudCredentialLabel)
	} else {
		options.LabelSelector = fmt.Sprintf("%s,%s=true", options.LabelSelector, CloudCredentialLabel)
	}

	secrets, err := s.secretClient.List(CredentialNamespace, *options)
	if err != nil {
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
			return nil, apierrors.NewResourceExpired(err.Error())
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list cloud credentials: %w", err))
	}

	credentials := make([]ext.CloudCredential, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		// Double-check the secret type matches our prefix
		if !isCloudCredentialSecret(&secret) {
			continue
		}
		credential, err := fromSecret(&secret, s.dynamicSchemaCache)
		// ignore broken credentials
		if err != nil {
			continue
		}
		credentials = append(credentials, *credential)
	}

	return &ext.CloudCredentialList{
		ListMeta: metav1.ListMeta{
			ResourceVersion:    secrets.ResourceVersion,
			Continue:           secrets.Continue,
			RemainingItemCount: secrets.RemainingItemCount,
		},
		Items: credentials,
	}, nil
}

// watch implements the watch functionality for cloudcredentials
func (s *Store) Watch(ctx context.Context, internaloptions *metainternalversion.ListOptions) (watch.Interface, error) {
	options, err := steveext.ConvertListOptions(internaloptions)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	return s.watch(ctx, options)
}

func (s *Store) watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	userInfo, isAdmin, err := s.userFrom(ctx, "watch")
	if err != nil {
		return nil, err
	}

	listOptions, err := toListOptions(options, userInfo, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to convert list options: %w", err)
	}

	if !features.FeatureGates().Enabled(features.WatchListClient) {
		listOptions.SendInitialEvents = nil
		listOptions.ResourceVersionMatch = ""
	}

	secretWatch, err := s.secretClient.Watch(CredentialNamespace, *listOptions)
	if err != nil {
		logrus.Errorf("cloudcredential: watch: error starting watch: %s", err)
		return nil, apierrors.NewInternalError(fmt.Errorf("cloudcredential: watch: error starting watch: %w", err))
	}

	cloudCredentialWatch := &watcher{
		ch: make(chan watch.Event, 100),
	}

	go func() {
		defer secretWatch.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, more := <-secretWatch.ResultChan():
				if !more {
					return
				}

				var obj runtime.Object
				switch event.Type {
				case watch.Bookmark:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("cloudcredential: watch: expected secret got %T", event.Object)
						continue
					}
					obj = &ext.CloudCredential{
						ObjectMeta: metav1.ObjectMeta{
							ResourceVersion: secret.ResourceVersion,
							Annotations:     secret.Annotations,
							Labels:          secret.Labels,
						},
					}
				case watch.Added, watch.Modified, watch.Deleted:
					secret, ok := event.Object.(*corev1.Secret)
					if !ok {
						logrus.Warnf("cloudcredential: watch: expected secret got %T", event.Object)
						continue
					}
					obj, err = fromSecret(secret, s.dynamicSchemaCache)
					if err != nil {
						logrus.Errorf("cloudcredential: watch: error converting secret '%s' to credential: %s", secret.Name, err)
						continue
					}
				default:
					obj = event.Object
				}

				if !cloudCredentialWatch.addEvent(watch.Event{
					Type:   event.Type,
					Object: obj,
				}) {
					return
				}
			}
		}
	}()

	return cloudCredentialWatch, nil
}

// watcher implements [watch.Interface]
type watcher struct {
	mu     sync.RWMutex
	closed bool
	ch     chan watch.Event
}

func (w *watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}
	close(w.ch)
	w.closed = true
}

func (w *watcher) ResultChan() <-chan watch.Event { return w.ch }

func (w *watcher) addEvent(event watch.Event) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return false
	}

	w.ch <- event
	return true
}
