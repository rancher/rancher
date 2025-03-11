package jsonpath

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/ptr"
)

func isAlphabetic(b byte) bool {
	return b >= 'A' && b <= 'Z' ||
		b >= 'a' && b <= 'z' ||
		b == '_' ||
		b == '-'
}

// parseInt attempts to parse an integer from the provided byte slice. If no integer could be found, value is returned.
func parseInt(path []byte, value int) (int, int, error) {
	intBytes := make([]byte, len(path))

	var i int
	var c byte

	if path[i] == '-' {
		intBytes[i] = '-'
		i++
	}

	for i, c = range path {
		if c == ':' || c == ']' || c == ',' {
			break
		}

		intBytes[i] = c
	}

	if i == 0 {
		return 0, value, nil
	}

	n, err := strconv.Atoi(string(intBytes[:i]))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse integer: %v", err)
	}

	return i, n, nil
}

func parseIndexRange(path []byte, start int) (int, *indexRange, error) {
	var i int

	if path[0] == ']' {
		return 1, &indexRange{start: &start, end: ptr.To(-1)}, nil
	}

	consumed, end, err := parseInt(path[i:], -1)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range end: %v", err)
	}

	i += consumed

	if path[i] == ']' {
		return i + 1, &indexRange{start: &start, end: &end}, nil
	} else if path[i] != ':' {
		i++
		return 0, nil, fmt.Errorf("expected \":\" but found %c", path[i])
	}

	i++
	consumed, step, err := parseInt(path[i:], 1)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range step: %v", err)
	}

	i += consumed

	if path[i] != ']' {
		return 0, nil, fmt.Errorf("expected \"]\" but found %c", path[i])
	}

	return i + 1, &indexRange{start: &start, end: &end, step: &step}, nil
}

func parseIndexUnion(path []byte, start int) (int, *indexRange, error) {
	var i int

	r := &indexRange{
		union: []int{start},
	}

	for i < len(path) {
		consumed, n, err := parseInt(path[i:], 0)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse range union option: %v", err)
		}

		if consumed == 0 {
			return 0, nil, fmt.Errorf("index unions options may not be empty")
		}

		i += consumed

		r.union = append(r.union, n)

		if path[i] == ']' {
			break
		}

		if path[i] != ',' {
			return 0, nil, fmt.Errorf("expected \",\" but found %c", path[i])
		}
	}

	return i + 1, r, nil
}

func parseIdentifierSubscript(path []byte) (int, *indexRange, error) {
	if len(path) == 0 || path[0] != '[' || bytes.HasPrefix(path, []byte{'[', '\''}) {
		return 0, nil, nil
	}

	if len(path) == 1 {
		return 0, nil, fmt.Errorf("expected ']' but none")
	}

	i := 1

	if path[i] == '*' {
		if i+1 >= len(path) {
			return 0, nil, fmt.Errorf("expected \"]\" but none")
		}

		if path[i+1] != ']' {
			return 0, nil, fmt.Errorf("expected \"]\" but found %c", path[i+1])
		}

		return i + 2, &indexRange{isWildcard: true}, nil
	}

	consumed, start, err := parseInt(path[1:], 0)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range start: %v", err)
	}

	i += consumed

	switch path[i] {
	case ']':
		return i + 1, &indexRange{start: &start}, nil
	case ':':
		consumed, r, err := parseIndexRange(path[i+1:], start)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse index range: %v", err)
		}

		return 1 + i + consumed, r, nil
	case ',':
		consumed, r, err := parseIndexUnion(path[i+1:], start)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse index union: %v", err)
		}

		return 1 + i + consumed, r, nil
	default:
		return 0, nil, fmt.Errorf("unexpected character '%c'", path[i])
	}
}

func parseChildSelectorBracketNotation(path []byte) (int, selector, error) {
	var i int
	var builder strings.Builder
	var found bool

	for ; i < len(path); i++ {
		if bytes.HasPrefix(path[i:], []byte{'\'', ']'}) {
			i += 2
			found = true
			break
		}

		if path[i] == '\'' {
			return 0, nil, fmt.Errorf("single quotes must be escapped in bracket notation")
		}

		if path[i] == '\\' {
			i++

			if i == len(path) {
				return 0, nil, fmt.Errorf("unexpected end of path")
			}

			switch path[i] {
			case '\\', '\'':
			default:
				return 0, nil, fmt.Errorf("invalid escape sequence '\\%c'", path[i])
			}
		}

		builder.WriteByte(path[i])
	}

	if !found {
		return 0, nil, fmt.Errorf("expected \"']\" but none was found")
	}

	identifier := builder.String()

	s := selectChild{
		identifier: identifier,
		isWildcard: identifier == "*",
	}

	var err error
	var consumed int

	consumed, s.r, err = parseIdentifierSubscript(path[i:])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse subscript: %v", err)
	}

	return i + consumed, s, nil
}

