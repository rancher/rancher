package ext

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	wapiv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	wappsv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	apiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
)

const (
	apiSvcName = "v1alpha1.ext.cattle.io"
	// Port is used both for APIService & remotedialer tunnel
	apiSvcPort     = int32(5554)
	svcName        = "api-extension"
	namespace      = "cattle-system"
	deploymentName = "api-extension"
	serviceName    = "api-extension"

	tlsName  = "apiserver-poc.default.svc"
	certName = "cattle-apiextension-tls"
	caName   = "cattle-apiextension-ca"
)

type handler struct {
	apiClient        wapiv1.APIServiceClient
	serviceClient    wcorev1.ServiceClient
	deploymentClient wappsv1.DeploymentClient
	podClient        wcorev1.PodClient
	saClient         wcorev1.ServiceAccountClient
	crClient         wrbacv1.ClusterRoleClient
	crbClient        wrbacv1.ClusterRoleBindingClient

	cancel       context.CancelFunc
	remoteTunnel *remoteTunnel
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		apiClient:        clients.API.APIService(),
		serviceClient:    clients.Core.Service(),
		deploymentClient: clients.Apps.Deployment(),
		podClient:        clients.Core.Pod(),
		saClient:         clients.Core.ServiceAccount(),
		crClient:         clients.RBAC.ClusterRole(),
		crbClient:        clients.RBAC.ClusterRoleBinding(),
		remoteTunnel: &remoteTunnel{
			restConfig: clients.RESTConfig,
			podClient:  clients.Core.Pod(),
		},
	}
	clients.Mgmt.Setting().OnChange(ctx, "api-extension", h.OnChange)

	go func() {
		router := mux.NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				userName := req.Header.Get("X-Remote-User")
				groups := req.Header.Get("X-Remote-Groups")
				ctx = request.WithUser(req.Context(), &user.DefaultInfo{
					Name:   userName,
					Groups: []string{groups},
				})
				req = req.WithContext(ctx)

				next.ServeHTTP(w, req)
			})
		})
		ext.RegisterSubRoutes(router, clients)
		err := server.ListenAndServe(ctx, 5555, 0, router, &server.ListenOpts{
			Secrets:       clients.Core.Secret(),
			CAName:        caName,
			CANamespace:   namespace,
			CertName:      certName,
			CertNamespace: namespace,
			TLSListenerConfig: dynamiclistener.Config{
				SANs: []string{tlsName},
				FilterCN: func(cns ...string) []string {
					return []string{tlsName}
				},
			},
		})
		if err != nil {
			panic(err)
		}
		<-ctx.Done()
	}()
}

func (h *handler) OnChange(key string, setting *mgmt.Setting) (*mgmt.Setting, error) {
	if setting == nil || setting.Name != settings.ImperativeAPIExtension.Name {
		return setting, nil
	}

	switch getEffectiveValue(setting) {
	case "on":
		if err := h.ensureAPIExtensionEnabled(); err != nil {
			return setting, err
		}
		var ctx context.Context
		ctx, h.cancel = context.WithCancel(context.Background())
		go h.remoteTunnel.Start(ctx)
	case "off":
		if err := h.ensureAPIExtensionDisabled(); err != nil {
			return setting, err
		}
		if h.cancel != nil {
			h.cancel()
		}
		h.remoteTunnel.Stop()
	}

	return setting, nil
}

