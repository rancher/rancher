package monitoring

import (
	"errors"
	"regexp"
	"time"

	ncondition "github.com/rancher/norman/condition"
	"github.com/rancher/norman/controller"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	ConditionMetricExpressionDeployed = condition(mgmtv3.MonitoringConditionMetricExpressionDeployed)
)

var temfileRegexp = regexp.MustCompile("/tmp/[-_a-zA-Z0-9]+")

type condition ncondition.Cond

func (c condition) Del(obj *mgmtv3.MonitoringStatus) {
	condIdx := findCond(obj, mgmtv3.ClusterConditionType(c))
	if condIdx == nil {
		return
	}

	obj.Conditions = append(obj.Conditions[:*condIdx], obj.Conditions[*condIdx+1:]...)
}

func (c condition) Add(obj *mgmtv3.MonitoringStatus) {
	condIdx := findCond(obj, mgmtv3.ClusterConditionType(c))
	if condIdx != nil {
		return
	}

	obj.Conditions = append(obj.Conditions, mgmtv3.MonitoringCondition{
		Type:   mgmtv3.ClusterConditionType(c),
		Status: corev1.ConditionFalse,
	})
}

func (c condition) True(obj *mgmtv3.MonitoringStatus) {
	setStatus(obj, mgmtv3.ClusterConditionType(c), corev1.ConditionTrue)
}

func (c condition) IsTrue(obj *mgmtv3.MonitoringStatus) bool {
	return getStatus(obj, mgmtv3.ClusterConditionType(c)) == corev1.ConditionTrue
}

func (c condition) LastUpdated(obj *mgmtv3.MonitoringStatus, ts string) {
	setTS(obj, mgmtv3.ClusterConditionType(c), ts)
}

func (c condition) GetLastUpdated(obj *mgmtv3.MonitoringStatus) string {
	return getTS(obj, mgmtv3.ClusterConditionType(c))
}

func (c condition) False(obj *mgmtv3.MonitoringStatus) {
	setStatus(obj, mgmtv3.ClusterConditionType(c), corev1.ConditionFalse)
}

func (c condition) IsFalse(obj *mgmtv3.MonitoringStatus) bool {
	return getStatus(obj, mgmtv3.ClusterConditionType(c)) == corev1.ConditionFalse
}

func (c condition) GetStatus(obj *mgmtv3.MonitoringStatus) corev1.ConditionStatus {
	return getStatus(obj, mgmtv3.ClusterConditionType(c))
}

func (c condition) Unknown(obj *mgmtv3.MonitoringStatus) {
	setStatus(obj, mgmtv3.ClusterConditionType(c), corev1.ConditionUnknown)
}

func (c condition) CreateUnknownIfNotExists(obj *mgmtv3.MonitoringStatus) {
	cond := findCond(obj, mgmtv3.ClusterConditionType(c))
	if cond == nil {
		c.Unknown(obj)
	}
}

func (c condition) IsUnknown(obj *mgmtv3.MonitoringStatus) bool {
	return getStatus(obj, mgmtv3.ClusterConditionType(c)) == corev1.ConditionUnknown
}

func (c condition) Reason(obj *mgmtv3.MonitoringStatus, reason string) {
	condIdx := findOrCreateCond(obj, mgmtv3.ClusterConditionType(c))
	obj.Conditions[*condIdx].Reason = reason
	touchTS(obj, condIdx)
}

func (c condition) SetMessageIfBlank(obj *mgmtv3.MonitoringStatus, message string) {
	if c.GetMessage(obj) == "" {
		c.Message(obj, message)
	}
}

func (c condition) Message(obj *mgmtv3.MonitoringStatus, message string) {
	condIdx := findOrCreateCond(obj, mgmtv3.ClusterConditionType(c))
	obj.Conditions[*condIdx].Message = message
	touchTS(obj, condIdx)
}

func (c condition) GetMessage(obj *mgmtv3.MonitoringStatus) string {
	condIdx := findCond(obj, mgmtv3.ClusterConditionType(c))
	if condIdx == nil {
		return ""
	}
	return obj.Conditions[*condIdx].Message
}

func (c condition) ReasonAndMessageFromError(obj *mgmtv3.MonitoringStatus, err error) {
	if k8serrors.IsConflict(err) {
		return
	}
	condIdx := findOrCreateCond(obj, mgmtv3.ClusterConditionType(c))
	obj.Conditions[*condIdx].Message = err.Error()
	obj.Conditions[*condIdx].Reason = "Error"
	touchTS(obj, condIdx)
}

