// Copyright © 2018 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause

package fluentd

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/vmware/kube-fluentd-operator/config-reloader/util"
)

var reComment = regexp.MustCompile("^\\s*#.*$")

var reStartDirective = regexp.MustCompile("^<([^/\\s]+)(\\s+(.*))?>\\s*")

var reEndDirective = regexp.MustCompile("^</(.*)>\\s*")

var reParam = regexp.MustCompile("^([^<\\s]+)(\\s+(.+))?")

type Fragment []*Directive

type Params map[string]*Param

// Directive represents a fluentd directive:
// <Name Tag>
//   Params[0]
//   Params[n]
//   <Nested[0].Name Nested[0].Tag>
//    ...etc
//   </Nested[0].Name>
// </Name>
type Directive struct {
	Name   string
	Tag    string
	Params Params
	Nested Fragment
}

// Param just holds a Name/Value pair
type Param struct {
	Name  string
	Value string
}

// Type return the @type parameter or type parameter. "" if not type is defined
func (d *Directive) Type() string {
	// basic v0/v1 compatibility
	p := d.Params["@type"]
	if p == nil {
		p = d.Params["type"]
	}

	if p == nil {
		return ""
	}

	return p.Value
}

func (f Fragment) Clone() Fragment {
	if f == nil {
		return nil
	}

	res := Fragment{}
	for _, ele := range f {
		res = append(res, ele.Clone())
	}
	return res
}

func (d *Directive) Clone() *Directive {
	if d == nil {
		return nil
	}

	return &Directive{
		Name:   d.Name,
		Tag:    d.Tag,
		Params: d.Params.Clone(),
		Nested: d.Nested.Clone(),
	}
}

func (p *Param) Clone() *Param {
	return &Param{
		Name:  p.Name,
		Value: p.Value,
	}
}

func (p Params) Clone() Params {
	res := Params{}
	for k, v := range p {
		res[k] = v.Clone()
	}

	return res
}

// ParamsFromKV make a Params from a string-string map
func ParamsFromKV(keyValues ...string) Params {
	res := make(map[string]*Param, len(keyValues)/2)

	for i := 0; i < len(keyValues)-1; i += 2 {
		if i+1 >= len(keyValues) {
			// the last missing value is ignored
			continue
		}

		k := keyValues[i]
		v := keyValues[i+1]
		res[k] = &Param{
			Name:  k,
			Value: v,
		}
	}

	return res
}

func (d *Directive) ParamVerbatim(name string) string {
	p := d.Params[name]

	if p == nil {
		return ""
	}

	return p.Value
}

func (d *Directive) Param(name string) string {
	return util.TrimTrailingComment(d.ParamVerbatim(name))
}

// SetParam associates a value with the given name. If value is empty it
// clears the parameter
func (d *Directive) SetParam(name string, value string) {
	p := d.Params[name]

	if value == "" {
		delete(d.Params, name)
		return
	}

	if p == nil {
		p = &Param{
			Name:  name,
			Value: value,
		}
		d.Params[name] = p
	} else {
		p.Value = value
	}
}

func (d *Directive) String() string {
	return d.stringIndent(0)
}

func writeIndent(b *bytes.Buffer, n int) {
	b.WriteString(strings.Repeat(" ", n))
}

func sortedKeys(m Params) []string {
	keys := make([]string, len(m))
	i := 0

	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	return keys
}

func (d *Directive) stringIndent(indent int) string {
	var buffer bytes.Buffer

	writeIndent(&buffer, indent)
	t := d.Tag
	if t != "" {
		t = " " + t
	}
	buffer.WriteString(fmt.Sprintf("<%s%s>\n", d.Name, t))

	for _, k := range sortedKeys(d.Params) {
		v := d.Params[k]
		writeIndent(&buffer, indent+2)
		buffer.WriteString(v.String())
	}

	if len(d.Params) > 0 && len(d.Nested) > 0 {
		buffer.WriteString("\n")
	}

	for _, n := range d.Nested {
		buffer.WriteString(n.stringIndent(indent + 2))
	}

	writeIndent(&buffer, indent)
	buffer.WriteString(fmt.Sprintf("</%s>\n", d.Name))

	return buffer.String()
}

func (p *Param) String() string {
	return fmt.Sprintf("%s %s\n", p.Name, p.Value)
}

func (f Fragment) String() string {
	buf := &bytes.Buffer{}

	for _, element := range f {
		buf.WriteString(element.stringIndent(0))
		buf.WriteString("\n")
	}

	return buf.String()
}

func topDir(s *Stack) *Directive {
	top := s.Peek()
	return top.(*Directive)
}

// ParseString produces a fragment of fluentd config part
func ParseString(s string) (Fragment, error) {
	res := []*Directive{}

	stack := NewStack()
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		if reComment.MatchString(line) {
			continue
		}

		line = util.Trim(line)

		start := reStartDirective.FindStringSubmatch(line)
		if len(start) > 0 {
			d := &Directive{
				Name:   util.Trim(start[1]),
				Params: Params{},
			}
			if len(start) > 2 {
				d.Tag = util.Trim(start[3])
			}

			if stack.Len() == 0 {
				res = append(res, d)
			} else {
				top := topDir(stack)
				top.Nested = append(top.Nested, d)
			}
			stack.Push(d)

			continue
		}

		end := reEndDirective.FindStringSubmatch(line)
		if len(end) > 0 {
			name := end[1]

			if stack.Len() == 0 {
				return nil, errors.New("syntax error")
			}
			top := topDir(stack)

			if top.Name == name {
				stack.Pop()
				continue
			}

			return nil, errors.New("mismatched tags")
		}

		p := reParam.FindStringSubmatch(line)
		if len(p) > 0 {
			param := &Param{
				Name: p[1],
			}
			if len(p) > 2 {
				param.Value = p[3]
				// special handling for @type as it is processed
				if param.Name == "type" || param.Name == "@type" {
					param.Value = util.TrimTrailingComment(param.Value)
				}
			}

			if stack.Len() == 0 {
				return nil, fmt.Errorf("syntax error: dangling parameter %s", param.Name)
			}
			top := topDir(stack)
			top.Params[param.Name] = param

			continue
		}
	}

	if stack.Len() != 0 {
		return nil, errors.New("syntax error: incomplete directive")
	}

	return res, nil
}
