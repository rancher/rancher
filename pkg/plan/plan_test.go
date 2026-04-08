package plan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("valid plan json", func(t *testing.T) {
		raw := `{"files":[{"path":"/tmp/test","content":"aGVsbG8gd29ybGQ="}],"instructions":[{"name":"setup","command":"/bin/sh"}]}`

		p, err := Parse([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
			t.Errorf("unexpected files: %+v", p.Files)
		}

		if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "setup" {
			t.Errorf("unexpected instructions: %+v", p.OneTimeInstructions)
		}
	})

	t.Run("invalid plan json", func(t *testing.T) {
		_, err := Parse([]byte(`{not valid json`))
		if err == nil {
			t.Fatal("expected an error")
		}

		if !strings.Contains(err.Error(), "failed to parse plan") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestMarshalParse(t *testing.T) {
	original := Plan{
		Files: []File{
			{
				Content:     "aGVsbG8gd29ybGQ=",
				UID:         -1,
				GID:         -1,
				Path:        "/etc/myapp/config.yaml",
				Permissions: "0644",
			},
			{
				Path:      "/etc/myapp/",
				Directory: true,
			},
		},
		OneTimeInstructions: []OneTimeInstruction{
			{
				CommonInstruction: CommonInstruction{
					Name:    "install",
					Command: "/bin/sh",
					Args:    []string{"-c", "echo hello"},
				},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []PeriodicInstruction{
			{
				CommonInstruction: CommonInstruction{
					Name:    "healthcheck",
					Command: "/bin/sh",
					Args:    []string{"-c", "echo ok"},
				},
				PeriodSeconds:    60,
				SaveStderrOutput: true,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored Plan
	restored, err = Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("plan changed after encode/parse\nwant: %+v\n got: %+v", original, restored)
	}
}

func TestParseJSONKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, p Plan)
	}{
		{
			name:  "files",
			input: `{"files":[{"path":"/tmp/test","content":"aGVsbG8gd29ybGQ="}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
					t.Errorf("got %+v", p.Files)
				}
			},
		},
		{
			name:  "instructions",
			input: `{"instructions":[{"name":"test","command":"/bin/sh"}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "test" {
					t.Errorf("got %+v", p.OneTimeInstructions)
				}
			},
		},
		{
			name:  "periodicInstructions",
			input: `{"periodicInstructions":[{"name":"cron","periodSeconds":300}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.PeriodicInstructions) != 1 || p.PeriodicInstructions[0].PeriodSeconds != 300 {
					t.Errorf("got %+v", p.PeriodicInstructions)
				}
			},
		},
		{
			name:  "probes / httpGet",
			input: `{"probes":{"web":{"httpGet":{"url":"http://localhost/health","insecure":true}}}}`,
			validate: func(t *testing.T, p Plan) {
				probe, ok := p.Probes["web"]
				if !ok {
					t.Fatal("probes.web missing")
				}
				if probe.HTTPGetAction.URL != "http://localhost/health" || !probe.HTTPGetAction.Insecure {
					t.Errorf("got %+v", probe.HTTPGetAction)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Plan

			p, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			tt.validate(t, p)
		})
	}
}

func TestChecksum(t *testing.T) {
	input := []byte(`{"files":[]}`)

	// verify idempotency by calling Checksum() twice with the same input
	output1, output2 := Checksum(input), Checksum(input)
	if output1 != output2 {
		t.Error("checksum result is not idempotent")
	}
}
