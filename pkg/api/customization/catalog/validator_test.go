package catalog

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateURL(t *testing.T) {
	type args struct {
		pathURL string
	}
	tests := []struct {
		name    string
		pathURL string
		wantErr bool
	}{
		{
			name:    "Remove control characters",
			pathURL: "http://example.com/1\r2\n345\b67\t",
			wantErr: true,
		},
		{
			name:    "Remove urlEncoded control characters",
			pathURL: "https://example.com/12%003%1F45%0A%0a6",
			wantErr: true,
		},
		{
			name:    "Remove all control characters, allow uppercase scheme",
			pathURL: "https://www.example%0D.com/Hello\r\nWorld",
			wantErr: true,
		},
		{
			name:    "Allow git protocol",
			pathURL: "git://www.example.com",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := validateURL(tt.pathURL)
			assert.Equal(t, gotErr != nil, tt.wantErr)
		})
	}
}

func TestValidateHelm3Version(t *testing.T) {
	request := testAsAPIContext()
	schema := testAsSchema()
	testsWithValues := []struct {
		name        string
		helmVersion string
		wantErr     bool
	}{
		{
			name:        "Empty helm 3 value",
			helmVersion: "",
			wantErr:     false,
		},
		{
			name:        "Non helm 3 value",
			helmVersion: "v2",
			wantErr:     false,
		},
		{
			name:        "Allow full helm 3 value",
			helmVersion: "helm_v3",
			wantErr:     false,
		},
		{
			name:        "Allow shortened helm 3 value",
			helmVersion: "v3",
			wantErr:     false,
		},
		{
			name:        "Invalid helm 3 value",
			helmVersion: "sdfasdfasdv3",
			wantErr:     true,
		},
	}

	testWithNoVersion := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "No helm 3 value in data map",
			wantErr: false,
		},
	}

	for _, tt := range testsWithValues {
		t.Run(tt.name, func(t *testing.T) {
			data := make(map[string]interface{})
			data["helmVersion"] = tt.helmVersion
			gotErr := Validator(request, schema, data)
			assert.Equal(t, gotErr != nil, tt.wantErr)
		})
	}

	for _, tt := range testWithNoVersion {
		t.Run(tt.name, func(t *testing.T) {
			data := make(map[string]interface{})
			gotErr := Validator(request, schema, data)
			assert.Equal(t, gotErr != nil, tt.wantErr)
		})
	}
}

// setup api context for validation test
func testAsAPIContext() *types.APIContext {
	testAPIContext := &types.APIContext{}
	return testAPIContext
}

// setup schema for validation test
func testAsSchema() *types.Schema {
	testSchema := &types.Schema{}
	return testSchema
}
