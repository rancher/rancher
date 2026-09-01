package management

import (
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
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

func endpointExists(name string, clients *wrangler.Context) (*v3.ProxyEndpoint, bool, error) {
	endpoint, err := clients.Mgmt.ProxyEndpoint().Cache().Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return endpoint, false, nil
		}
		return endpoint, false, err
	}
	return endpoint, true, nil
}

func createOrDisableEndpoint(endpoint v3.ProxyEndpoint, disabled bool, clients *wrangler.Context) error {
	_, exists, err := endpointExists(endpoint.Name, clients)
	if err != nil {
		return err
	}
	if !disabled && !exists {
		_, err = clients.Mgmt.ProxyEndpoint().Create(&endpoint)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	if disabled && exists {
		err = clients.Mgmt.ProxyEndpoint().Delete(endpoint.Name, &v1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if disabled {
		return nil
	}

	// It exists and is not disabled, so ensure it has the current routes.
	//
	// This has to tolerate a conflict. AddProxyEndpointData is reached from two places at once:
	// the startup seeding path, where any error is fatal (pkg/multiclustermanager/app.go), and
	// the proxysettings controller, which re-runs it on every Setting change
	// (pkg/controllers/managementapi/whitelistproxy/proxysettings). Settings churn heavily during
	// startup, so the two writers race on the same object and Rancher used to die with
	// "Operation cannot be fulfilled on proxyendpoints.management.cattle.io".
	//
	// The read has to come from the live client rather than the cache: on a conflict the cache
	// still holds the copy that lost, so retrying against it would conflict again every time.
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := clients.Mgmt.ProxyEndpoint().Get(endpoint.Name, v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Deleted underneath us, e.g. it was disabled concurrently. Nothing to update.
				return nil
			}
			return err
		}

		if slices.Equal(current.Spec.Routes, endpoint.Spec.Routes) {
			// Already correct. Skipping the write is what keeps the two writers from
			// contending in the first place.
			return nil
		}

		current = current.DeepCopy()
		current.Spec.Routes = endpoint.Spec.Routes
		_, err = clients.Mgmt.ProxyEndpoint().Update(current)
		return err
	})
}
