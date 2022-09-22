package taints

import (
	"fmt"
	"strings"

	rketypes "github.com/rancher/rke/types"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func GetTaintsString(taint v1.Taint) string {
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}

func GetRKETaintsString(taint rketypes.RKETaint) string {
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

func GetRKETaintsFromStrings(sources []string) []rketypes.RKETaint {
	rtn := make([]rketypes.RKETaint, len(sources))
	for i, source := range sources {
		t := GetTaintFromString(source)
		rtn[i] = rketypes.RKETaint{
			Key:       t.Key,
			Value:     t.Value,
			Effect:    t.Effect,
			TimeAdded: t.TimeAdded,
		}
	}
	return rtn
}

func GetStringsFromRKETaint(taints []rketypes.RKETaint) []string {
	var rtn []string
	for _, taint := range taints {
		rtn = append(rtn, GetRKETaintsString(taint))
	}
	return rtn
}

func GetRKETaintsFromTaints(sources []v1.Taint) []rketypes.RKETaint {
	rtn := make([]rketypes.RKETaint, len(sources))
	for i, source := range sources {
		rtn[i] = rketypes.RKETaint{
			Key:       source.Key,
			Effect:    source.Effect,
			Value:     source.Value,
			TimeAdded: source.TimeAdded,
		}
	}
	return rtn
}

func GetStringsFromTaint(taints []v1.Taint) []string {
	var rtn []string
	for _, taint := range taints {
		rtn = append(rtn, GetTaintsString(taint))
	}
	return rtn
}

func GetTaintsFromStrings(sources []string) []v1.Taint {
	var rtn []v1.Taint
	for _, source := range sources {
		taint := GetTaintFromString(source)
		if taint == nil {
			continue
		}
		rtn = append(rtn, *taint)
	}
	return rtn
}

// MergeTaints will override t1 taint by t2 with same key and effect
func MergeTaints(t1 []v1.Taint, t2 []v1.Taint) []v1.Taint {
	set1 := GetKeyEffectTaintSet(t1)
	set2 := GetKeyEffectTaintSet(t2)
	rtn := t2
	for key, i := range set1 {
		if j, ok := set2[key]; ok {
			logrus.Infof("overriding taint %s with %s", GetTaintsString(t1[i]), GetTaintsString(t2[j]))
			continue
		}
		rtn = append(rtn, t1[i])
	}
	return rtn
}
