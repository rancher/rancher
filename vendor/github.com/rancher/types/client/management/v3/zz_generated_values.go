package client

const (
	ValuesType                  = "values"
	ValuesFieldBoolValue        = "boolValue"
	ValuesFieldIntValue         = "intValue"
	ValuesFieldStringSliceValue = "stringSliceValue"
	ValuesFieldStringValue      = "stringValue"
)

type Values struct {
	BoolValue        *bool    `json:"boolValue,omitempty"`
	IntValue         *int64   `json:"intValue,omitempty"`
	StringSliceValue []string `json:"stringSliceValue,omitempty"`
	StringValue      string   `json:"stringValue,omitempty"`
}
