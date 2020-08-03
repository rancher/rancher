package aks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_generateUniqueLogWorkspace(t *testing.T) {
	tests := []struct {
		name          string
		workspaceName string
		want          string
	}{
		{
			name:          "basic test",
			workspaceName: "ThisIsAValidInputklasjdfkljasjgireqahtawjsfklakjghrehtuirqhjfhwjkdfhjkawhfdjkhafvjkahg",
			want:          "ThisIsAValidInputklasjdfkljasjgireqahtawjsfkla-fb8fb22278d8eb98",
		},
	}
	for _, tt := range tests {
		got := generateUniqueLogWorkspace(tt.workspaceName)
		assert.Equal(t, tt.want, got)
		assert.Len(t, got, 63)
	}
}
