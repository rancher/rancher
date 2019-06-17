package client

const (
	StringMatchType        = "stringMatch"
	StringMatchFieldExact  = "exact"
	StringMatchFieldPrefix = "prefix"
	StringMatchFieldRegex  = "regex"
	StringMatchFieldSuffix = "suffix"
)

type StringMatch struct {
	Exact  string `json:"exact,omitempty" yaml:"exact,omitempty"`
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Regex  string `json:"regex,omitempty" yaml:"regex,omitempty"`
	Suffix string `json:"suffix,omitempty" yaml:"suffix,omitempty"`
}
