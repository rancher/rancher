package utils

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli"
)

type TableWriter struct {
	quite         bool
	HeaderFormat  string
	ValueFormat   string
	err           error
	headerPrinted bool
	Writer        *tabwriter.Writer
}

func NewTableWriter(values [][]string, ctx *cli.Context) *TableWriter {
	t := &TableWriter{
		Writer: tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0),
	}
	t.HeaderFormat, t.ValueFormat = SimpleFormat(values)

	if ctx.Bool("quiet") {
		t.HeaderFormat = ""
		t.ValueFormat = "{{.ID}}\n"
	}

	customFormat := ctx.String("format")
	if customFormat == "json" {
		t.HeaderFormat = ""
		t.ValueFormat = "json"
	} else if customFormat != "" {
		t.ValueFormat = customFormat + "\n"
		t.HeaderFormat = ""
	}

	return t
}

func (t *TableWriter) Err() error {
	return t.err
}

func (t *TableWriter) writeHeader() {
	if t.HeaderFormat != "" && !t.headerPrinted {
		t.headerPrinted = true
		t.err = printTemplate(t.Writer, t.HeaderFormat, struct{}{})
		if t.err != nil {
			return
		}
	}
}

func (t *TableWriter) Write(obj interface{}) {
	if t.err != nil {
		return
	}

	t.writeHeader()
	if t.err != nil {
		return
	}

	if t.ValueFormat == "json" {
		content, err := json.Marshal(obj)
		t.err = err
		if t.err != nil {
			return
		}
		_, t.err = t.Writer.Write(append(content, byte('\n')))
	} else {
		t.err = printTemplate(t.Writer, t.ValueFormat, obj)
	}
}

func (t *TableWriter) Close() error {
	if t.err != nil {
		return t.err
	}
	t.writeHeader()
	if t.err != nil {
		return t.err
	}
	return t.Writer.Flush()
}

func SimpleFormat(values [][]string) (string, string) {
	headerBuffer := bytes.Buffer{}
	valueBuffer := bytes.Buffer{}
	for _, v := range values {
		appendTabDelim(&headerBuffer, v[0])
		if strings.Contains(v[1], "{{") {
			appendTabDelim(&valueBuffer, v[1])
		} else {
			appendTabDelim(&valueBuffer, "{{."+v[1]+"}}")
		}
	}

	headerBuffer.WriteString("\n")
	valueBuffer.WriteString("\n")

	return headerBuffer.String(), valueBuffer.String()
}

func appendTabDelim(buf *bytes.Buffer, value string) {
	if buf.Len() == 0 {
		buf.WriteString(value)
	} else {
		buf.WriteString("\t")
		buf.WriteString(value)
	}
}

func printTemplate(out io.Writer, templateContent string, obj interface{}) error {
	funcMap := map[string]interface{}{
		"json": FormatJSON,
	}
	tmpl, err := template.New("").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return err
	}

	return tmpl.Execute(out, obj)
}

func FormatJSON(data interface{}) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "    ")
	return string(bytes) + "\n", err
}
