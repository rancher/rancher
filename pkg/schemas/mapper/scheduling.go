package mapper

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v1 "k8s.io/api/core/v1"
)

var (
	exprRegexp = regexp.MustCompile("^(.*?)\\s*(=|!=|<|>| in | notin )\\s*(.*)$")
)

type SchedulingMapper struct {
}

func (s SchedulingMapper) FromInternal(data map[string]interface{}) {
	defer func() {
		delete(data, "nodeSelector")
		delete(data, "affinity")
	}()

	var requireAll []string
	for key, value := range convert.ToMapInterface(data["nodeSelector"]) {
		if value == "" {
			requireAll = append(requireAll, key)
		} else {
			requireAll = append(requireAll, fmt.Sprintf("%s = %s", key, value))
		}
	}

	if len(requireAll) > 0 {
		values.PutValue(data, requireAll, "scheduling", "node", "requireAll")
	}

	v, ok := data["affinity"]
	if !ok || v == nil {
		return
	}

	affinity := &v1.Affinity{}
	if err := convert.ToObj(v, affinity); err != nil {
		return
	}

	if affinity.NodeAffinity != nil {
		s.nodeAffinity(data, affinity.NodeAffinity)
	}
}

func (s SchedulingMapper) nodeAffinity(data map[string]interface{}, nodeAffinity *v1.NodeAffinity) {
	var requireAll []string
	var requireAny []string
	var preferred []string

	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, term := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			exprs := NodeSelectorTermToStrings(term)
			if len(exprs) == 0 {
				continue
			}
			if len(requireAny) > 0 {
				// Once any is set all new terms go to any
				requireAny = append(requireAny, strings.Join(exprs, " && "))
			} else if len(requireAll) > 0 {
				// If all is already set, we actually need to move everything to any
				requireAny = append(requireAny, strings.Join(requireAll, " && "))
				requireAny = append(requireAny, strings.Join(exprs, " && "))
				requireAll = []string{}
			} else {
				// The first term is considered all
				requireAll = exprs
			}
		}
	}

	if nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		sortPreferred(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
		for _, term := range nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			exprs := NodeSelectorTermToStrings(term.Preference)
			preferred = append(preferred, strings.Join(exprs, " && "))
		}
	}

	if len(requireAll) > 0 {
		values.PutValue(data, requireAll, "scheduling", "node", "requireAll")
	}
	if len(requireAny) > 0 {
		values.PutValue(data, requireAny, "scheduling", "node", "requireAny")
	}
	if len(preferred) > 0 {
		values.PutValue(data, preferred, "scheduling", "node", "preferred")
	}
}

func sortPreferred(terms []v1.PreferredSchedulingTerm) {
	sort.Slice(terms, func(i, j int) bool {
		return terms[i].Weight > terms[j].Weight
	})
}

func NodeSelectorTermToStrings(term v1.NodeSelectorTerm) []string {
	exprs := []string{}

	for _, expr := range term.MatchExpressions {
		nextExpr := ""
		switch expr.Operator {
		case v1.NodeSelectorOpIn:
			if len(expr.Values) > 1 {
				nextExpr = fmt.Sprintf("%s in (%s)", expr.Key, strings.Join(expr.Values, ", "))
			} else if len(expr.Values) == 1 {
				nextExpr = fmt.Sprintf("%s = %s", expr.Key, expr.Values[0])
			}
		case v1.NodeSelectorOpNotIn:
			if len(expr.Values) > 1 {
				nextExpr = fmt.Sprintf("%s notin (%s)", expr.Key, strings.Join(expr.Values, ", "))
			} else if len(expr.Values) == 1 {
				nextExpr = fmt.Sprintf("%s != %s", expr.Key, expr.Values[0])
			}
		case v1.NodeSelectorOpExists:
			nextExpr = expr.Key
		case v1.NodeSelectorOpDoesNotExist:
			nextExpr = "!" + expr.Key
		case v1.NodeSelectorOpGt:
			if len(expr.Values) == 1 {
				nextExpr = fmt.Sprintf("%s > %s", expr.Key, expr.Values[0])
			}
		case v1.NodeSelectorOpLt:
			if len(expr.Values) == 1 {
				nextExpr = fmt.Sprintf("%s < %s", expr.Key, expr.Values[0])
			}
		}

		if nextExpr != "" {
			exprs = append(exprs, nextExpr)
		}
	}

	return exprs
}

