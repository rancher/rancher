package apiservice

import (
	"context"
	"crypto/sha256"
	"encoding/base64"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	appscontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	serviceAccounts corev1controllers.ServiceAccountCache
	deploymentCache appscontrollers.DeploymentCache
	daemonSetCache  appscontrollers.DaemonSetCache
	secretsCache    corev1controllers.SecretCache
	secretsClient   corev1controllers.SecretClient
	settings        mgmtcontrollers.SettingClient
	apiServices     mgmtcontrollers.APIServiceCache
	services        corev1controllers.ServiceCache
	embedded        bool
	ctx             context.Context
}

func Register(ctx context.Context, context *wrangler.Context, embedded bool) {
	h := &handler{
		serviceAccounts: context.Core.ServiceAccount().Cache(),
		deploymentCache: context.Apps.Deployment().Cache(),
		daemonSetCache:  context.Apps.DaemonSet().Cache(),
		secretsCache:    context.Core.Secret().Cache(),
		secretsClient:   context.Core.Secret(),
		settings:        context.Mgmt.Setting(),
		apiServices:     context.Mgmt.APIService().Cache(),
		services:        context.Core.Service().Cache(),
		embedded:        embedded,
		ctx:             ctx,
	}

	relatedresource.WatchClusterScoped(ctx, "apiservice-watch-owner",
		relatedresource.OwnerResolver(false, v3.SchemeGroupVersion.String(), "APIService"),
		context.Mgmt.APIService(), context.Core.ServiceAccount())

	relatedresource.WatchClusterScoped(ctx, "apiservice-watch-settings", h.resolveSettingToAPIServices,
		context.Mgmt.APIService(), context.Mgmt.Setting())

	context.Mgmt.Setting().OnChange(ctx, "apiservice-settings", h.SetupInternalServerURL)
	mgmtcontrollers.RegisterAPIServiceGeneratingHandler(ctx, context.Mgmt.APIService(),
		context.Apply.
			WithSetOwnerReference(true, false).
			WithCacheTypes(context.Core.ServiceAccount(),
				context.Core.Secret()),
		"", "apiservice", h.OnChange, nil)
}

func (h *handler) resolveSettingToAPIServices(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if name != settings.ServerURL.Name {
		return nil, nil
	}
	services, err := h.apiServices.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var result []relatedresource.Key
	for _, service := range services {
		result = append(result, relatedresource.Key{
			Name: service.Name,
		})
	}
	return result, nil
}

func (h *handler) OnChange(obj *v3.APIService, status v3.APIServiceStatus) ([]runtime.Object, v3.APIServiceStatus, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName(obj.Name, "api-service"),
			Namespace: namespace.System,
		},
	}

	objs := []runtime.Object{
		sa,
	}

	status.ServiceAccountName = sa.Name
	status.ServiceAccountNamespace = sa.Namespace

	if obj.Spec.SecretNamespace == "" ||
		obj.Spec.SecretName == "" {
		return objs, status, nil
	}

	token, err := h.getToken(sa)
	if err != nil || len(token) == 0 {
		return objs, status, err
	}

	if len(token) == 0 {
		return objs, status, nil
	}

	internalServer := settings.InternalServerURL.Get()
	if internalServer == "" {
		return objs, status, nil
	}
	internalCA := settings.InternalCACerts.Get()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Spec.SecretName,
			Namespace: obj.Spec.SecretNamespace,
		},
		Data: map[string][]byte{
			"token": []byte(token),
			"url":   []byte(internalServer + "/v3/connect"),
		},
	}
	if len(internalCA) > 0 {
		secret.Data["ca.crt"] = []byte(internalCA)
	}

	return append(objs, secret), status, nil
}

func (h *handler) getToken(sa *corev1.ServiceAccount) (string, error) {
	sa, err := h.serviceAccounts.Get(sa.Namespace, sa.Name)
	if apierror.IsNotFound(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	// create a secret-based token for the service account
	sName := serviceaccounttoken.ServiceAccountSecretName(sa)
	secret, err := h.secretsCache.Get(sa.Namespace, sName)
	if err != nil {
		if !apierror.IsNotFound(err) {
			return "", err
		}
		sc := serviceaccounttoken.SecretTemplate(sa)
		secret, err = h.secretsClient.Create(sc)
		if err != nil {
			if !apierror.IsAlreadyExists(err) {
				return "", err
			}
			secret, err = h.secretsCache.Get(sa.Namespace, sName)
			if err != nil {
				if !apierror.IsNotFound(err) {
					return "", err
				}
				secret, err = h.secretsClient.Get(sa.Namespace, sName, metav1.GetOptions{})
				if err != nil {
					return "", err
				}
			}
		}
	}
	token := secret.Data["token"]
	if len(token) == 0 {
		return "", nil
	}

	hash := sha256.Sum256(token)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}
