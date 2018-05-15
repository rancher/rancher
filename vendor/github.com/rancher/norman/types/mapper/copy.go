package mapper

import (
	"fmt"

	"github.com/rancher/norman/types"
)

type Copy struct {
	From, To string
}

func (c Copy) FromInternal(data map[string]interface{}) {
	if data == nil {
		return
	}
	v, ok := data[c.From]
	if ok {
		data[c.To] = v
	}
}

func (c Copy) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}
	t, tok := data[c.To]
	_, fok := data[c.From]
	if tok && !fok {
		data[c.From] = t
	}
}

func (c Copy) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	f, ok := s.ResourceFields[c.From]
	if !ok {
		return fmt.Errorf("field %s missing on schema %s", c.From, s.ID)
	}

	s.ResourceFields[c.To] = f
	return nil
}
