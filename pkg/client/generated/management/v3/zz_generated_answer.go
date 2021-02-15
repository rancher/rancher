package client

const (
	AnswerType                   = "answer"
	AnswerFieldClusterID         = "clusterId"
	AnswerFieldProjectID         = "projectId"
	AnswerFieldValues            = "values"
	AnswerFieldValuesForceString = "valuesForceString"
)

type Answer struct {
	ClusterID         string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ProjectID         string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Values            map[string]string `json:"values,omitempty" yaml:"values,omitempty"`
	ValuesForceString map[string]bool   `json:"valuesForceString,omitempty" yaml:"valuesForceString,omitempty"`
}
