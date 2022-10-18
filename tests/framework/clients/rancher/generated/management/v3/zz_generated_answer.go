package client

const (
	AnswerType                 = "answer"
	AnswerFieldClusterID       = "clusterId"
	AnswerFieldProjectID       = "projectId"
	AnswerFieldValues          = "values"
	AnswerFieldValuesSetString = "valuesSetString"
)

type Answer struct {
	ClusterID       string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	ProjectID       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Values          map[string]string `json:"values,omitempty" yaml:"values,omitempty"`
	ValuesSetString map[string]string `json:"valuesSetString,omitempty" yaml:"valuesSetString,omitempty"`
}
