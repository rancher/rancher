package common_test

import (
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"testing"
)

func TestEscapeUUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "valid guid string",
			arg:  "bfb34c007dc2c843adcc74ac3e27df21",
			want: "\\bf\\b3\\4c\\00\\7d\\c2\\c8\\43\\ad\\cc\\74\\ac\\3e\\27\\df\\21",
		},
		{
			name: "valid string",
			arg:  "abcdefghijklmnopqrstuvwxyz",
			want: "\\ab\\cd\\ef\\gh\\ij\\kl\\mn\\op\\qr\\st\\uv\\wx\\yz",
		},
		{
			name: "empty string",
			arg:  "",
			want: "\\",
		},
		{
			name: "odd length string",
			arg:  "abcde",
			want: "\\ab\\cd\\e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := common.EscapeUUID(tt.arg); got != tt.want {
				t.Errorf("EscapeUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}
