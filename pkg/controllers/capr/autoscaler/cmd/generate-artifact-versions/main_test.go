package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseMetadataFixture(t *testing.T) {
	var metadata component
	if err := parseMetadata(fixture(t, "component.json"), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Branches) != 4 {
		t.Fatalf("expected 4 branches, got %d", len(metadata.Branches))
	}
	foundChart := false
	for _, artifact := range metadata.Branches[1].Versions[0].Artifacts {
		if artifact.Version == "9.58.0" {
			foundChart = true
			break
		}
	}
	if !foundChart {
		t.Fatal("chart artifact was not parsed")
	}
}

func TestSelectMappingsFixture(t *testing.T) {
	var metadata component
	if err := parseMetadata(fixture(t, "component.json"), &metadata); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(t.TempDir(), "existing.go")
	if err := os.WriteFile(existingPath, fixture(t, "existing.go"), 0644); err != nil {
		t.Fatal(err)
	}
	existing, err := readExisting(existingPath)
	if err != nil {
		t.Fatal(err)
	}

	selected, warnings := selectMappings(metadata, existing)
	if len(warnings) != 1 {
		t.Fatalf("expected one incomplete-stream warning, got %v", warnings)
	}
	if selected[35] != (mapping{ImageTag: "1.35.1-9.1", ChartVersion: "9.58.0"}) {
		t.Fatalf("unexpected 1.35 selection: %#v", selected[35])
	}
	if selected[34] != (mapping{ImageTag: "1.34.2-5.1", ChartVersion: "9.53.0"}) {
		t.Fatalf("unexpected 1.34 selection: %#v", selected[34])
	}
	if selected[32] != existing[32] || selected[31] != existing[31] {
		t.Fatalf("incomplete and absent streams must preserve existing mappings: %#v", selected)
	}
}

func TestRenderFixture(t *testing.T) {
	mappings := map[int]mapping{
		35: {ImageTag: "1.35.1-9.1", ChartVersion: "9.58.0"},
		34: {ImageTag: "1.34.2-5.1", ChartVersion: "9.53.0"},
		32: {ImageTag: "1.32.3-1.5", ChartVersion: "9.50.1"},
		31: {ImageTag: "1.31.4-2.0", ChartVersion: "9.48.0"},
	}
	if actual := render(mappings); !bytes.Equal(actual, fixture(t, "rendered.go")) {
		t.Fatalf("rendered output does not match fixture:\n%s", actual)
	}
}

func TestReadExistingMissingFile(t *testing.T) {
	mappings, err := readExisting(filepath.Join(t.TempDir(), "missing.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 0 {
		t.Fatalf("expected no mappings, got %#v", mappings)
	}
}
