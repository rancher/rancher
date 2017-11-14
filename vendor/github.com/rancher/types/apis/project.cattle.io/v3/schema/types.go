package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	handlerMapper = &m.UnionEmbed{
		Fields: []m.UnionMapping{
			{
				FieldName:   "exec",
				CheckFields: []string{"command"},
			},
			{
				FieldName:   "tcpSocket",
				CheckFields: []string{"tcp", "port"},
			},
			{
				FieldName:   "httpGet",
				CheckFields: []string{"port"},
			},
		},
	}
)

type ScalingGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              interface{} `json:"spec"`
	Status            interface{} `json:"status"`
}

type handlerOverride struct {
	TCP bool
}

type EnvironmentFrom struct {
	Source     string
	SourceName string
	SourceKey  string
	Prefix     string
	Optional   bool
	TargetKey  string
}

type Resources struct {
	CPU       *ResourceRequest
	Memory    *ResourceRequest
	NvidiaGPU *ResourceRequest
}

type ResourceRequest struct {
	Request string
	Limit   string
}

type Scheduling struct {
	AntiAffinity      string
	Node              *NodeScheduling
	Tolerate          []string
	Scheduler         string
	Priority          *int64
	PriorityClassName string
}

type NodeScheduling struct {
	Name       string
	RequireAll []string
	RequireAny []string
	Preferred  []string
}

type deployOverride struct {
	v3.DeployConfig
}

type projectOverride struct {
	types.Namespaced
	ProjectID string `norman:"type=reference[/v3/schemas/project]"`
}
