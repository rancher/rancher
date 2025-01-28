package status

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SummaryCompleted = "Completed"
	SummaryError     = "Error"
)

type Status struct {
	TimeNow func() time.Time
}

func NewStatus() *Status {
	return &Status{
		TimeNow: time.Now,
	}
}

// AddCondition add condition to the conditions slice. Condition will be set to false if there is an error.
func (s *Status) AddCondition(conditions *[]metav1.Condition, condition metav1.Condition, reason string, err error) {
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = err.Error()
	} else {
		condition.Status = metav1.ConditionTrue
	}
	condition.Reason = reason
	condition.LastTransitionTime = metav1.Time{Time: s.TimeNow()}

	found := false
	for i := 0; i < len(*conditions); i++ {
		c := &(*conditions)[i]
		if condition.Type == c.Type {
			c.Status = condition.Status
			c.Reason = condition.Reason
			c.Message = condition.Message
			c.LastTransitionTime = metav1.Time{Time: s.TimeNow()}
			found = true
		}
	}
	if !found {
		*conditions = append(*conditions, condition)
	}
}

// CompareConditions compares two slices of conditions excluding the LastTransitionTime
func CompareConditions(s1 []metav1.Condition, s2 []metav1.Condition) bool {
	if len(s1) != len(s2) {
		return false
	}
	for _, c1 := range s1 {
		found := false
		for _, c2 := range s2 {
			if c1.Type == c2.Type &&
				c1.Status == c2.Status &&
				c1.Reason == c2.Reason &&
				c1.Message == c2.Message {
				found = true
				continue
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// KeepLastTransitionTimeIfConditionHasNotChanged update conditions LastTransitionTime with the value from conditionsFromCluster
// if condition is the same
func KeepLastTransitionTimeIfConditionHasNotChanged(conditions []metav1.Condition, conditionsFromCluster []metav1.Condition) {
	for _, cFromCluster := range conditionsFromCluster {
		for i := 0; i < len(conditions); i++ {
			c := &conditions[i]
			if c.Type == cFromCluster.Type &&
				c.Status == cFromCluster.Status &&
				c.Reason == cFromCluster.Reason &&
				c.Message == cFromCluster.Message {
				c.LastTransitionTime = cFromCluster.LastTransitionTime
			}
		}
	}
}
