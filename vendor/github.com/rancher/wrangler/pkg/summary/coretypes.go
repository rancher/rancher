package summary

import (
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func checkPodSelector(obj data.Object, condition []Condition, summary Summary) Summary {
	selector := obj.Map("spec", "selector")
	if selector == nil {
		return summary
	}

	if !isKind(obj, "ReplicaSet", "apps/", "extension/") &&
		!isKind(obj, "DaemonSet", "apps/", "extension/") &&
		!isKind(obj, "StatefulSet", "apps/", "extension/") &&
		!isKind(obj, "Deployment", "apps/", "extension/") &&
		!isKind(obj, "Job", "batch/") &&
		!isKind(obj, "Service") {
		return summary
	}

	_, hasMatch := selector["matchLabels"]
	if !hasMatch {
		_, hasMatch = selector["matchExpressions"]
	}
	sel := metav1.LabelSelector{}
	if hasMatch {
		if err := convert.ToObj(selector, &sel); err != nil {
			return summary
		}
	} else {
		sel.MatchLabels = map[string]string{}
		for k, v := range selector {
			sel.MatchLabels[k] = convert.ToString(v)
		}
	}

	t := "creates"
	if obj["kind"] == "Service" {
		t = "selects"
	}

	summary.Relationships = append(summary.Relationships, Relationship{
		Kind:       "Pod",
		APIVersion: "v1",
		Type:       t,
		Selector:   &sel,
	})
	return summary
}

func checkPod(obj data.Object, condition []Condition, summary Summary) Summary {
	if !isKind(obj, "Pod") {
		return summary
	}
	if obj.String("kind") != "Pod" || obj.String("apiVersion") != "v1" {
		return summary
	}
	summary = checkPodConfigMaps(obj, condition, summary)
	summary = checkPodSecrets(obj, condition, summary)
	summary = checkPodServiceAccount(obj, condition, summary)
	summary = checkPodProjectedVolume(obj, condition, summary)
	summary = checkPodPullSecret(obj, condition, summary)
	return summary
}

func checkPodPullSecret(obj data.Object, condition []Condition, summary Summary) Summary {
	for _, pullSecret := range obj.Slice("imagePullSecrets") {
		if name := pullSecret.String("name"); name != "" {
			summary.Relationships = append(summary.Relationships, Relationship{
				Name:       name,
				Kind:       "Secret",
				APIVersion: "v1",
				Type:       "uses",
			})
		}
	}
	return summary
}

func checkPodProjectedVolume(obj data.Object, condition []Condition, summary Summary) Summary {
	for _, vol := range obj.Slice("spec", "volumes") {
		for _, source := range vol.Slice("projected", "sources") {
			if secretName := source.String("secret", "name"); secretName != "" {
				summary.Relationships = append(summary.Relationships, Relationship{
					Name:       secretName,
					Kind:       "Secret",
					APIVersion: "v1",
					Type:       "uses",
				})
			}
			if configMap := source.String("configMap", "name"); configMap != "" {
				summary.Relationships = append(summary.Relationships, Relationship{
					Name:       configMap,
					Kind:       "Secret",
					APIVersion: "v1",
					Type:       "uses",
				})
			}
		}
	}
	return summary
}

func checkPodConfigMaps(obj data.Object, condition []Condition, summary Summary) Summary {
	names := map[string]bool{}
	for _, vol := range obj.Slice("spec", "volumes") {
		name := vol.String("configMap", "name")
		if name == "" || names[name] {
			continue
		}
		names[name] = true
		summary.Relationships = append(summary.Relationships, Relationship{
			Name:       name,
			Kind:       "ConfigMap",
			APIVersion: "v1",
			Type:       "uses",
		})
	}
	return summary
}

func checkPodSecrets(obj data.Object, condition []Condition, summary Summary) Summary {
	names := map[string]bool{}
	for _, vol := range obj.Slice("spec", "volumes") {
		name := vol.String("secret", "secretName")
		if name == "" || names[name] {
			continue
		}
		names[name] = true
		summary.Relationships = append(summary.Relationships, Relationship{
			Name:       name,
			Kind:       "Secret",
			APIVersion: "v1",
			Type:       "uses",
		})
	}
	return summary
}

func checkPodServiceAccount(obj data.Object, condition []Condition, summary Summary) Summary {
	saName := obj.String("spec", "serviceAccountName")
	summary.Relationships = append(summary.Relationships, Relationship{
		Name:       saName,
		Kind:       "ServiceAccount",
		APIVersion: "v1",
		Type:       "uses",
	})
	return summary

}
