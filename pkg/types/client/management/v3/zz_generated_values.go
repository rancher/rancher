package client

const (
	ValuesType                  = "values"
	ValuesFieldBoolValue        = "boolValue"
	ValuesFieldIntValue         = "intValue"
	ValuesFieldStringSliceValue = "stringSliceValue"
	ValuesFieldStringValue      = "stringValue"
)

type Values struct {
	BoolValue        bool     `json:"boolValue,omitempty" yaml:"boolValue,omitempty"`
	IntValue         int64    `json:"intValue,omitempty" yaml:"intValue,omitempty"`
	StringSliceValue []string `json:"stringSliceValue,omitempty" yaml:"stringSliceValue,omitempty"`
	StringValue      string   `json:"stringValue,omitempty" yaml:"stringValue,omitempty"`
}
