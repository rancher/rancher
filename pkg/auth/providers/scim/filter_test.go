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
				Operator:  OpEqual,
				Value:     "john.doe",
			},
		},
		{
			name:   "eq operator uppercase",
			filter: `userName EQ "test"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpEqual,
				Value:     "test",
			},
		},

		// Not equal operator
		{
			name:   "ne operator",
			filter: `active ne "false"`,
			want: &Filter{
				Attribute: "active",
				Operator:  OpNotEqual,
				Value:     "false",
			},
		},

		// Contains operator
		{
			name:   "co operator",
			filter: `userName co "john"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpContains,
				Value:     "john",
			},
		},

		// Starts with operator
		{
			name:   "sw operator",
			filter: `userName sw "john"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpStartsWith,
				Value:     "john",
			},
		},

		// Ends with operator
		{
			name:   "ew operator",
			filter: `userName ew "@example.com"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpEndsWith,
				Value:     "@example.com",
			},
		},

		// Present operator
		{
			name:   "pr operator",
			filter: `externalId pr`,
			want: &Filter{
				Attribute: "externalId",
				Operator:  OpPresent,
				Value:     "",
			},
		},

		// Comparison operators
		{
			name:   "gt operator",
			filter: `created gt "2024-01-01"`,
			want: &Filter{
				Attribute: "created",
				Operator:  OpGreaterThan,
				Value:     "2024-01-01",
			},
		},
		{
			name:   "ge operator",
			filter: `created ge "2024-01-01"`,
			want: &Filter{
				Attribute: "created",
				Operator:  OpGreaterOrEqual,
				Value:     "2024-01-01",
			},
		},
		{
			name:   "lt operator",
			filter: `created lt "2024-12-31"`,
			want: &Filter{
				Attribute: "created",
				Operator:  OpLessThan,
				Value:     "2024-12-31",
			},
		},
		{
			name:   "le operator",
			filter: `created le "2024-12-31"`,
			want: &Filter{
				Attribute: "created",
				Operator:  OpLessOrEqual,
				Value:     "2024-12-31",
			},
		},

		// Value edge cases
		{
			name:   "value with spaces",
			filter: `displayName eq "Engineering Team"`,
			want: &Filter{
				Attribute: "displayName",
				Operator:  OpEqual,
				Value:     "Engineering Team",
			},
		},
		{
			name:   "value with special characters",
			filter: `userName eq "user@example.com"`,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpEqual,
				Value:     "user@example.com",
			},
		},
		{
			name:   "escaped quote in value",
			filter: `displayName eq "Team \"Alpha\""`,
			want: &Filter{
				Attribute: "displayName",
				Operator:  OpEqual,
				Value:     `Team "Alpha"`,
			},
		},
		{
			name:   "extra whitespace",
			filter: `  userName   eq   "test"  `,
			want: &Filter{
				Attribute: "userName",
				Operator:  OpEqual,
				Value:     "test",
			},
		},

		// Attribute edge cases
		{
			name:   "nested attribute",
			filter: `name.familyName eq "Smith"`,
			want: &Filter{
				Attribute: "name.familyName",
				Operator:  OpEqual,
				Value:     "Smith",
			},
		},
		{
			name:   "attribute with underscore",
			filter: `external_id eq "123"`,
			want: &Filter{
				Attribute: "external_id",
				Operator:  OpEqual,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilter(tt.filter)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
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
			filter: &Filter{Attribute: "userName", Operator: OpEqual, Value: "john.doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "eq case insensitive match",
			filter: &Filter{Attribute: "userName", Operator: OpEqual, Value: "John.Doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "eq no match",
			filter: &Filter{Attribute: "userName", Operator: OpEqual, Value: "jane.doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "ne match",
			filter: &Filter{Attribute: "userName", Operator: OpNotEqual, Value: "jane.doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "ne no match",
			filter: &Filter{Attribute: "userName", Operator: OpNotEqual, Value: "john.doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "co match",
			filter: &Filter{Attribute: "userName", Operator: OpContains, Value: "ohn"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "co no match",
			filter: &Filter{Attribute: "userName", Operator: OpContains, Value: "xyz"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "sw match",
			filter: &Filter{Attribute: "userName", Operator: OpStartsWith, Value: "john"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "sw no match",
			filter: &Filter{Attribute: "userName", Operator: OpStartsWith, Value: "doe"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "ew match",
			filter: &Filter{Attribute: "userName", Operator: OpEndsWith, Value: ".doe"},
			value:  "john.doe",
			want:   true,
		},
		{
			name:   "ew no match",
			filter: &Filter{Attribute: "userName", Operator: OpEndsWith, Value: "john"},
			value:  "john.doe",
			want:   false,
		},
		{
			name:   "pr match when present",
			filter: &Filter{Attribute: "externalId", Operator: OpPresent},
			value:  "ext-123",
			want:   true,
		},
		{
			name:   "pr no match when empty",
			filter: &Filter{Attribute: "externalId", Operator: OpPresent},
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
			filter: &Filter{Attribute: "id", Operator: OpEqual, Value: "user-123"},
			value:  "user-123",
			want:   true,
		},
		{
			name:   "case exact no match different case",
			filter: &Filter{Attribute: "id", Operator: OpEqual, Value: "User-123"},
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

func TestFilterValidateForAttribute(t *testing.T) {
	tests := []struct {
		name             string
		filter           *Filter
		allowedAttribute string
		allowedOperators []FilterOperator
		wantErr          bool
		errContains      string
	}{
		{
			name:             "nil filter is valid",
			filter:           nil,
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual},
			wantErr:          false,
		},
		{
			name:             "valid attribute and operator",
			filter:           &Filter{Attribute: "userName", Operator: OpEqual, Value: "john"},
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual},
			wantErr:          false,
		},
		{
			name:             "attribute case insensitive match",
			filter:           &Filter{Attribute: "USERNAME", Operator: OpEqual, Value: "john"},
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual},
			wantErr:          false,
		},
		{
			name:             "multiple allowed operators",
			filter:           &Filter{Attribute: "userName", Operator: OpStartsWith, Value: "john"},
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual, OpStartsWith, OpContains},
			wantErr:          false,
		},
		{
			name:             "unsupported attribute",
			filter:           &Filter{Attribute: "externalId", Operator: OpEqual, Value: "ext-123"},
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual},
			wantErr:          true,
			errContains:      "unsupported filter attribute",
		},
		{
			name:             "unsupported operator",
			filter:           &Filter{Attribute: "userName", Operator: OpContains, Value: "john"},
			allowedAttribute: "userName",
			allowedOperators: []FilterOperator{OpEqual},
			wantErr:          true,
			errContains:      "unsupported filter operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.ValidateForAttribute(tt.allowedAttribute, tt.allowedOperators...)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
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
					assert.Contains(t, err.Error(), tt.errContains)
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
