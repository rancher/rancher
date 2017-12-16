package condition

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
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
		c.Reason(obj, err.Error())
		return obj, err
	}
	c.True(obj)
	return obj, nil
}

func (c Cond) Do(obj runtime.Object, f func() error) error {
	c.Unknown(obj)
	if err := f(); err != nil {
		c.False(obj)
		c.Reason(obj, err.Error())
		return err
	}
	c.True(obj)
	return nil
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
