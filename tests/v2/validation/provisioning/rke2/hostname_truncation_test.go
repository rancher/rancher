package rke2

import (
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HostnameTruncationTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	kubernetesVersions []string
	cnis               []string
	providers          []string
	advancedOptions    provisioning.AdvancedOptions
}

func (r *HostnameTruncationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *HostnameTruncationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client
}

func (r *HostnameTruncationTestSuite) TestProvisioningRKE2Cluster() {
	tests := []struct {
		name                        string
		machinePools                []provv1.RKEMachinePool
		defaultHostnameLengthLimits []int
	}{
		{
			name: "Cluster level truncation",
			machinePools: []provv1.RKEMachinePool{
				{
					Name: namegen.RandStringLower(1),
				},
				{
					Name: namegen.RandStringLower(31),
				},
				{
					Name: namegen.RandStringLower(63),
				},
			},
			defaultHostnameLengthLimits: []int{0, 10, 31, 63},
		},
		{
			name: "Machine pool level truncation - 10 characters",
			machinePools: []provv1.RKEMachinePool{
				{
					Name:                namegen.RandStringLower(1),
					HostnameLengthLimit: 10,
				},
				{
					Name:                namegen.RandStringLower(31),
					HostnameLengthLimit: 10,
				},
				{
					Name:                namegen.RandStringLower(63),
					HostnameLengthLimit: 10,
				},
			},
			defaultHostnameLengthLimits: []int{10, 16, 63},
		},
		{
			name: "Machine pool level truncation - 31 characters",
			machinePools: []provv1.RKEMachinePool{
				{
					Name:                namegen.RandStringLower(1),
					HostnameLengthLimit: 31,
				},
				{
					Name:                namegen.RandStringLower(31),
					HostnameLengthLimit: 31,
				},
				{
					Name:                namegen.RandStringLower(63),
					HostnameLengthLimit: 31,
				},
			},
			defaultHostnameLengthLimits: []int{10, 31, 63},
		},
		{
			name: "Machine pool level truncation - 63 characters",
			machinePools: []provv1.RKEMachinePool{
				{
					Name:                namegen.RandStringLower(1),
					HostnameLengthLimit: 63,
				},
				{
					Name:                namegen.RandStringLower(31),
					HostnameLengthLimit: 63,
				},
				{
					Name:                namegen.RandStringLower(63),
					HostnameLengthLimit: 63,
				},
			},
			defaultHostnameLengthLimits: []int{10, 31, 63},
		},
		{
			name: "Cluster and machine pool level truncation - 31 characters",
			machinePools: []provv1.RKEMachinePool{
				{
					Name:                namegen.RandStringLower(1),
					HostnameLengthLimit: 31,
				},
				{
					Name: namegen.RandStringLower(31),
				},
				{
					Name:                namegen.RandStringLower(63),
					HostnameLengthLimit: 31,
				},
			},
			defaultHostnameLengthLimits: []int{10, 31, 63},
		},
	}

	var name string
	for _, tt := range tests {
		for _, providerName := range r.providers {
			provider := CreateProvider(providerName)
			providerName := " Node Provider: " + provider.Name.String()
			for _, kubeVersion := range r.kubernetesVersions {
				name = tt.name + providerName + " Kubernetes version: " + kubeVersion
				for _, cni := range r.cnis {
					name += " cni: " + cni
					r.Run(name, func() {
						for _, limit := range tt.defaultHostnameLengthLimits {
							subSession := r.session.NewSession()

							client, err := r.client.WithSession(subSession)
							require.NoError(r.T(), err)

							TestHostnameTruncation(r.T(), client, provider, tt.machinePools, limit, kubeVersion, cni, r.advancedOptions)

							subSession.Cleanup()
						}
					})
				}
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHostnameTruncationTestSuite(t *testing.T) {
	suite.Run(t, new(HostnameTruncationTestSuite))
}
