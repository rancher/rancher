package main_test

import (
	"bytes"
	"io"
	"testing"
	"text/template"

	"github.com/rancher/rancher/pkg/codegen/buildconfig"
	"github.com/stretchr/testify/require"
)

func TestGoConstantsWriterRun(t *testing.T) {
	t.Parallel()
	const contents = `b: 3
a: foo
c: 3.14`
	in := bytes.NewBufferString(contents)
	out := new(bytes.Buffer)

	const rawTemplate = `
	package buildconfig
	
	const (
	{{ . }})
	`
	tmpl, err := template.New("").Parse(rawTemplate)
	require.NoError(t, err)
	w := &main.GoConstantsWriter{
		Tmpl:   tmpl,
		Input:  in,
		Output: out,
	}
	require.NoError(t, w.Run())

	want :=
		`package buildconfig

const (
	A = "foo"
	B = "3"
	C = "3.14"
)
`
	got := out.String()
	require.Equal(t, want, got)

	// Running a second time with the same Input source must fail.
	require.Error(t, w.Run())
}

func TestGoConstantsWriterFailsWithBadConfiguration(t *testing.T) {
	t.Parallel()
	const rawTemplate = `
	package buildconfig
	
	const (
	{{ . }})
	`
	tmpl, err := template.New("").Parse(rawTemplate)
	require.NoError(t, err)
	const contents = `a: foo
b: 3
c: 3.14`
	output := new(bytes.Buffer)

	tests := []struct {
		name   string
		tmpl   *template.Template
		input  io.Reader
		output io.Writer
	}{
		{
			name:   "nil template",
			tmpl:   nil,
			input:  bytes.NewBufferString(contents),
			output: output,
		},
		{
			name:   "empty template",
			tmpl:   template.New(""),
			input:  bytes.NewBufferString(contents),
			output: output,
		},
		{
			name:   "nil input",
			tmpl:   tmpl,
			input:  nil,
			output: output,
		},
		{
			name:   "empty input",
			tmpl:   tmpl,
			input:  bytes.NewBufferString(""),
			output: output,
		},
		{
			name:   "nil output",
			tmpl:   tmpl,
			input:  bytes.NewBufferString(contents),
			output: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := main.GoConstantsWriter{
				Input:  test.input,
				Output: test.output,
				Tmpl:   test.tmpl,
			}
			require.Error(t, w.Run())
		})
	}
}
