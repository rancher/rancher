package cluster

import (
	"cmp"
	"math"
	"slices"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
)

var Conditions = []string{
	string(apimgmtv3.ClusterConditionProvisioned),
	string(apimgmtv3.ClusterConditionPreBootstrapped),
	string(apimgmtv3.ClusterConditionConnected),
	string(apimgmtv3.ClusterConditionAgentDeployed),
	string(apimgmtv3.ClusterConditionUpgraded),
}

func SummarizeCondition(cluster *apimgmtv3.Cluster) {
	if cluster == nil {
		return
	}

	clusterConditions := slices.Clone(cluster.Status.Conditions)
	if len(clusterConditions) == 0 {
		return
	}

	// build list of messages, making sure to truncate if over
	// for positive conditions that are false, we need to set false
	// for negative conditions that are true, we also need to set false
	// if all conditions are positive (good state), then we need to check our unknown conditions
	// if priority is important: surface all error messages. if no error messages, surface unknown (progressing).
	// if only green, mark cluster as Ready

	status := corev1.ConditionTrue
	for _, condition := range clusterConditions {
		if condition.Status == corev1.ConditionFalse {
			status = corev1.ConditionFalse
			break
		}
		if condition.Status == corev1.ConditionUnknown {
			status = corev1.ConditionUnknown
		}
	}

	var existingReady *apimgmtv3.ClusterCondition
	cons := []apimgmtv3.ClusterCondition{}
	for _, condition := range clusterConditions {
		if condition.Type == "Ready" {
			existingReady = &condition
			continue
		}
		if condition.Status == status {
			cons = append(cons, condition)
		}
	}

	order := make(map[string]int, len(Conditions))
	for rank, name := range Conditions {
		order[name] = rank
	}

	slices.SortFunc(cons, func(a, b apimgmtv3.ClusterCondition) int {
		r1, ok1 := order[string(a.Type)]
		r2, ok2 := order[string(b.Type)]

		if !ok1 {
			r1 = math.MaxInt
		}
		if !ok2 {
			r2 = math.MaxInt
		}

		if r1 != r2 {
			return cmp.Compare(r1, r2)
		}

		return cmp.Compare(string(a.Type), string(b.Type))
	})

	var messages []string
	if status != corev1.ConditionTrue {
		for _, condition := range cons {
			messages = append(messages, condition.Message)
		}
	}

	reason := "ClusterReady"
	if status == corev1.ConditionUnknown {
		reason = "ClusterProgressing"
	} else if status == corev1.ConditionFalse {
		reason = "ClusterNotReady"
	}

	ready := apimgmtv3.ClusterCondition{
		Type:    "Ready",
		Message: strings.Join(messages, "; "),
		Status:  status,
		Reason:  reason,
	}

	if existingReady != nil {
		if ready.Reason == existingReady.Reason &&
			ready.Message == existingReady.Message &&
			ready.Status == existingReady.Status {
			return
		}
	}

	apimgmtv3.ClusterConditionReady.SetStatus(cluster, string(ready.Status))
	apimgmtv3.ClusterConditionReady.Message(cluster, ready.Message)
	apimgmtv3.ClusterConditionReady.Reason(cluster, ready.Reason)
}