func (h *handler) ensureAPIExtensionEnabled() error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
	}
	// TODO: Handle update
	_, err := h.saClient.Create(sa)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ServiceAccount %s: %w", sa.Name, err)
	}

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	// TODO: Handle update
	_, err = h.crClient.Create(cr)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRole %s: %w", cr.Name, err)
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     cr.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}
	// TODO: Handle update
	_, err = h.crbClient.Create(crb)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRoleBinding %s: %w", crb.Name, err)
	}

	apiService := &apiv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiSvcName,
		},
		Spec: apiv1.APIServiceSpec{
			Service: &apiv1.ServiceReference{
				Namespace: namespace,
				Name:      svcName,
				Port:      ptr.To(apiSvcPort),
			},
			Group:   tokens.SchemeGroupVersion.Group,
			Version: tokens.SchemeGroupVersion.Version,
			// TODO: this done for POC sake, but shouldn't be done for prod
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  100,
			VersionPriority:       100,
		},
	}

	// TODO: Handle update
	_, err = h.apiClient.Create(apiService)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create APIService %s: %w", apiService.Name, err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "api-extension",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "api-extension",
					},
				},
				Spec: corev1.PodSpec{
					// TODO: Use api-extension specific service account
					ServiceAccountName: sa.Name,
					Containers: []corev1.Container{
						{
							Name:            "api-extension",
							Image:           settings.ImperativeAPIImage.Get(),
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: apiSvcPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	// TODO: Handle update
	_, err = h.deploymentClient.Create(deployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Deployment %s: %w", deployment.Name, err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "api-extension",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       apiSvcPort,
					TargetPort: intstr.FromInt32(apiSvcPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	// TODO: Handle update
	_, err = h.serviceClient.Create(service)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Service %s: %w", service.Name, err)
	}

	return nil
}

func (h *handler) ensureAPIExtensionDisabled() error {
	err := h.apiClient.Delete(apiSvcName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete APIService %s: %w", apiSvcName, err)
	}

	err = h.deploymentClient.Delete(namespace, deploymentName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Deployment %s: %w", deploymentName, err)
	}

	err = h.serviceClient.Delete(namespace, serviceName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Service %s: %w", serviceName, err)
	}

	return nil
}

func getEffectiveValue(setting *mgmt.Setting) string {
	value := setting.Value
	if value == "" {
		value = setting.Default
	}
	return value
}

type remoteTunnel struct {
	restConfig *rest.Config
	podClient  wcorev1.PodClient

	running atomic.Bool
}

func (r *remoteTunnel) Start(ctx context.Context) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}

	for {
		err := r.start(ctx)
		if err != nil {
			logrus.Error(err)
			time.Sleep(time.Second)
			continue
		}

		break
	}
}

func (r *remoteTunnel) Stop() {
	r.running.CompareAndSwap(true, false)
}

func (r *remoteTunnel) start(ctx context.Context) error {
	readyCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		err := r.runForwarder(ctx, readyCh)
		errCh <- err
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			return err
		case <-readyCh:
		}
		break
	}

	dialer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			// TODO: Only there for a POC
			InsecureSkipVerify: true,
		},
	}
	err := remotedialer.ClientConnect(
		ctx,
		fmt.Sprintf("wss://localhost:%d/connect", apiSvcPort),
		// TODO: Shared secret here would be a good idea
		http.Header{},
		dialer,
		func(string, string) bool { return true },
		nil,
	)
	return err
}

func (r *remoteTunnel) runForwarder(ctx context.Context, readyCh chan struct{}) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(r.restConfig)
	if err != nil {
		return err
	}

	var podName string
	for podName == "" {
		pods, err := r.podClient.List(namespace, metav1.ListOptions{
			LabelSelector: "app=api-extension",
		})
		if err != nil {
			return err
		}

		if len(pods.Items) != 1 {
			time.Sleep(time.Second)
			continue
		}

		podName = pods.Items[0].Name
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimPrefix(r.restConfig.Host, "https://")
	serverURL := url.URL{
		Scheme: "https",
		Path:   path,
		Host:   hostIP,
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{
		Transport: roundTripper,
	}, http.MethodPost, &serverURL)

	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d", apiSvcPort)}, ctx.Done(), readyCh, out, errOut)
	if err != nil {
		return err
	}

	err = forwarder.ForwardPorts()
	if err != nil {
		return err
	}

	return nil
}
