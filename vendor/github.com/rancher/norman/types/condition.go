package types

import (
	"github.com/rancher/norman/types/convert"
)

var (
	CondEQ      = QueryConditionType{ModifierEQ, 1}
	CondNE      = QueryConditionType{ModifierNE, 1}
	CondNull    = QueryConditionType{ModifierNull, 0}
	CondNotNull = QueryConditionType{ModifierNotNull, 0}
	CondIn      = QueryConditionType{ModifierIn, -1}
	CondNotIn   = QueryConditionType{ModifierNotIn, -1}
	CondOr      = QueryConditionType{ModifierType("or"), 1}
	CondAnd     = QueryConditionType{ModifierType("and"), 1}

	mods = map[ModifierType]QueryConditionType{
		CondEQ.Name:      CondEQ,
		CondNE.Name:      CondNE,
		CondNull.Name:    CondNull,
		CondNotNull.Name: CondNotNull,
		CondIn.Name:      CondIn,
		CondNotIn.Name:   CondNotIn,
		CondOr.Name:      CondOr,
		CondAnd.Name:     CondAnd,
	}
)

type QueryConditionType struct {
	Name ModifierType
	Args int
}

type QueryCondition struct {
	Field         string
	Value         string
	Values        map[string]bool
	conditionType QueryConditionType
	left, right   *QueryCondition
}

func (q *QueryCondition) Valid(data map[string]interface{}) bool {
	switch q.conditionType {
	case CondAnd:
		if q.left == nil || q.right == nil {
			return false
		}
		return q.left.Valid(data) && q.right.Valid(data)
	case CondOr:
		if q.left == nil || q.right == nil {
			return false
		}
		return q.left.Valid(data) || q.right.Valid(data)
	case CondEQ:
		return q.Value == convert.ToString(data[q.Field])
	case CondNE:
		return q.Value != convert.ToString(data[q.Field])
	case CondIn:
		return q.Values[convert.ToString(data[q.Field])]
	case CondNotIn:
		return !q.Values[convert.ToString(data[q.Field])]
	case CondNotNull:
		return convert.ToString(data[q.Field]) != ""
	case CondNull:
		return convert.ToString(data[q.Field]) == ""
	}

	return false
}

func (q *QueryCondition) ToCondition() Condition {
	cond := Condition{
		Modifier: q.conditionType.Name,
	}
	if q.conditionType.Args == 1 {
		cond.Value = q.Value
	} else if q.conditionType.Args == -1 {
		stringValues := []string{}
		for val := range q.Values {
			stringValues = append(stringValues, val)
		}
		cond.Value = stringValues
	}

	return cond
}

func ValidMod(mod ModifierType) bool {
	_, ok := mods[mod]
	return ok
}

func NewConditionFromString(field string, mod ModifierType, values ...string) *QueryCondition {
	q := &QueryCondition{
		Field:         field,
		conditionType: mods[mod],
		Values:        map[string]bool{},
	}

	for i, value := range values {
		if i == 0 {
			q.Value = value
		}
		q.Values[value] = true
	}

	return q
}
