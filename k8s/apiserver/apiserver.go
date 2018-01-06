package apiserver

import (
	"net"

	"context"

	"net/http"

	"fmt"

	"github.com/rancher/norman/signal"
	"github.com/rancher/norman/types/slice"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionscmd "k8s.io/apiextensions-apiserver/pkg/cmd/server"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	kubeexternalinformers "k8s.io/client-go/informers"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
)

func APIServer() {
	ctx := signal.SigTermCancelContext(context.Background())
	if err := RunAPIServer(ctx, NewAPIServerOptions()); err != nil {
		panic(err)
	}
}

type Options struct {
	ListenPort int
}

func NewAPIServerOptions() *Options {
	return &Options{
		ListenPort: 8081,
	}
}

func (a *Options) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	return &user.DefaultInfo{
		Name:   "admin",
		Groups: []string{"system:masters", "system:authenticated"},
	}, true, nil
}

func (a *Options) Authorizer(next authorizer.Authorizer) authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(a authorizer.Attributes) (bool, string, error) {
		if a.GetUser() != nil && slice.ContainsString(a.GetUser().GetGroups(), "system:masters") {
			return true, "", nil
		}
		return next.Authorize(a)
	})
}

func RunAPIServer(ctx context.Context, apiServer *Options) error {
	runOptions := options.NewServerRunOptions()
	runOptions.Authentication.ServiceAccounts.Lookup = false
	runOptions.Authorization.Mode = modes.ModeRBAC
	runOptions.InsecureServing.BindPort = apiServer.ListenPort
	runOptions.SecureServing.BindPort = 0
	runOptions.Etcd.StorageConfig.ServerList = []string{"http://127.0.0.1:2379"}
	runOptions.EnableAggregatorRouting = false

	_, ipNet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		return err
	}
	runOptions.ServiceClusterIPRange = *ipNet

	kubeAPIServerConfig, sharedInformers, versionedInformers, _, _, err := app.CreateKubeAPIServerConfig(runOptions, nil, nil)
	if err != nil {
		return err
	}
	kubeAPIServerConfig.GenericConfig.Authenticator = apiServer
	kubeAPIServerConfig.GenericConfig.Authorizer = apiServer.Authorizer(kubeAPIServerConfig.GenericConfig.Authorizer)

	apiExtensionsConfig, err := createAPIExtensionsConfig(*kubeAPIServerConfig.GenericConfig, versionedInformers, runOptions)
	if err != nil {
		return err
	}

	apiExtensionsServer, err := apiExtensionsConfig.Complete().New(genericapiserver.EmptyDelegate)
	if err != nil {
		return err
	}

	kubeAPIServer, err := app.CreateKubeAPIServer(kubeAPIServerConfig, apiExtensionsServer.GenericAPIServer, sharedInformers, versionedInformers)
	if err != nil {
		return err
	}

	kubeAPIServer.GenericAPIServer.PrepareRun()

	done := make(chan error)
	server := http.Server{Addr: fmt.Sprintf("localhost:%d", apiServer.ListenPort), Handler: kubeAPIServer.GenericAPIServer.Handler}

	go func() {
		done <- server.ListenAndServe()
	}()

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	kubeAPIServer.GenericAPIServer.RunPostStartHooks(ctx.Done())

	return <-done
}

func createAPIExtensionsConfig(kubeAPIServerConfig genericapiserver.Config, externalInformers kubeexternalinformers.SharedInformerFactory, commandOptions *options.ServerRunOptions) (*apiextensionsapiserver.Config, error) {
	genericConfig := kubeAPIServerConfig
	etcdOptions := *commandOptions.Etcd
	etcdOptions.StorageConfig.Codec = apiextensionsapiserver.Codecs.LegacyCodec(v1beta1.SchemeGroupVersion)
	etcdOptions.StorageConfig.Copier = apiextensionsapiserver.Scheme
	genericConfig.RESTOptionsGetter = &genericoptions.SimpleRestOptionsFactory{Options: etcdOptions}

	apiextensionsConfig := &apiextensionsapiserver.Config{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config:                genericConfig,
			SharedInformerFactory: externalInformers,
		},
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionscmd.NewCRDRESTOptionsGetter(etcdOptions),
		},
	}

	return apiextensionsConfig, nil
}
