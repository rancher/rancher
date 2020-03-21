package summary

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/wrangler/pkg/data"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Summary struct {
	State         string
	Error         bool
	Transitioning bool
	Message       []string
}

func (s Summary) String() string {
	if !s.Transitioning && !s.Error {
		return s.State
	}
	var msg string
	if s.Transitioning {
		msg = "[progressing"
	}
	if s.Error {
		if len(msg) > 0 {
			msg += ",error]"
		} else {
			msg = "error]"
		}
	} else {
		msg += "]"
	}
	if len(s.Message) > 0 {
		msg = msg + " " + s.Message[0]
	}
	return msg
}

func (s Summary) IsReady() bool {
	return !s.Error && !s.Transitioning
}

func (s *Summary) DeepCopy() *Summary {
	v := *s
	return &v
}

func (s *Summary) DeepCopyInto(v *Summary) {
	*v = *s
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

func Summarize(runtimeObj runtime.Object) Summary {
	var (
		obj     data.Object
		summary Summary
	)

	if s, ok := runtimeObj.(*SummarizedObject); ok {
		return s.Summary
	}

	unstr, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return summary
	}

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
