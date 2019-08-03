package catalog

import "testing"

func Test_containsIconURL(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			"base",
			`name: afc616
			version: 1.21.0
			appVersion: 6.9.0
			description: DataDog Agent
			keywords:
			- monitoring
			- alerting
			- metric
			home: https://www.datadoghq.com
			icon: https://raw.githubusercontent.com/matthewbelisle-wf/afc616/master/xss.svg
			sources:
			- https://app.datadoghq.com/account/settings#agent/kubernetes
			- https://github.com/DataDog/datadog-agent
			maintainers:
			- name: hkaj
			email: haissam@datadoghq.com
			- name: irabinovitch
			email: ilan@datadoghq.com
			- name: xvello
			email: xavier.vello@datadoghq.com
			- name: charlyf
			email: charly@datadoghq.com`,
			"https://raw.githubusercontent.com/matthewbelisle-wf/afc616/master/xss.svg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findIconURL(tt.s); got != tt.want {
				t.Errorf("containsIconURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
