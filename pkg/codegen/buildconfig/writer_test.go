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
	cfg := map[string]string{
		"b": "3",
		"a": "foo",
		"c": "3.14",
	}
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
		Config: cfg,
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
	cfg := map[string]string{
		"a": "foo",
		"b": "3",
		"c": "3.14",
	}
	output := new(bytes.Buffer)

	tests := []struct {
		name   string
		tmpl   *template.Template
		config map[string]string
		output io.Writer
	}{
		{
			name:   "nil template",
			tmpl:   nil,
			config: cfg,
			output: output,
		},
		{
			name:   "empty template",
			tmpl:   template.New(""),
			config: cfg,
			output: output,
		},
		{
			name:   "nil config",
			tmpl:   tmpl,
			config: nil,
			output: output,
		},
		{
			name:   "nil output",
			tmpl:   tmpl,
			config: cfg,
			output: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := main.GoConstantsWriter{
				Config: test.config,
				Output: test.output,
				Tmpl:   test.tmpl,
			}
			require.Error(t, w.Run())
		})
	}
}
