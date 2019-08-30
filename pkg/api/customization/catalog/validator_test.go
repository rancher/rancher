package catalog

import (
	"testing"

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
