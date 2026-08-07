package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentsFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		names []string
		value string
	}{
		{
			name:  "split long option",
			args:  []string{"server", "--data-dir", "/custom/rke2"},
			names: []string{"--data-dir", "-d"},
			value: "/custom/rke2",
		},
		{
			name:  "combined short option",
			args:  []string{"-d=/custom/rke2"},
			names: []string{"--data-dir", "-d"},
			value: "/custom/rke2",
		},
		{
			name:  "first option wins across aliases",
			args:  []string{"-d", "/first", "--data-dir=/second"},
			names: []string{"--data-dir", "-d"},
			value: "/first",
		},
		{
			name:  "missing option value",
			args:  []string{"--data-dir"},
			names: []string{"--data-dir", "-d"},
		},
		{
			name:  "does not match option prefix",
			args:  []string{"--data-directory=/custom/rke2"},
			names: []string{"--data-dir", "-d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, NewArguments(tt.args).First(tt.names...))
		})
	}
}

func TestArgumentsValues(t *testing.T) {
	t.Parallel()

	args := NewArguments([]string{
		"--kube-scheduler-arg", "secure-port=10262",
		"--kube-scheduler-arg=tls-cert-file=/custom/scheduler.crt",
		"--kube-controller-manager-arg", "secure-port=10261",
		"--kube-scheduler-arg", "tls-private-key-file=/custom/scheduler.key",
		"--kube-scheduler-args=not-a-match",
		"--kube-scheduler-arg",
	})

	assert.Equal(t, []string{
		"secure-port=10262",
		"tls-cert-file=/custom/scheduler.crt",
		"tls-private-key-file=/custom/scheduler.key",
	}, args.Values("--kube-scheduler-arg"))
	assert.Nil(t, args.Values("--missing"))
}
