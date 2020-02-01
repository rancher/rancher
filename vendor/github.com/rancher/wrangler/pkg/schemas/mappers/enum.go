package mappers

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/schemas"
)

type Enum struct {
	DefaultMapper
	field string
	vals  map[string]string
}

func NewEnum(field string, vals ...string) schemas.Mapper {
	f := &Enum{
		DefaultMapper: DefaultMapper{
			Field: field,
		},
		vals: map[string]string{},
	}

	for _, v := range vals {
		k := v
		if strings.Contains(v, "=") {
			v, k = kv.Split(v, "=")
		}
		f.vals[normalize(v)] = k
	}

	return f
}

func normalize(v string) string {
	v = strings.ReplaceAll(v, "_", "")
	v = strings.ReplaceAll(v, "-", "")
	return strings.ToLower(v)
}

func (d *Enum) FromInternal(data data.Object) {
}

func (d *Enum) ToInternal(data data.Object) error {
	v, ok := data[d.Field]
	if ok {
		newValue, ok := d.vals[normalize(convert.ToString(v))]
		if !ok {
			return fmt.Errorf("%s is not a valid value for field %s", v, d.Field)
		}
		data[d.Field] = newValue
	}
	return nil
}
