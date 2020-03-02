package summary

import (
	"strings"

	"github.com/rancher/wrangler/pkg/data"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Summary struct {
	State         string
	Error         bool
	Transitioning bool
	Message       []string
}

func dedupMessage(messages []string) []string {
	if len(messages) <= 1 {
		return messages
	}

	seen := map[string]bool{}
	var result []string

	for _, message := range messages {
		if seen[message] {
			continue
		}
		seen[message] = true
		result = append(result, message)
	}

	return result
}

func Summarize(unstr *unstructured.Unstructured) Summary {
	var (
		obj     data.Object
		summary Summary
	)

	if unstr != nil {
		obj = unstr.Object
	}

	conditions := getConditions(obj)

	for _, summarizer := range Summarizers {
		summary = summarizer(obj, conditions, summary)
	}

	if summary.State == "" {
		summary.State = "active"
	}

	summary.State = strings.ToLower(summary.State)
	summary.Message = dedupMessage(summary.Message)
	return summary
}