func parseChildSelectorDotNotation(path []byte) (int, selector, error) {
	var i int
	var builder strings.Builder

	for i = 0; i < len(path); i++ {
		if path[i] == '.' || path[i] == '[' {
			break
		}

		if path[i] == '*' && i > 0 {
			return 0, nil, fmt.Errorf("found unescaped '*' in identifier")
		}

		if path[i] != '*' && !isAlphabetic(path[i]) {
			return 0, nil, fmt.Errorf("only characters in range A-Za-z_- are allowed in dot notation identifiers but found '%c'", path[i])
		}

		builder.WriteByte(path[i])
	}

	identifier := builder.String()

	s := selectChild{
		identifier: identifier,
		isWildcard: identifier == "*",
	}

	var err error
	var consumed int

	consumed, s.r, err = parseIdentifierSubscript(path[i:])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse subscript: %v", err)
	}

	return i + consumed, s, nil
}

func parseChildUnion(path []byte) (int, selector, error) {
	if path[0] != '[' {
		return 0, nil, fmt.Errorf("expected '[' but found %c", path[0])
	}

	i := 1

	var found bool
	var builder strings.Builder

	s := selectChild{
		union: make([]string, 0),
	}

	for i < len(path) {
		if path[i] == ']' {
			found = true
			i++
			break
		}

		if path[i] == ',' {
			if builder.String() == "" {
				return 0, nil, fmt.Errorf("union options may not be empty")
			}

			s.union = append(s.union, builder.String())
			builder.Reset()
		} else if !isAlphabetic(path[i]) {
			return 0, nil, fmt.Errorf("only characters in range A-Za-z_- are allowed in union options but found '%c'", path[i])
		} else {
			builder.WriteByte(path[i])
		}

		i++
	}

	if builder.Len() > 0 {
		if builder.String() == "" {
			return 0, nil, fmt.Errorf("union options may not be empty")
		}

		s.union = append(s.union, builder.String())
	}

	if len(s.union) == 0 {
		return 0, nil, fmt.Errorf("expected at least one union option")
	}

	if !found {
		return 0, nil, fmt.Errorf("expected ']' but none was found")
	}

	var err error
	var consumed int

	consumed, s.r, err = parseIdentifierSubscript(path[i:])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse subscript: %v", err)
	}

	return i + consumed, s, nil
}

func Parse(path string) (*JSONPath, error) {
	jsonPath := &JSONPath{}
	pathBytes := []byte(path)

	if !bytes.HasPrefix(pathBytes, []byte{'$'}) {
		return nil, fmt.Errorf("paths must begin with the root object identifier: '%s'", rootNodeBody)
	}

	jsonPath.selectors = append(jsonPath.selectors, selectRootElement{})
	pathBytes = pathBytes[1:]

	for i := 0; i < len(pathBytes); {
		switch {
		case bytes.HasPrefix(pathBytes[i:], []byte{'[', '\''}):
			consumed, s, err := parseChildSelectorBracketNotation(pathBytes[i+2:])
			if err != nil {
				return nil, fmt.Errorf("failed to parse child selector: %v", err)
			}

			i += 2 + consumed

			jsonPath.selectors = append(jsonPath.selectors, s)
		case bytes.HasPrefix(pathBytes[i:], []byte{'.', '.'}):
			consumed, s, err := parseChildSelectorDotNotation(pathBytes[i+2:])
			if err != nil {
				return nil, fmt.Errorf("failed to parse child selector: %v", err)
			}
			i += consumed + 2

			jsonPath.selectors = append(jsonPath.selectors, selectRecursiveDescent{
				inner: s,
			})
		case pathBytes[i] == '.':
			var s selector
			var err error
			var consumed int

			if pathBytes[i+1] == '[' {
				consumed, s, err = parseChildUnion(pathBytes[i+1:])
				if err != nil {
					return nil, fmt.Errorf("failed to parse child selector: %v", err)
				}

				i += 1 + consumed
			} else {
				consumed, s, err = parseChildSelectorDotNotation(pathBytes[i+1:])
				if err != nil {
					return nil, fmt.Errorf("failed to parse child selector: %v", err)
				}
				i += 1 + consumed
			}

			jsonPath.selectors = append(jsonPath.selectors, s)
		default:
			return nil, fmt.Errorf("unexpected character '%c'", pathBytes[i])
		}
	}

	return jsonPath, nil
}
