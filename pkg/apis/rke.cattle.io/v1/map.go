package v1

import (
	"encoding/json"

	"github.com/rancher/wrangler/v3/pkg/data/convert"
)

type GenericMap struct {
	Data map[string]interface{} `json:"-"`
}

func (in GenericMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(in.Data)
}

func (in *GenericMap) UnmarshalJSON(data []byte) error {
	in.Data = map[string]interface{}{}
	return json.Unmarshal(data, &in.Data)
}

func (in *GenericMap) DeepCopyInto(out *GenericMap) {
	out.Data = map[string]interface{}{}
	if err := convert.ToObj(in.Data, &out.Data); err != nil {
		panic(err)
	}
}
