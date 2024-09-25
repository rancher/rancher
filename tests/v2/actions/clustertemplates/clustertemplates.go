package clustertemplates

import (
	"context"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	mgmt "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	CniCalico                         = "calico"
	ClusterEnforcementSetting         = "cluster-template-enforcement"
	EnabledClusterEnforcementSetting  = "true"
	DisabledClusterEnforcementSetting = "false"
	IsRequiredQuestion                = true
	UserPrincipalID                   = "local://"
)

// NewRKE1ClusterTemplateRevisionTemplate is a constructor that creates and returns config required to create cluster template revisions
func NewRKE1ClusterTemplateRevisionTemplate(templateRevisionConfig mgmt.ClusterTemplateRevision, templateId string) mgmt.ClusterTemplateRevision {
	clusterConfig := mgmt.ClusterSpecBase{
		RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
			Version: templateRevisionConfig.ClusterConfig.RancherKubernetesEngineConfig.Version,
			Network: &mgmt.NetworkConfig{
				Plugin: templateRevisionConfig.ClusterConfig.RancherKubernetesEngineConfig.Network.Plugin,
			},
		},
	}

	var rkeTemplateConfig = mgmt.ClusterTemplateRevision{
		Name:              namegen.AppendRandomString("rketemplate-revision-"),
		ClusterTemplateID: templateId,
		ClusterConfig:     &clusterConfig,
		Questions:         templateRevisionConfig.Questions,
	}
	return rkeTemplateConfig
}

// CreateRkeTemplate is a helper that creates an rke1 template in the rancher server
func CreateRkeTemplate(client *rancher.Client, members []mgmt.Member) (*mgmt.ClusterTemplate, error) {
	rkeTemplateName := mgmt.ClusterTemplate{
		Name:    namegen.AppendRandomString("rketemplate-"),
		Members: members,
	}
	createTemplate, err := client.Management.ClusterTemplate.Create(&rkeTemplateName)

	return createTemplate, err
}

// CreateRkeTemplateRevision is a helper that takes an rke1 template revision config and create an rke1 template revision config.
func CreateRkeTemplateRevision(client *rancher.Client, templateRevisionConfig mgmt.ClusterTemplateRevision, templateId string) (*mgmt.ClusterTemplateRevision, error) {
	rkeTemplateConfig := NewRKE1ClusterTemplateRevisionTemplate(templateRevisionConfig, templateId)

	clusterTemplateRevision, err := client.Management.ClusterTemplateRevision.Create(&rkeTemplateConfig)
	if err != nil {
		return nil, err
	}
	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterTemplateRevision, err := client.Management.ClusterTemplateRevision.ByID(clusterTemplateRevision.ID)
		if err != nil {
			return false, err
		}

		return clusterTemplateRevision.State == "active", nil
	})
	if err != nil {
		return nil, err
	}

	return clusterTemplateRevision, nil
}
