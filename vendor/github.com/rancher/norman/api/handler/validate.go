package handler

import (
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/types"
)

func ParseAndValidateBody(apiContext *types.APIContext, create bool) (map[string]interface{}, error) {
	data, err := parse.Body(apiContext.Request)
	if err != nil {
		return nil, err
	}

	if create {
		for key, value := range apiContext.SubContextAttributeProvider.Create(apiContext, apiContext.Schema) {
			if data == nil {
				data = map[string]interface{}{}
			}
			data[key] = value
		}
	}

	b := builder.NewBuilder(apiContext)

	op := builder.Create
	if !create {
		op = builder.Update
	}
	data, err = b.Construct(apiContext.Schema, data, op)
	if err != nil {
		return nil, err
	}

	return data, nil
}
