package utils

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "works http",
			url:     "http://example.com/abc?d=ef",
			wantErr: false,
		},
		{
			name:    "works git",
			url:     "git://example.com/abc?d=ef",
			wantErr: false,
		},
		{
			name: "cntrl error",
			url: "http://example.com/	abc",
			wantErr: true,
		},
		{
			name:    "urlencode error",
			url:     "git://example.com%0D/abc",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateURL(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
