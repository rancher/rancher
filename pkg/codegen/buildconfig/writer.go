package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io"
	"reflect"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type buildConstantsConfig struct {
	CspAdapterMinVersion    *string `yaml:"cspAdapterMinVersion,omitempty"`
	DefaultShellVersion     *string `yaml:"defaultShellVersion,omitempty"`
	FleetVersion            *string `yaml:"fleetVersion,omitempty"`
	ProvisioningCAPIVersion *string `yaml:"provisioningCAPIVersion,omitempty"`
	WebhookVersion          *string `yaml:"webhookVersion,omitempty"`
}

type GoConstantsWriter struct {
	Input  io.Reader
	Output io.Writer
	Tmpl   *template.Template
	buf    []byte
	cfg    buildConstantsConfig
}

// Run loads YAML data from the pre-configured Input source, processes it, and outputs a template with formatted
// Go constants in the pre-configured Output source. This method can only be run once, since the Input source gets fully read.
func (f *GoConstantsWriter) Run() error {
	if err := f.load(); err != nil {
		return err
	}
	if err := f.process(); err != nil {
		return err
	}
	if err := f.write(); err != nil {
		return err
	}
	return nil
}

func (f *GoConstantsWriter) load() error {
	if f.Input == nil {
		return errors.New("nil input")
	}
	b, err := io.ReadAll(f.Input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	if len(b) == 0 {
		return errors.New("nothing was read")
	}
	if err := yaml.Unmarshal(b, &f.cfg); err != nil {
		return fmt.Errorf("failed to unmarshal raw YAML from input: %w", err)
	}
	return nil
}

func (f *GoConstantsWriter) process() error {
	if f.Tmpl == nil {
		return errors.New("nil template")
	}

	capitalize := cases.Title(language.English, cases.NoLower)
	var builder strings.Builder
	v := reflect.ValueOf(f.cfg)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		// Dereference pointers to real values and skip nil pointers
		fieldValue := field.Interface()
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				continue
			}

			// Dereference the pointer to get the underlying value
			fieldValue = field.Elem().Interface()
		}

		// Capitalize the key to make the constant exported in the generated Go file.
		fieldName := v.Type().Field(i).Name
		fieldName = capitalize.String(fieldName)
		s := fmt.Sprintf("\t%s = %q\n", fieldName, fieldValue)
		builder.WriteString(s)
	}

	buf := new(bytes.Buffer)
	if err := f.Tmpl.Execute(buf, builder.String()); err != nil {
		return err
	}
	f.buf = buf.Bytes()
	return nil
}

func (f *GoConstantsWriter) write() error {
	if f.Output == nil {
		return errors.New("nil output")
	}
	formatted, err := format.Source(f.buf)
	if err != nil {
		return err
	}
	_, err = f.Output.Write(formatted)
	return err
}
