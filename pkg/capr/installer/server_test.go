package installer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func TestHandler_ServeHTTP(t *testing.T) {
	type scriptArgs struct {
		path string
		body string
	}
	tests := []struct {
		name string
		args scriptArgs
	}{
		{
			name: "Retrieve Linux script",
			args: scriptArgs{
				path: "/system-agent-install.sh",
				body: "#!/usr/bin/env sh",
			},
		},
		{
			name: "Retrieve Windows script",
			args: scriptArgs{
				path: "/wins-agent-install.ps1",
				body: "Invoke-WinsInstaller @PSBoundParameters",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			req, err := http.NewRequest(http.MethodGet, tt.args.path, nil)
			a.Nil(err)
			rr := httptest.NewRecorder()
			handler := handler{}

			// act
			err = settings.SystemAgentInstallScript.Set("https://raw.githubusercontent.com/rancher/system-agent/main/install.sh")
			a.Nil(err)
			err = settings.SystemAgentInstallerImage.Set("rancher/system-agent-installer-")
			a.Nil(err)

			handler.ServeHTTP(rr, req)

			// assert
			a.Equal(rr.Code, http.StatusOK)
			a.Contains(rr.Body.String(), tt.args.body)
		})
	}
}

func TestHandler_ServeHTTPAirgap(t *testing.T) {
	type scriptArgs struct {
		path string
		body string
	}
	tests := []struct {
		name string
		args scriptArgs
	}{
		{
			name: "Retrieve Linux script in mock airgap",
			args: scriptArgs{
				path: "/system-agent-install.sh",
				body: "#!/usr/bin/env sh",
			},
		},
		{
			name: "Retrieve Windows script in mock airgap",
			args: scriptArgs{
				path: "/wins-agent-install.ps1",
				body: "Invoke-WinsInstaller @PSBoundParameters",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			err := settings.SystemAgentInstallScript.Set("https://raw.githubusercontent.com/rancher/system-agent/main/install.sh")
			a.Nil(err)
			err = settings.SystemAgentInstallerImage.Set("rancher/system-agent-installer-")
			a.Nil(err)

			req, err := http.NewRequest(http.MethodGet, tt.args.path, nil)
			a.Nil(err)
			rr := httptest.NewRecorder()
			handler := handler{}
			air := httptest.NewUnstartedServer(&handler)

			// act
			air.Config.Handler.ServeHTTP(rr, req)
			defer air.Config.Close()

			// assert
			a.Equal(rr.Code, http.StatusOK)
			a.Contains(rr.Body.String(), tt.args.body)

		})
	}
}
