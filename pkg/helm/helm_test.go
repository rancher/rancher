package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_craftExtension(t *testing.T) {

	tests := []struct {
		name    string
		iconURL string
		want    string
	}{
		{
			"file case",
			"file://xss.svg",
			".svg",
		},
		{
			"web case",
			"https://raw.githubusercontent.com/JFrogDev/artifactory-dcos/master/images/jfrog_med.png",
			".png",
		},
		{
			"library file case",
			"file://charts/cert-manager/letsencrypt-logo-horizontal.svg",
			".svg",
		},
		{
			"empty case",
			"",
			"",
		},
		{
			"no suffix",
			"https://rawgithubusercontent.com/JFrogDev/artifactory-dcos/master/images/jfrog_med",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := craftExtension(tt.iconURL)
			assert.Equal(t, tt.want, got)
		})
	}
}
