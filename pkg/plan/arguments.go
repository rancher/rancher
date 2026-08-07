package plan

import "strings"

// Arguments is an ordered command-line argument list. It is a lightweight view
// over the supplied slice; NewArguments does not copy the values.
type Arguments []string

// NewArguments creates an Arguments view for querying command-line options.
func NewArguments(args []string) Arguments {
	return Arguments(args)
}

// First returns the first value supplied for any exact option name, or an empty
// string when none has a value. It accepts
// both split (--option value) and combined (--option=value) forms. Names are
// checked in argument order, so aliases preserve the command line's precedence.
func (a Arguments) First(names ...string) string {
	for i, arg := range a {
		for _, name := range names {
			if arg == name {
				if i+1 < len(a) {
					return a[i+1]
				}
				continue
			}
			if strings.HasPrefix(arg, name+"=") {
				return strings.TrimPrefix(arg, name+"=")
			}
		}
	}
	return ""
}

// Values returns every value supplied for one exact option name, preserving
// command-line order. It accepts both split (--option value) and combined
// (--option=value) forms, which is useful for repeated wrapper options.
func (a Arguments) Values(name string) []string {
	var values []string
	for i, arg := range a {
		if arg == name {
			if i+1 < len(a) {
				values = append(values, a[i+1])
			}
			continue
		}
		if strings.HasPrefix(arg, name+"=") {
			values = append(values, strings.TrimPrefix(arg, name+"="))
		}
	}
	return values
}
