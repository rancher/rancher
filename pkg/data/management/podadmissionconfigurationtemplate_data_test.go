package management

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	example1 = &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-example1",
		},
		Description: "This is an example for testing",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: v3.PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "restricted",
				EnforceVersion: "latest",
				Audit:          "restricted",
				AuditVersion:   "latest",
				Warn:           "restricted",
				WarnVersion:    "latest",
			},
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{"user-a", "user-b", "user-c"},
				RuntimeClasses: []string{"runtime-a", "runtime-b", "runtime-c"},
				Namespaces:     []string{"ns-a", "ns-b", "ns-c"},
			},
		},
	}
	example2 = &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "another-example",
		},
		Description: "This is another example for testing",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{"user-a", "user-1", "user-2", "user-3"},
				RuntimeClasses: []string{"runtime-1", "runtime-2", "runtime-3"},
				Namespaces:     []string{"ns-1", "ns-2", "ns-3"},
			},
		},
	}
	example1plug2 = &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-example1",
		},
		Description: "This is an example for testing",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: v3.PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "restricted",
				EnforceVersion: "latest",
				Audit:          "restricted",
				AuditVersion:   "latest",
				Warn:           "restricted",
				WarnVersion:    "latest",
			},
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{"user-1", "user-2", "user-3", "user-a", "user-b", "user-c"},
				RuntimeClasses: []string{"runtime-1", "runtime-2", "runtime-3", "runtime-a", "runtime-b", "runtime-c"},
				Namespaces:     []string{"ns-1", "ns-2", "ns-3", "ns-a", "ns-b", "ns-c"},
			},
		},
	}
)

func Test_merge(t *testing.T) {
	type args struct {
		base       *v3.PodSecurityAdmissionConfigurationTemplate
		additional *v3.PodSecurityAdmissionConfigurationTemplate
	}
	tests := []struct {
		name string
		args args
		want *v3.PodSecurityAdmissionConfigurationTemplate
	}{
		{
			name: "base is empty",
			args: args{
				base:       nil,
				additional: example1,
			},
			want: example1,
		},
		{
			name: "additional is empty",
			args: args{
				base:       example1,
				additional: nil,
			},
			want: example1,
		},
		{
			name: "both are empty",
			args: args{
				base:       nil,
				additional: nil,
			},
			want: nil,
		},
		{
			name: "addition contains different exceptions",
			args: args{
				base:       example1,
				additional: example2,
			},
			want: example1plug2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, mergeExemptions(tt.args.base, tt.args.additional), "mergeExemptions(%v, %v)", tt.args.base, tt.args.additional)
		})
	}
}
