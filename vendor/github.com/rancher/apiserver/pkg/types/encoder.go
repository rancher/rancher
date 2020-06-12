package types

import (
	"encoding/json"
	"io"

	"github.com/ghodss/yaml"
)

func JSONEncoder(writer io.Writer, v interface{}) error {
	return json.NewEncoder(writer).Encode(v)
}

func YAMLEncoder(writer io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	_, err = writer.Write(buf)
	return err
}
