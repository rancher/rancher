package jsonpath

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

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
		if c == ':' || c == ']' {
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

func parseIndexRange(path []byte) (int, *indexRange, error) {
	if path[0] != '[' {
		return 0, nil, fmt.Errorf("expected \"[\" but found %c", path[0])
	}

	head := 1

	if path[head] == '*' {
		if path[head+1] != ']' {
			return 0, nil, fmt.Errorf("expected \"]\" but found %c", path[head+1])
		}

		return head + 2, &indexRange{isWildcard: true}, nil
	}

	consumed, start, err := parseInt(path[1:], 0)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range start: %v", err)
	}

	head += consumed

	if path[head] == ']' {
		return head + 1, &indexRange{start: &start}, nil
	} else if path[head] != ':' {
		return 0, nil, fmt.Errorf("expected \":\" but found %c", path[head])
	}

	head++
	consumed, end, err := parseInt(path[head:], -1)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range end: %v", err)
	}

	head += consumed

	if path[head] == ']' {
		return head + 1, &indexRange{start: &start, end: &end}, nil
	} else if path[head] != ':' {
		head += 1
		return 0, nil, fmt.Errorf("expected \":\" but found %c", path[head])
	}

	head++
	consumed, step, err := parseInt(path[head:], 1)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse range step: %v", err)
	}

	head += consumed

	if path[head] != ']' {
		return 0, nil, fmt.Errorf("expected \"]\" but found %c", path[head])
	}

	return head + 1, &indexRange{start: &start, end: &end, step: &step}, nil
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

	// if next character is a bracket and isn't starting a new bracket identifier parse index range
	if i < len(path)-2 && path[i] == '[' && path[i+1] != '\'' {
		consumed, r, err := parseIndexRange(path[i:])
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse index range: %v", err)
		}

		i += consumed
		s.r = r
	}

	return i, s, nil
}

func parseChildSelectorDotNotation(path []byte) (int, selector, error) {
	var i int
	var builder strings.Builder

charLoop:
	for i = 0; i < len(path); i++ {
		switch path[i] {
		case '.', '[':
			break charLoop

		default:
			if path[i] == '*' && i > 0 {
				return 0, nil, fmt.Errorf("found unescaped '*' in identifier")
			}

			if path[i] != '*' && !(path[i] >= 'A' && path[i] <= 'Z' || path[i] >= 'a' && path[i] <= 'z' || path[i] == '_' || path[i] == '-') {
				return 0, nil, fmt.Errorf("only characters in range A-Za-z_- are allowed in dot notation identifiers but found '%c'", path[i])
			}

			builder.WriteByte(path[i])
		}
	}

	identifier := builder.String()

	s := selectChild{
		identifier: identifier,
		isWildcard: identifier == "*",
	}

	if i < len(path)-2 && path[i] == '[' && path[i+1] != '\'' {
		consumed, indexRange, err := parseIndexRange(path[i:])
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse index range: %v", err)
		}

		i += consumed

		s.r = indexRange
	}

	return i, s, nil
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
			consumed, s, err := parseChildSelectorDotNotation(pathBytes[i+1:])
			if err != nil {
				return nil, fmt.Errorf("failed to parse child selector: %v", err)
			}
			i += 1 + consumed

			jsonPath.selectors = append(jsonPath.selectors, s)
		default:
			return nil, fmt.Errorf("unexpected character '%c'", pathBytes[i])
		}
	}

	return jsonPath, nil
}
