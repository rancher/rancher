package scim

import (
	"fmt"
	"strings"
	"unicode"
)

// filterOperator represents a SCIM filter comparison operator (RFC 7644 §3.4.2.2).
type filterOperator string

const (
	opEqual          filterOperator = "eq" // Equal
	opNotEqual       filterOperator = "ne" // Not equal
	opContains       filterOperator = "co" // Contains
	opStartsWith     filterOperator = "sw" // Starts with
	opEndsWith       filterOperator = "ew" // Ends with
	opPresent        filterOperator = "pr" // Present (has value)
	opGreaterThan    filterOperator = "gt" // Greater than
	opGreaterOrEqual filterOperator = "ge" // Greater than or equal
	opLessThan       filterOperator = "lt" // Less than
	opLessOrEqual    filterOperator = "le" // Less than or equal
)

// supportedOperators lists all operators defined in RFC 7644.
var supportedOperators = map[string]filterOperator{
	"eq": opEqual,
	"ne": opNotEqual,
	"co": opContains,
	"sw": opStartsWith,
	"ew": opEndsWith,
	"pr": opPresent,
	"gt": opGreaterThan,
	"ge": opGreaterOrEqual,
	"lt": opLessThan,
	"le": opLessOrEqual,
}

// Filter represents a parsed SCIM filter expression.
// Currently supports simple attribute filters: attribute op value
// e.g., userName eq "john.doe"
type Filter struct {
	Attribute string
	Operator  filterOperator
	Value     string // Empty for "pr" (present) operator
}

// ParseFilter parses a SCIM filter string into a Filter struct.
// Supports RFC 7644 §3.4.2.2 filter syntax for simple attribute expressions.
//
// Supported formats:
//   - attribute op "value"  (e.g., userName eq "john.doe")
//   - attribute pr          (e.g., externalId pr)
//
// Returns nil if the filter string is empty.
// Returns an error with scimType "invalidFilter" for invalid syntax.
func ParseFilter(filter string) (*Filter, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, nil
	}

	// Tokenize the filter expression.
	tokens, err := tokenize(filter)
	if err != nil {
		return nil, err
	}

	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid filter syntax: expected 'attribute op [value]'")
	}

	attr := tokens[0]
	opStr := strings.ToLower(tokens[1])

	// Validate attribute name.
	if !isValidAttributeName(attr) {
		return nil, fmt.Errorf("invalid attribute name %q", attr)
	}

	// Validate operator.
	op, ok := supportedOperators[opStr]
	if !ok {
		return nil, fmt.Errorf("invalid filter operator %q", opStr)
	}

	// Handle "pr" (present) operator - no value required.
	if op == opPresent {
		if len(tokens) > 2 {
			return nil, fmt.Errorf("operator 'pr' does not accept a value")
		}
		return &Filter{
			Attribute: attr,
			Operator:  op,
			Value:     "",
		}, nil
	}

	// All other operators require a value.
	if len(tokens) != 3 {
		return nil, fmt.Errorf("operator %q requires a value", opStr)
	}

	return &Filter{
		Attribute: attr,
		Operator:  op,
		Value:     tokens[2],
	}, nil
}

// MatchesValue evaluates the filter against a single string value.
// The caseExact parameter controls whether comparison is case-sensitive.
// Returns true if the filter is nil (no filter = match all).
func (f *Filter) MatchesValue(value string, caseExact bool) bool {
	if f == nil {
		return true
	}

	target := f.Value
	compareValue := value

	if !caseExact {
		target = strings.ToLower(target)
		compareValue = strings.ToLower(value)
	}

	switch f.Operator {
	case opEqual:
		return compareValue == target
	case opNotEqual:
		return compareValue != target
	case opContains:
		return strings.Contains(compareValue, target)
	case opStartsWith:
		return strings.HasPrefix(compareValue, target)
	case opEndsWith:
		return strings.HasSuffix(compareValue, target)
	case opPresent:
		return value != ""
	case opGreaterThan:
		return compareValue > target
	case opGreaterOrEqual:
		return compareValue >= target
	case opLessThan:
		return compareValue < target
	case opLessOrEqual:
		return compareValue <= target
	default:
		return false
	}
}

// Matches is a convenience method that performs case-insensitive matching.
// Returns true if the filter is nil (no filter = match all).
func (f *Filter) Matches(value string) bool {
	return f.MatchesValue(value, false)
}

// MatchesCaseExact performs case-sensitive matching.
// Returns true if the filter is nil (no filter = match all).
func (f *Filter) MatchesCaseExact(value string) bool {
	return f.MatchesValue(value, true)
}

// ValidateForAttribute checks if the filter is valid for the specified attribute and operators.
// Returns an error if the filter uses an unsupported attribute or operator.
func (f *Filter) ValidateForAttribute(allowedAttribute string, allowedOperators ...filterOperator) error {
	if f == nil {
		return nil
	}

	// Check attribute (case-insensitive per SCIM spec).
	if !strings.EqualFold(f.Attribute, allowedAttribute) {
		return fmt.Errorf("unsupported filter attribute %q: only %q is supported", f.Attribute, allowedAttribute)
	}

	// Check operator.
	for _, allowed := range allowedOperators {
		if f.Operator == allowed {
			return nil
		}
	}

	allowedStrs := make([]string, len(allowedOperators))
	for i, op := range allowedOperators {
		allowedStrs[i] = string(op)
	}
	return fmt.Errorf("unsupported filter operator %q for attribute %q: only %s allowed",
		f.Operator, f.Attribute, strings.Join(allowedStrs, ", "))
}

// tokenize splits the filter string into tokens: attribute, operator, [value].
// Handles quoted values with escaped quotes inside.
func tokenize(filter string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for i := 0; i < len(filter); i++ {
		c := filter[i]

		if escapeNext {
			current.WriteByte(c)
			escapeNext = false
			continue
		}

		if c == '\\' && inQuotes {
			escapeNext = true
			continue
		}

		if c == '"' {
			inQuotes = !inQuotes
			continue
		}

		if !inQuotes && unicode.IsSpace(rune(c)) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if inQuotes {
		return nil, fmt.Errorf("unclosed quote in filter")
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// isValidAttributeName checks if the attribute name is valid per SCIM spec.
// Valid names contain letters, digits, underscores, and dots (for nested attributes).
// Must start with a letter or underscore.
func isValidAttributeName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if unicode.IsLetter(r) || r == '_' {
			continue
		}
		if i > 0 && (unicode.IsDigit(r) || r == '.') {
			continue
		}
		return false
	}
	return true
}