func StringsToNodeSelectorTerm(exprs []string) []v1.NodeSelectorTerm {
	result := []v1.NodeSelectorTerm{}

	for _, inter := range exprs {
		term := v1.NodeSelectorTerm{}

		for _, expr := range strings.Split(inter, "&&") {
			groups := exprRegexp.FindStringSubmatch(expr)
			selectorRequirement := v1.NodeSelectorRequirement{}

			if groups == nil {
				if strings.HasPrefix(expr, "!") {
					selectorRequirement.Key = strings.TrimSpace(expr[1:])
					selectorRequirement.Operator = v1.NodeSelectorOpDoesNotExist
				} else {
					selectorRequirement.Key = strings.TrimSpace(expr)
					selectorRequirement.Operator = v1.NodeSelectorOpExists
				}
			} else {
				selectorRequirement.Key = strings.TrimSpace(groups[1])
				selectorRequirement.Values = convert.ToValuesSlice(groups[3])
				op := strings.TrimSpace(groups[2])
				switch op {
				case "=":
					selectorRequirement.Operator = v1.NodeSelectorOpIn
				case "!=":
					selectorRequirement.Operator = v1.NodeSelectorOpNotIn
				case "notin":
					selectorRequirement.Operator = v1.NodeSelectorOpNotIn
				case "in":
					selectorRequirement.Operator = v1.NodeSelectorOpIn
				case "<":
					selectorRequirement.Operator = v1.NodeSelectorOpLt
				case ">":
					selectorRequirement.Operator = v1.NodeSelectorOpGt
				}
			}

			term.MatchExpressions = append(term.MatchExpressions, selectorRequirement)
		}

		result = append(result, term)
	}

	return result
}

func (s SchedulingMapper) ToInternal(data map[string]interface{}) error {
	defer func() {
		delete(data, "scheduling")
	}()

	nodeName := convert.ToString(values.GetValueN(data, "scheduling", "node", "nodeId"))
	if nodeName != "" {
		data["nodeName"] = nodeName
	}

	requireAllV := values.GetValueN(data, "scheduling", "node", "requireAll")
	requireAnyV := values.GetValueN(data, "scheduling", "node", "requireAny")
	preferredV := values.GetValueN(data, "scheduling", "node", "preferred")

	if requireAllV == nil && requireAnyV == nil && preferredV == nil {
		return nil
	}

	requireAll := convert.ToStringSlice(requireAllV)
	requireAny := convert.ToStringSlice(requireAnyV)
	preferred := convert.ToStringSlice(preferredV)

	if len(requireAll) == 0 && len(requireAny) == 0 && len(preferred) == 0 {
		values.PutValue(data, nil, "affinity", "nodeAffinity")
		return nil
	}

	nodeAffinity := v1.NodeAffinity{}

	if len(requireAll) > 0 {
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{
				AggregateTerms(StringsToNodeSelectorTerm(requireAll)),
			},
		}
	}

	if len(requireAny) > 0 {
		if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{}
		}
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = StringsToNodeSelectorTerm(requireAny)
	}

	if len(preferred) > 0 {
		count := int32(100)
		for _, term := range StringsToNodeSelectorTerm(preferred) {
			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, v1.PreferredSchedulingTerm{
					Weight:     count,
					Preference: term,
				})
			count--
		}
	}

	affinity, _ := convert.EncodeToMap(&v1.Affinity{
		NodeAffinity: &nodeAffinity,
	})

	if nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
		values.PutValue(affinity, nil, "nodeAffinity", "preferredDuringSchedulingIgnoredDuringExecution")
	}

	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		values.PutValue(affinity, nil, "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution")
	}

	data["affinity"] = affinity

	return nil
}

func AggregateTerms(terms []v1.NodeSelectorTerm) v1.NodeSelectorTerm {
	result := v1.NodeSelectorTerm{}
	for _, term := range terms {
		result.MatchExpressions = append(result.MatchExpressions, term.MatchExpressions...)
	}
	return result
}

func (s SchedulingMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	delete(schema.ResourceFields, "nodeSelector")
	delete(schema.ResourceFields, "affinity")
	return nil
}
