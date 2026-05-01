package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      string
		want        *Filter
		wantErr     bool
		errContains string
	}{
		// Empty/nil cases
		{
			name:   "empty filter returns nil",
			filter: "",
			want:   nil,
		},
		{
			name:   "whitespace only returns nil",
			filter: "   ",
			want:   nil,
		},

		// Equal operator
		{
			name:   "eq operator",
			filter: `userName eq "john.doe"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEqual,
				Value:     "john.doe",
			},
		},
		{
			name:   "eq operator uppercase",
			filter: `userName EQ "test"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEqual,
				Value:     "test",
			},
		},

		// Not equal operator
		{
			name:   "ne operator",
			filter: `active ne "false"`,
			want: &Filter{
				Attribute: "active",
				Operator:  opNotEqual,
				Value:     "false",
			},
		},

		// Contains operator
		{
			name:   "co operator",
			filter: `userName co "john"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opContains,
				Value:     "john",
			},
		},

		// Starts with operator
		{
			name:   "sw operator",
			filter: `userName sw "john"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opStartsWith,
				Value:     "john",
			},
		},

		// Ends with operator
		{
			name:   "ew operator",
			filter: `userName ew "@example.com"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEndsWith,
				Value:     "@example.com",
			},
		},

		// Present operator
		{
			name:   "pr operator",
			filter: `externalId pr`,
			want: &Filter{
				Attribute: "externalId",
				Operator:  opPresent,
				Value:     "",
			},
		},

		// Comparison operators
		{
			name:   "gt operator",
			filter: `created gt "2024-01-01"`,
			want: &Filter{
				Attribute: "created",
				Operator:  opGreaterThan,
				Value:     "2024-01-01",
			},
		},
		{
			name:   "ge operator",
			filter: `created ge "2024-01-01"`,
			want: &Filter{
				Attribute: "created",
				Operator:  opGreaterOrEqual,
				Value:     "2024-01-01",
			},
		},
		{
			name:   "lt operator",
			filter: `created lt "2024-12-31"`,
			want: &Filter{
				Attribute: "created",
				Operator:  opLessThan,
				Value:     "2024-12-31",
			},
		},
		{
			name:   "le operator",
			filter: `created le "2024-12-31"`,
			want: &Filter{
				Attribute: "created",
				Operator:  opLessOrEqual,
				Value:     "2024-12-31",
			},
		},

		// Value edge cases
		{
			name:   "value with spaces",
			filter: `displayName eq "Engineering Team"`,
			want: &Filter{
				Attribute: "displayName",
				Operator:  opEqual,
				Value:     "Engineering Team",
			},
		},
		{
			name:   "value with special characters",
			filter: `userName eq "user@example.com"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEqual,
				Value:     "user@example.com",
			},
		},
		{
			name:   "escaped quote in value",
			filter: `displayName eq "Team \"Alpha\""`,
			want: &Filter{
				Attribute: "displayName",
				Operator:  opEqual,
				Value:     `Team "Alpha"`,
			},
		},
		{
			name:   "extra whitespace",
			filter: `  userName   eq   "test"  `,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEqual,
				Value:     "test",
			},
		},

		// Attribute edge cases
		{
			name:   "nested attribute",
			filter: `name.familyName eq "Smith"`,
			want: &Filter{
				Attribute: "name.familyName",
				Operator:  opEqual,
				Value:     "Smith",
			},
		},
		{
			name:   "attribute with underscore",
			filter: `external_id eq "123"`,
			want: &Filter{
				Attribute: "external_id",
				Operator:  opEqual,
				Value:     "123",
			},
		},

		// Error cases
		{
			name:        "invalid operator",
			filter:      `userName xx "john"`,
			wantErr:     true,
			errContains: "invalid filter operator",
		},
		{
			name:        "missing value for eq",
			filter:      `userName eq`,
			wantErr:     true,
			errContains: "requires a value",
		},
		{
			name:        "pr with value",
			filter:      `userName pr "john"`,
			wantErr:     true,
			errContains: "does not accept a value",
		},
		{
			name:        "unclosed quote",
			filter:      `userName eq "john`,
			wantErr:     true,
			errContains: "unclosed quote",
		},
		{
			name:        "only attribute",
			filter:      `userName`,
			wantErr:     true,
			errContains: "invalid filter syntax",
		},
		{
			name:        "invalid attribute name starts with digit",
			filter:      `123name eq "test"`,
			wantErr:     true,
			errContains: "invalid attribute name",
		},
		{
			name:        "invalid attribute name with hyphen",
			filter:      `user-name eq "test"`,
			wantErr:     true,
			errContains: "invalid attribute name",
		},

		// URN-prefixed attributes
		{
			name:   "URN-prefixed user attribute",
			filter: `urn:ietf:params:scim:schemas:core:2.0:User:userName eq "john.doe"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  opEqual,
				Value:     "john.doe",
			},
		},
		{
			name:   "URN-prefixed group attribute",
			filter: `urn:ietf:params:scim:schemas:core:2.0:Group:displayName eq "Engineering"`,
			want: &Filter{
				Attribute: "displayName",
				Operator:  opEqual,
				Value:     "Engineering",
			},
		},
		{
			name:   "URN-prefixed attribute with pr operator",
			filter: `urn:ietf:params:scim:schemas:core:2.0:User:externalId pr`,
			want: &Filter{
				Attribute: "externalId",
				Operator:  opPresent,
				Value:     "",
			},
		},
		{
			name:        "unknown URN prefix in filter",
			filter:      `urn:custom:schema:attr eq "value"`,
			wantErr:     true,
			errContains: "unrecognized schema URN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilter(tt.filter)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripSchemaURN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		path                 string
		expectedResourceType string
		wantAttrPath         string
		wantResourceType     string
		wantErr              bool
		errContains          string
	}{
		{
			name:         "bare attribute",
			path:         "userName",
			wantAttrPath: "userName",
		},
		{
			name:         "bare nested attribute",
			path:         "name.familyName",
			wantAttrPath: "name.familyName",
		},
		{
			name:             "user URN prefix",
			path:             "urn:ietf:params:scim:schemas:core:2.0:User:userName",
			wantAttrPath:     "userName",
			wantResourceType: "User",
		},
		{
			name:             "group URN prefix",
			path:             "urn:ietf:params:scim:schemas:core:2.0:Group:displayName",
			wantAttrPath:     "displayName",
			wantResourceType: "Group",
		},
		{
			name:                 "user URN matching resource type",
			path:                 "urn:ietf:params:scim:schemas:core:2.0:User:active",
			expectedResourceType: "User",
			wantAttrPath:         "active",
			wantResourceType:     "User",
		},
		{
			name:                 "user URN mismatched resource type",
			path:                 "urn:ietf:params:scim:schemas:core:2.0:User:userName",
			expectedResourceType: "Group",
			wantErr:              true,
			errContains:          "does not match",
		},
		{
			name:                 "group URN mismatched resource type",
			path:                 "urn:ietf:params:scim:schemas:core:2.0:Group:displayName",
			expectedResourceType: "User",
			wantErr:              true,
			errContains:          "does not match",
		},
		{
			name:        "unknown URN prefix",
			path:        "urn:ietf:params:scim:schemas:extension:custom:2.0:Foo:bar",
			wantErr:     true,
			errContains: "unrecognized schema URN",
		},
		{
			name:             "URN with complex path",
			path:             "urn:ietf:params:scim:schemas:core:2.0:User:emails[primary eq true].value",
			wantAttrPath:     "emails[primary eq true].value",
			wantResourceType: "User",
		},
		{
			name:             "URN with members filter path",
			path:             `urn:ietf:params:scim:schemas:core:2.0:Group:members[value eq "u-123"]`,
			wantAttrPath:     `members[value eq "u-123"]`,
			wantResourceType: "Group",
		},
		{
			name:         "empty path",
			path:         "",
			wantAttrPath: "",
		},
		{
			name:        "URN prefix with no attribute",
			path:        "urn:ietf:params:scim:schemas:core:2.0:User:",
			wantErr:     true,
			errContains: "missing attribute name",
		},
		{
			name:        "malformed urn prefix",
			path:        "urn:bogus",
			wantErr:     true,
			errContains: "unrecognized schema URN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attrPath, resourceType, err := stripSchemaURN(tt.path, tt.expectedResourceType)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAttrPath, attrPath)
			assert.Equal(t, tt.wantResourceType, resourceType)
		})
	}
}

func TestFilterMatches(t *testing.T) {
	tests := []struct {
		name   string
		filter *Filter
		value  string
		want   bool
	}{
		{
			name:   "nil filter matches everything",
			filter: nil,
			value:  "anything",
			want:   true,
		},
		{
			name:   "eq exact match",
			filter: &Filter{Attribute: "userName", Operator: opEqual, Value: "john.doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "eq case insensitive match",
			filter: &Filter{Attribute: "userName", Operator: opEqual, Value: "John.Doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "eq no match",
			filter: &Filter{Attribute: "userName", Operator: opEqual, Value: "jane.doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "ne match",
			filter: &Filter{Attribute: "userName", Operator: opNotEqual, Value: "jane.doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "ne no match",
			filter: &Filter{Attribute: "userName", Operator: opNotEqual, Value: "john.doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "co match",
			filter: &Filter{Attribute: "userName", Operator: opContains, Value: "ohn"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "co no match",
			filter: &Filter{Attribute: "userName", Operator: opContains, Value: "xyz"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "sw match",
			filter: &Filter{Attribute: "userName", Operator: opStartsWith, Value: "john"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "sw no match",
			filter: &Filter{Attribute: "userName", Operator: opStartsWith, Value: "doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "ew match",
			filter: &Filter{Attribute: "userName", Operator: opEndsWith, Value: ".doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "ew no match",
			filter: &Filter{Attribute: "userName", Operator: opEndsWith, Value: "john"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "pr match when present",
			filter: &Filter{Attribute: "externalId", Operator: opPresent},
			value:  "ext-123",
			want:   true,
		},
		{
			name:   "pr no match when empty",
			filter: &Filter{Attribute: "externalId", Operator: opPresent},
			value:  "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Matches(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterMatchesCaseExact(t *testing.T) {
	tests := []struct {
		name   string
		filter *Filter
		value  string
		want   bool
	}{
		{
			name:   "case exact match",
			filter: &Filter{Attribute: "id", Operator: opEqual, Value: "user-123"},
			value:  "user-123",
			want:   true,
		},
		{
			name:   "case exact no match different case",
			filter: &Filter{Attribute: "id", Operator: opEqual, Value: "User-123"},
			value:  "user-123",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.MatchesCaseExact(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterValidateForAttributes(t *testing.T) {
	tests := []struct {
		name              string
		filter            *Filter
		allowedAttributes []string
		allowedOperators  []filterOperator
		wantErr           bool
		errContains       string
	}{
		{
			name:              "nil filter is valid",
			filter:            nil,
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual},
		},
		{
			name:              "valid attribute and operator",
			filter:            &Filter{Attribute: "userName", Operator: opEqual, Value: "john"},
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual},
		},
		{
			name:              "attribute case insensitive match",
			filter:            &Filter{Attribute: "USERNAME", Operator: opEqual, Value: "john"},
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual},
		},
		{
			name:              "multiple allowed operators",
			filter:            &Filter{Attribute: "userName", Operator: opStartsWith, Value: "john"},
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual, opStartsWith, opContains},
		},
		{
			name:              "multiple allowed attributes matches first",
			filter:            &Filter{Attribute: "userName", Operator: opEqual, Value: "john"},
			allowedAttributes: []string{"userName", "externalId"},
			allowedOperators:  []filterOperator{opEqual},
		},
		{
			name:              "multiple allowed attributes matches second",
			filter:            &Filter{Attribute: "externalId", Operator: opEqual, Value: "ext-123"},
			allowedAttributes: []string{"userName", "externalId"},
			allowedOperators:  []filterOperator{opEqual},
		},
		{
			name:              "unsupported attribute",
			filter:            &Filter{Attribute: "externalId", Operator: opEqual, Value: "ext-123"},
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual},
			wantErr:           true,
			errContains:       "unsupported filter attribute",
		},
		{
			name:              "unsupported operator",
			filter:            &Filter{Attribute: "userName", Operator: opContains, Value: "john"},
			allowedAttributes: []string{"userName"},
			allowedOperators:  []filterOperator{opEqual},
			wantErr:           true,
			errContains:       "unsupported filter operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.ValidateForAttributes(tt.allowedAttributes, tt.allowedOperators...)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        []string
		wantErr     bool
		errContains string
	}{
		{
			name:  "simple tokens",
			input: `a b c`,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "quoted string",
			input: `a "b c" d`,
			want:  []string{"a", "b c", "d"},
		},
		{
			name:  "escaped quote",
			input: `a "b \"c\" d" e`,
			want:  []string{"a", `b "c" d`, "e"},
		},
		{
			name:  "multiple spaces",
			input: `a    b   c`,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "pr operator no value",
			input: `externalId pr`,
			want:  []string{"externalId", "pr"},
		},
		{
			name:        "unclosed quote",
			input:       `a "b c`,
			wantErr:     true,
			errContains: "unclosed quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidAttributeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple name", input: "userName", want: true},
		{name: "with underscore", input: "user_name", want: true},
		{name: "with digit", input: "user1", want: true},
		{name: "nested", input: "name.familyName", want: true},
		{name: "deeply nested", input: "emails.value", want: true},
		{name: "starts with digit", input: "1user", want: false},
		{name: "with hyphen", input: "user-name", want: false},
		{name: "empty", input: "", want: false},
		{name: "only dots", input: "...", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidAttributeName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
