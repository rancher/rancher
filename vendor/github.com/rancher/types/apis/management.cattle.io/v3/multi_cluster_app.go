package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	MultiClusterAppConditionInstalled condition.Cond = "Installed"
	MultiClusterAppConditionDeployed  condition.Cond = "Deployed"
)

type MultiClusterApp struct {
	types.Namespaced
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status

	Spec   MultiClusterAppSpec   `json:"spec"`
	Status MultiClusterAppStatus `json:"status"`
}

type MultiClusterAppSpec struct {
	TemplateVersionName  string          `json:"templateVersionName,omitempty" norman:"type=reference[templateVersion],required"`
	Answers              []Answer        `json:"answers,omitempty"`
	Wait                 bool            `json:"wait,omitempty"`
	Timeout              int             `json:"timeout,omitempty" norman:"min=1,default=300"`
	Targets              []Target        `json:"targets,omitempty" norman:"required,noupdate"`
	Members              []Member        `json:"members,omitempty"`
	Roles                []string        `json:"roles,omitempty" norman:"type=array[reference[roleTemplate]],required"`
	RevisionHistoryLimit int             `json:"revisionHistoryLimit,omitempty" norman:"default=10"`
	UpgradeStrategy      UpgradeStrategy `json:"upgradeStrategy,omitempty"`
}

type MultiClusterAppStatus struct {
	Conditions   []v3.AppCondition `json:"conditions,omitempty"`
	RevisionName string            `json:"revisionName,omitempty" norman:"type=reference[multiClusterAppRevision],required"`
	HelmVersion  string            `json:"helmVersion,omitempty" norman:"nocreate,noupdate"`
}

type Target struct {
	ProjectName string `json:"projectName,omitempty" norman:"type=reference[project],required"`
	AppName     string `json:"appName,omitempty" norman:"type=reference[v3/projects/schemas/app]"`
	State       string `json:"state,omitempty"`
	Healthstate string `json:"healthState,omitempty"`
}

func (t *Target) ObjClusterName() string {
	if parts := strings.SplitN(t.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type Answer struct {
	ProjectName string            `json:"projectName,omitempty" norman:"type=reference[project]"`
	ClusterName string            `json:"clusterName,omitempty" norman:"type=reference[cluster]"`
	Values      map[string]string `json:"values,omitempty" norman:"required"`
}

func (a *Answer) ObjClusterName() string {
	return a.ClusterName
}

type Member struct {
	UserName           string `json:"userName,omitempty" norman:"type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"type=reference[principal]"`
	DisplayName        string `json:"displayName,omitempty"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"type=reference[principal]"`
	AccessType         string `json:"accessType,omitempty" norman:"type=enum,options=owner|member|read-only"`
}

type UpgradeStrategy struct {
	RollingUpdate *RollingUpdate `json:"rollingUpdate,omitempty"`
}

type RollingUpdate struct {
	BatchSize int `json:"batchSize,omitempty"`
	Interval  int `json:"interval,omitempty"`
}

type MultiClusterAppRevision struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	TemplateVersionName string   `json:"templateVersionName,omitempty" norman:"type=reference[templateVersion]"`
	Answers             []Answer `json:"answers,omitempty"`
}

type MultiClusterAppRollbackInput struct {
	RevisionName string `json:"revisionName,omitempty" norman:"type=reference[multiClusterAppRevision]"`
}

type UpdateMultiClusterAppTargetsInput struct {
	Projects []string `json:"projects" norman:"type=array[reference[project]],required"`
	Answers  []Answer `json:"answers"`
}
