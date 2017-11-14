package mapper

import (
	"fmt"

	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

type SetValue struct {
	From, To string
	Value    interface{}
	IfEq     interface{}
}

func (s SetValue) FromInternal(data map[string]interface{}) {
	v, ok := values.GetValue(data, strings.Split(s.From, "/")...)
	if !ok {
		return
	}

	if v == s.IfEq {
		values.PutValue(data, s.Value, strings.Split(s.To, "/")...)
	}
}

func (s SetValue) ToInternal(data map[string]interface{}) {
	v, ok := values.GetValue(data, strings.Split(s.To, "/")...)
	if !ok {
		return
	}

	if v == s.Value {
		values.PutValue(data, s.IfEq, strings.Split(s.From, "/")...)
	}
}

func (s SetValue) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	_, _, _, ok, err := getField(schema, schemas, s.To)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("failed to find defined field for %s on schemas %s", s.To, schema.ID)
	}

	return nil
}
