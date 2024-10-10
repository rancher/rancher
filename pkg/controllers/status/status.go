package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
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
			found = true
		}
	}
	if !found {
		*conditions = append(*conditions, condition)
	}
}
