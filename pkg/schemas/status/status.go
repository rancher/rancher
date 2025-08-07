package status

import (
	"strings"

	"github.com/rancher/wrangler/v3/pkg/summary"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Set(data map[string]interface{}) {
	if data == nil {
		return
	}
	summary := summary.SummarizeWithOptions(&unstructured.Unstructured{Object: data}, nil)
	data["state"] = summary.State
	data["transitioning"] = "no"
	if summary.Error {
		data["transitioning"] = "error"
	} else if summary.Transitioning {
		data["transitioning"] = "yes"
	}
	data["transitioningMessage"] = strings.Join(summary.Message, "; ")
}
