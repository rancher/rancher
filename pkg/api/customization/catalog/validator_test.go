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
		want    string
	}{
		{
			name:    "Remove control characters",
			pathURL: "http://example.com/1\r2\n345\b67\t",
			want:    "http://example.com/1234567",
		},
		{
			name:    "Remove urlEncoded control characters",
			pathURL: "https://example.com/12%003%1F45%0A%0a6",
			want:    "https://example.com/123456",
		},
		{
			name:    "Remove all control characters, allow uppercase scheme",
			pathURL: "https://www.example%0D.com/Hello\r\nWorld",
			want:    "https://www.example.com/HelloWorld",
		},
		{
			name:    "Allow git protocol",
			pathURL: "git://www.example.com",
			want:    "git://www.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeURL(tt.pathURL)
			assert.Equal(t, got, tt.want)
		})
	}
}
