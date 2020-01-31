package subscribe

import (
	"io"

	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/schemaserver/writer"
)

type Converter struct {
	writer.EncodingResponseWriter
	apiOp *types.APIRequest
	obj   interface{}
}

func MarshallObject(apiOp *types.APIRequest, event types.APIEvent) types.APIEvent {
	if event.Error != nil {
		return event
	}

	data, err := newConverter(apiOp).ToAPIObject(event.Object)
	if err != nil {
		event.Error = err
		return event
	}

	event.Data = data
	return event
}

func newConverter(apiOp *types.APIRequest) *Converter {
	c := &Converter{
		apiOp: apiOp,
	}
	c.EncodingResponseWriter = writer.EncodingResponseWriter{
		ContentType: "application/json",
		Encoder:     c.Encoder,
	}
	return c
}

func (c *Converter) ToAPIObject(data types.APIObject) (interface{}, error) {
	c.obj = nil
	if err := c.Body(c.apiOp, nil, data); err != nil {
		return types.APIObject{}, err
	}
	return c.obj, nil
}

func (c *Converter) Encoder(_ io.Writer, obj interface{}) error {
	c.obj = obj
	return nil
}
