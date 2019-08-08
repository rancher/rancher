package taints

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

var (
	HostOSLabels = map[string]labels.Set{
		"beta.kubernetes.io/os": labels.Set(map[string]string{
			"beta.kubernetes.io/os": "linux",
		}),
		"kubernetes.io/os": labels.Set(map[string]string{
			"kubernetes.io/os": "linux",
		}),
	}
	NodeTaint = v1.Taint{
		Key:    "cattle.io/os",
		Value:  "linux",
		Effect: v1.TaintEffectNoSchedule,
	}
)

func GetTaintsString(taint v1.Taint) string {
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}

func GetKeyEffectString(taint v1.Taint) string {
	return fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
}

func GetTaintFromString(taintStr string) *v1.Taint {
	taintStruct := strings.Split(taintStr, "=")
	if len(taintStruct) != 2 {
		logrus.Warnf("taint string %s is not validated", taintStr)
		return nil
	}
	tmp := strings.Split(taintStruct[1], ":")
	if len(tmp) != 2 {
		logrus.Warnf("taint string %s is not validated", taintStr)
		return nil
	}
	key := taintStruct[0]
	value := tmp[0]
	effect := v1.TaintEffect(tmp[1])
	return &v1.Taint{
		Key:    key,
		Value:  value,
		Effect: effect,
	}
}

func GetTaintSet(taints []v1.Taint) map[string]int {
	rtn := map[string]int{}
	for i, taint := range taints {
		rtn[GetTaintsString(taint)] = i
	}
	return rtn
}

func GetKeyEffectTaintSet(taints []v1.Taint) map[string]int {
	rtn := map[string]int{}
	for i, taint := range taints {
		rtn[GetKeyEffectString(taint)] = i
	}
	return rtn
}

func GetTaintsFromStrings(sources []string) []v1.Taint {
	var rtn []v1.Taint
	for _, taintStr := range sources {
		taint := GetTaintFromString(taintStr)
		if taint != nil {
			rtn = append(rtn, *taint)
		}
	}
	return rtn
}

func GetToDiffTaints(current, desired []v1.Taint) (toAdd map[int]v1.Taint, toDel map[int]v1.Taint) {
	toAdd, toDel = map[int]v1.Taint{}, map[int]v1.Taint{}
	currentSet := GetTaintSet(current)
	desiredSet := GetTaintSet(desired)
	for k, index := range currentSet {
		if _, ok := desiredSet[k]; !ok {
			toDel[index] = current[index]
		}
	}
	for k, index := range desiredSet {
		if _, ok := currentSet[k]; !ok {
			toAdd[index] = desired[index]
		}
	}
	return toAdd, toDel
}

func GetRKETaintsFromStrings(sources []string) []v3.RKETaint {
	rtn := make([]v3.RKETaint, len(sources))
	for i, source := range sources {
		t := GetTaintFromString(source)
		rtn[i] = v3.RKETaint{
			Key:       t.Key,
			Value:     t.Value,
			Effect:    t.Effect,
			TimeAdded: t.TimeAdded,
		}
	}
	return rtn
}

func GetSelectorByNodeSelectorTerms(terms []v1.NodeSelectorTerm) map[string][]labels.Selector {
	rtn := map[string][]labels.Selector{}
	for _, term := range terms {
		for _, req := range term.MatchExpressions {
			selector, err := helper.NodeSelectorRequirementsAsSelector([]v1.NodeSelectorRequirement{req})
			if err != nil {
				logrus.Warnf("failed to create selector from workload node affinity, error: %s", err.Error())
				continue
			}
			rtn[req.Key] = append(rtn[req.Key], selector)
		}
	}
	return rtn
}

func CanDeployToLinuxHost(selectorMap map[string][]labels.Selector) bool {
	for key, selectors := range selectorMap {
		hostOSLabel, ok := HostOSLabels[key]
		if !ok || len(selectors) == 0 {
			continue
		}
		result := true
		// all the selector with same key should match our host labels
		for _, selector := range selectors {
			result = result && selector.Matches(hostOSLabel)
		}
		if result {
			return true
		}
	}
	return false
}

func GetTolerationByTaint(taint v1.Taint) v1.Toleration {
	return v1.Toleration{
		Key:      taint.Key,
		Value:    taint.Value,
		Operator: v1.TolerationOpEqual,
		Effect:   taint.Effect,
	}
}