func (c condition) GetReason(obj *mgmtv3.MonitoringStatus) string {
	condIdx := findCond(obj, mgmtv3.ClusterConditionType(c))
	if condIdx == nil {
		return ""
	}
	return obj.Conditions[*condIdx].Reason
}

func (c condition) Once(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error)) (*mgmtv3.MonitoringStatus, error) {
	if c.IsFalse(obj) {
		return obj, &controller.ForgetError{
			Err: errors.New(c.GetReason(obj)),
		}
	}
	return c.DoUntilTrue(obj, f)
}

func (c condition) DoUntilTrue(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error)) (*mgmtv3.MonitoringStatus, error) {
	if c.IsTrue(obj) {
		return obj, nil
	}
	return c.do(obj, f, true)
}

func (c condition) DoUntilFalse(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error)) (*mgmtv3.MonitoringStatus, error) {
	if c.IsFalse(obj) {
		return obj, nil
	}
	return c.do(obj, f, false)
}

func (c condition) Do(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error)) (*mgmtv3.MonitoringStatus, error) {
	return c.do(obj, f, true)
}

func (c condition) do(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error), conditionStatus bool) (*mgmtv3.MonitoringStatus, error) {
	status := c.GetStatus(obj)
	ts := c.GetLastUpdated(obj)
	reason := c.GetReason(obj)
	message := c.GetMessage(obj)

	obj, err := c.doInternal(obj, f, conditionStatus)

	// This is to prevent non stop flapping of states and update
	if status == c.GetStatus(obj) &&
		reason == c.GetReason(obj) {
		if message != c.GetMessage(obj) {
			replaced := temfileRegexp.ReplaceAllString(c.GetMessage(obj), "file_path_redacted")
			c.Message(obj, replaced)
		}
		if message == c.GetMessage(obj) {
			c.LastUpdated(obj, ts)
		}
	}

	return obj, err
}

func (c condition) doInternal(obj *mgmtv3.MonitoringStatus, f func() (*mgmtv3.MonitoringStatus, error), conditionStatus bool) (*mgmtv3.MonitoringStatus, error) {
	if !c.IsFalse(obj) {
		c.Unknown(obj)
	}

	newObj, err := f()
	if newObj != nil {
		obj = newObj
	}

	if err != nil {
		if _, ok := err.(*controller.ForgetError); ok {
			if c.GetMessage(obj) == "" {
				c.ReasonAndMessageFromError(obj, err)
			}
			return obj, err
		}
		if conditionStatus {
			c.False(obj)
		} else {
			c.Unknown(obj)
		}
		c.ReasonAndMessageFromError(obj, err)
		return obj, err
	}
	if conditionStatus {
		c.True(obj)
	} else {
		c.False(obj)
	}
	c.Reason(obj, "")
	c.Message(obj, "")
	return obj, nil
}

func touchTS(obj *mgmtv3.MonitoringStatus, condIdx *int) {
	obj.Conditions[*condIdx].LastUpdateTime = time.Now().Format(time.RFC3339)
}

func setTS(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType, ts string) {
	condIdx := findOrCreateCond(obj, condName)
	obj.Conditions[*condIdx].LastUpdateTime = ts
}

func getTS(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType) string {
	condIdx := findCond(obj, condName)
	if condIdx == nil {
		return ""
	}
	return obj.Conditions[*condIdx].LastUpdateTime
}

func setStatus(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType, status corev1.ConditionStatus) {
	condIdx := findOrCreateCond(obj, condName)
	obj.Conditions[*condIdx].Status = status
	touchTS(obj, condIdx)
}

func getStatus(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType) corev1.ConditionStatus {
	condIdx := findCond(obj, condName)
	if condIdx == nil {
		return ""
	}
	return obj.Conditions[*condIdx].Status
}

func findCond(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType) *int {
	for idx, cond := range obj.Conditions {
		if cond.Type == condName {
			return &idx
		}
	}
	return nil
}

func findOrCreateCond(obj *mgmtv3.MonitoringStatus, condName mgmtv3.ClusterConditionType) *int {
	if condIdx := findCond(obj, condName); condIdx != nil {
		return condIdx
	}

	obj.Conditions = append(
		obj.Conditions,
		mgmtv3.MonitoringCondition{
			Type:   condName,
			Status: corev1.ConditionUnknown,
		},
	)

	size := len(obj.Conditions) - 1
	return &size
}
