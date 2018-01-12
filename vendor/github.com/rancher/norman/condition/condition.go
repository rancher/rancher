package condition

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	err2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type Cond string

func (c Cond) True(obj runtime.Object) {
	setStatus(obj, string(c), "True")
}

func (c Cond) IsTrue(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "True"
}

func (c Cond) False(obj runtime.Object) {
	setStatus(obj, string(c), "False")
}

func (c Cond) IsFalse(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "False"
}

func (c Cond) Unknown(obj runtime.Object) {
	setStatus(obj, string(c), "Unknown")
}

func (c Cond) IsUnknown(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "Unknown"
}

func (c Cond) Reason(obj runtime.Object, reason string) {
	cond := findOrCreateCond(obj, string(c))
	getFieldValue(cond, "Reason").SetString(reason)
}

func (c Cond) Message(obj runtime.Object, message string) {
	cond := findOrCreateCond(obj, string(c))
	getFieldValue(cond, "Message").SetString(message)
}

func (c Cond) GetMessage(obj runtime.Object) string {
	cond := findOrCreateCond(obj, string(c))
	return getFieldValue(cond, "Message").String()
}

func (c Cond) ReasonAndMessageFromError(obj runtime.Object, err error) {
	if err2.IsConflict(err) {
		return
	}
	cond := findOrCreateCond(obj, string(c))
	getFieldValue(cond, "Message").SetString(err.Error())
	if ce, ok := err.(*conditionError); ok {
		getFieldValue(cond, "Reason").SetString(ce.reason)
	} else {
		getFieldValue(cond, "Reason").SetString("Error")
	}
	touchTS(cond)
}

func (c Cond) GetReason(obj runtime.Object) string {
	cond := findOrCreateCond(obj, string(c))
	return getFieldValue(cond, "Reason").String()
}

func (c Cond) Once(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	if c.IsFalse(obj) {
		return obj, &controller.ForgetError{
			Err: errors.New(c.GetReason(obj)),
		}
	}

	if c.IsTrue(obj) {
		return obj, nil
	}

	c.Unknown(obj)
	newObj, err := f()
	if newObj != nil {
		obj = newObj
	}

	if err != nil {
		c.False(obj)
		c.ReasonAndMessageFromError(obj, err)
		return obj, err
	}
	c.True(obj)
	return obj, nil
}

func (c Cond) DoUntilTrue(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	if c.IsTrue(obj) {
		return obj, nil
	}

	c.Unknown(obj)
	newObj, err := f()
	if newObj != nil {
		obj = newObj
	}

	if err != nil {
		c.ReasonAndMessageFromError(obj, err)
		return obj, err
	}
	c.True(obj)
	c.Reason(obj, "")
	c.Message(obj, "")
	return obj, nil
}

func (c Cond) Do(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	return c.do(obj, f)
}

func (c Cond) do(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	c.Unknown(obj)
	newObj, err := f()
	if newObj != nil {
		obj = newObj
	}

	if err != nil {
		c.False(obj)
		c.ReasonAndMessageFromError(obj, err)
		return obj, err
	}
	c.True(obj)
	c.Reason(obj, "")
	c.Message(obj, "")
	return obj, nil
}

func touchTS(value reflect.Value) {
	now := time.Now().Format(time.RFC3339)
	getFieldValue(value, "LastUpdateTime").SetString(now)
}

func getStatus(obj interface{}, condName string) string {
	cond := findOrCreateCond(obj, condName)
	return getFieldValue(cond, "Status").String()
}

func setStatus(obj interface{}, condName, status string) {
	cond := findOrCreateCond(obj, condName)
	getFieldValue(cond, "Status").SetString(status)
	touchTS(cond)
}

func findOrCreateCond(obj interface{}, condName string) reflect.Value {
	condSlice := getValue(obj, "Status", "Conditions")
	cond := findCond(condSlice, condName)
	if cond != nil {
		return *cond
	}

	newCond := reflect.New(condSlice.Type().Elem()).Elem()
	newCond.FieldByName("Type").SetString(condName)
	newCond.FieldByName("Status").SetString("Unknown")
	condSlice.Set(reflect.Append(condSlice, newCond))
	return newCond
}

func findCond(val reflect.Value, name string) *reflect.Value {
	for i := 0; i < val.Len(); i++ {
		cond := val.Index(i)
		typeVal := getFieldValue(cond, "Type")
		if typeVal.String() == name {
			return &cond
		}
	}

	return nil
}

func getValue(obj interface{}, name ...string) reflect.Value {
	v := reflect.ValueOf(obj)
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = v.Type()
	}

	field := v.FieldByName(name[0])
	if len(name) == 1 {
		return field
	}
	return getFieldValue(field, name[1:]...)
}

func getFieldValue(v reflect.Value, name ...string) reflect.Value {
	field := v.FieldByName(name[0])
	if len(name) == 1 {
		return field
	}
	return getFieldValue(field, name[1:]...)
}

func Error(reason string, err error) error {
	return &conditionError{
		reason:  reason,
		message: err.Error(),
	}
}

type conditionError struct {
	reason  string
	message string
}

func (e *conditionError) Error() string {
	return e.message
}
