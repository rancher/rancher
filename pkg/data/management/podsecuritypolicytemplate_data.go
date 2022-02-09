package management

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const runAsAny = "RunAsAny"
const mustRunAs = "MustRunAs"
const mustRunAsNonRoot = "MustRunAsNonRoot"

func addDefaultPodSecurityPolicyTemplates(management *config.ManagementContext) error {
	pspts := management.Management.PodSecurityPolicyTemplates("")
	policies := []*v3.PodSecurityPolicyTemplate{
		{
			ObjectMeta: v12.ObjectMeta{
				Name: "unrestricted",
			},
			Spec: v1beta1.PodSecurityPolicySpec{
				Privileged:               true,
				AllowPrivilegeEscalation: toBoolPtr(true),
				AllowedCapabilities: []v1.Capability{
					"*",
				},
				Volumes: []v1beta1.FSType{
					"*",
				},
				HostNetwork: true,
				HostPorts: []v1beta1.HostPortRange{
					{Min: 0, Max: 65535},
				},
				HostIPC: true,
				HostPID: true,
				RunAsUser: v1beta1.RunAsUserStrategyOptions{
					Rule: runAsAny,
				},
				SELinux: v1beta1.SELinuxStrategyOptions{
					Rule: runAsAny,
				},
				SupplementalGroups: v1beta1.SupplementalGroupsStrategyOptions{
					Rule: runAsAny,
				},
				FSGroup: v1beta1.FSGroupStrategyOptions{
					Rule: runAsAny,
				},
			},
			Description: "This is the default unrestricted Pod Security Policy Template. It is the most permissive " +
				"Pod Security Policy that can be created in Kubernetes and is equivalent to running Kubernetes " +
				"without Pod Security Policies enabled.",
		},
		{
			ObjectMeta: v12.ObjectMeta{
				Name: "restricted",
			},
			Spec: v1beta1.PodSecurityPolicySpec{
				Privileged:               false,
				AllowPrivilegeEscalation: toBoolPtr(false),
				RequiredDropCapabilities: []v1.Capability{
					"ALL",
				},
				Volumes: []v1beta1.FSType{
					"configMap",
					"emptyDir",
					"projected",
					"secret",
					"downwardAPI",
					"persistentVolumeClaim",
				},
				HostNetwork: false,
				HostIPC:     false,
				HostPID:     false,
				RunAsUser: v1beta1.RunAsUserStrategyOptions{
					Rule: runAsAny,
				},
				SELinux: v1beta1.SELinuxStrategyOptions{
					Rule: runAsAny,
				},
				SupplementalGroups: v1beta1.SupplementalGroupsStrategyOptions{
					Rule: mustRunAs,
					Ranges: []v1beta1.IDRange{
						{Min: 1, Max: 65535},
					},
				},
				FSGroup: v1beta1.FSGroupStrategyOptions{
					Rule: mustRunAs,
					Ranges: []v1beta1.IDRange{
						{Min: 1, Max: 65535},
					},
				},
				ReadOnlyRootFilesystem: false,
			},
			Description: "This is the default restricted Pod Security Policy Template. It restricts many user " +
				"actions and does not allow privilege escalation.",
		},
		{
			ObjectMeta: v12.ObjectMeta{
				Name: "restricted-noroot",
			},
			Spec: v1beta1.PodSecurityPolicySpec{
				Privileged:               false,
				AllowPrivilegeEscalation: toBoolPtr(false),
				RequiredDropCapabilities: []v1.Capability{
					"ALL",
				},
				Volumes: []v1beta1.FSType{
					"configMap",
					"emptyDir",
					"projected",
					"secret",
					"downwardAPI",
					"csi",
					"persistentVolumeClaim",
				},
				HostNetwork: false,
				HostIPC:     false,
				HostPID:     false,
				RunAsUser: v1beta1.RunAsUserStrategyOptions{
					Rule: mustRunAsNonRoot,
				},
				SELinux: v1beta1.SELinuxStrategyOptions{
					Rule: runAsAny,
				},
				SupplementalGroups: v1beta1.SupplementalGroupsStrategyOptions{
					Rule: mustRunAs,
					Ranges: []v1beta1.IDRange{
						{Min: 1, Max: 65535},
					},
				},
				FSGroup: v1beta1.FSGroupStrategyOptions{
					Rule: mustRunAs,
					Ranges: []v1beta1.IDRange{
						{Min: 1, Max: 65535},
					},
				},
				ReadOnlyRootFilesystem: false,
			},
			Description: "This is the restricted Pod Security Policy Template to match upstream Kubernetes restricted PSP. " +
				"It restricts all user actions, does not allow privilege escalation and requires containers to run without root privileges.",
		},
	}
	for _, policy := range policies {
		_, err := pspts.Create(policy)
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating default pspt: %v", err)
		}
	}

	return nil
}

func toBoolPtr(boolean bool) *bool {
	return &boolean
}
