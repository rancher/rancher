package client


	


import (
	
)

const (
    ClusterTemplateQuestionsOutputType = "clusterTemplateQuestionsOutput"
	ClusterTemplateQuestionsOutputFieldQuestions = "questions"
)

type ClusterTemplateQuestionsOutput struct {
        Questions []Question `json:"questions,omitempty" yaml:"questions,omitempty"`
}

