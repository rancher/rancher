package management

import (
	"context"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	awsProxyEndpoint = v3.ProxyEndpoint{
		ObjectMeta: v1.ObjectMeta{
			Name: "rancher-aws-endpoints",
		},
		Spec: v3.ProxyEndpointSpec{
			Routes: []v3.ProxyEndpointRoute{
				// https://docs.aws.amazon.com/general/latest/gr/iam-service.html
				{Domain: "iam.amazonaws.com"},
				{Domain: "iam.us-gov.amazonaws.com"},
				{Domain: "iam.%.amazonaws.com.cn"},
				// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_dual-stack_endpoint_support.html
				{Domain: "iam.global.api.aws"},
				// https://docs.aws.amazon.com/general/latest/gr/ec2-service.html
				{Domain: "ec2.%.amazonaws.com"},
				{Domain: "ec2.%.amazonaws.com.cn"},
				{Domain: "ec2.%.api.aws"},
				// https://docs.aws.amazon.com/general/latest/gr/eks.html
				{Domain: "eks.%.amazonaws.com"},
				{Domain: "eks.%.amazonaws.com.cn"},
				{Domain: "eks.%.api.aws"},
				// https://docs.aws.amazon.com/general/latest/gr/kms.html
				{Domain: "kms.%.amazonaws.com"},
				{Domain: "kms.%.amazonaws.com.cn"},
				{Domain: "kms.%.api.aws"},
			},
		},
	}
	digitalOceanProxyEndpoint = v3.ProxyEndpoint{
		ObjectMeta: v1.ObjectMeta{
			Name: "rancher-digitalocean-endpoints",
		},
		Spec: v3.ProxyEndpointSpec{
			Routes: []v3.ProxyEndpointRoute{
				{Domain: "api.digitalocean.com"},
			},
		},
	}
	linodeProxyEndpoint = v3.ProxyEndpoint{
		ObjectMeta: v1.ObjectMeta{
			Name: "rancher-linode-endpoints",
		},
		Spec: v3.ProxyEndpointSpec{
			Routes: []v3.ProxyEndpointRoute{
				{Domain: "api.linode.com"},
			},
		},
	}
)

type handler struct {
	clients *wrangler.Context
}

func ManageProxyEndpointData(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clients: clients,
	}
	clients.Mgmt.Setting().OnChange(ctx, "handle-builtin-proxy-endpoints", h.manageDefaultProxyEndpoints)
}

func (h *handler) manageDefaultProxyEndpoints(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || setting.Name != "disable-default-proxy-endpoint" {
		return setting, nil
	}
	err := AddProxyEndpointData(setting.Value, h.clients)
	return setting, err
}

// AddProxyEndpointData adds default ProxyEndpoint resources unless they are disabled via the DisableDefaultProxyEndpoint setting.
func AddProxyEndpointData(disabledEndpointsSetting string, clients *wrangler.Context) error {
	disabledEndpoints := strings.Split(disabledEndpointsSetting, ",")
	var endpointsDisabled []string
	for _, endpoint := range disabledEndpoints {
		if trimmed := strings.TrimSpace(endpoint); trimmed != "" {
			endpointsDisabled = append(endpointsDisabled, trimmed)
		}
	}

	disableAll := slices.Contains(endpointsDisabled, "all")

	builtInEndpoints := []struct {
		name     string
		endpoint v3.ProxyEndpoint
	}{
		{name: "rancher-aws-endpoints", endpoint: awsProxyEndpoint},
		{name: "rancher-digitalocean-endpoints", endpoint: digitalOceanProxyEndpoint},
		{name: "rancher-linode-endpoints", endpoint: linodeProxyEndpoint},
	}
	for _, endpoint := range builtInEndpoints {
		err := createOrDisableEndpoint(endpoint.endpoint, slices.Contains(endpointsDisabled, endpoint.name) || disableAll, clients)
		if err != nil {
			return err
		}
	}
	return nil
}

func endpointExists(name string, clients *wrangler.Context) (bool, error) {
	_, err := clients.Mgmt.ProxyEndpoint().Get(name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createOrDisableEndpoint(endpoint v3.ProxyEndpoint, disabled bool, clients *wrangler.Context) error {
	exists, err := endpointExists(endpoint.Name, clients)
	if err != nil {
		return err
	}
	if !disabled && !exists {
		_, err = clients.Mgmt.ProxyEndpoint().Create(&endpoint)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	if disabled && exists {
		err = clients.Mgmt.ProxyEndpoint().Delete(endpoint.Name, &v1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
