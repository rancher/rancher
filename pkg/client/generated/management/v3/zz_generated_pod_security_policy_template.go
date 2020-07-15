package client

import (
	"github.com/rancher/norman/types"
)

const (
	PodSecurityPolicyTemplateType                                 = "podSecurityPolicyTemplate"
	PodSecurityPolicyTemplateFieldAllowPrivilegeEscalation        = "allowPrivilegeEscalation"
	PodSecurityPolicyTemplateFieldAllowedCSIDrivers               = "allowedCSIDrivers"
	PodSecurityPolicyTemplateFieldAllowedCapabilities             = "allowedCapabilities"
	PodSecurityPolicyTemplateFieldAllowedFlexVolumes              = "allowedFlexVolumes"
	PodSecurityPolicyTemplateFieldAllowedHostPaths                = "allowedHostPaths"
	PodSecurityPolicyTemplateFieldAllowedProcMountTypes           = "allowedProcMountTypes"
	PodSecurityPolicyTemplateFieldAllowedUnsafeSysctls            = "allowedUnsafeSysctls"
	PodSecurityPolicyTemplateFieldAnnotations                     = "annotations"
	PodSecurityPolicyTemplateFieldCreated                         = "created"
	PodSecurityPolicyTemplateFieldCreatorID                       = "creatorId"
	PodSecurityPolicyTemplateFieldDefaultAddCapabilities          = "defaultAddCapabilities"
	PodSecurityPolicyTemplateFieldDefaultAllowPrivilegeEscalation = "defaultAllowPrivilegeEscalation"
	PodSecurityPolicyTemplateFieldDescription                     = "description"
	PodSecurityPolicyTemplateFieldFSGroup                         = "fsGroup"
	PodSecurityPolicyTemplateFieldForbiddenSysctls                = "forbiddenSysctls"
	PodSecurityPolicyTemplateFieldHostIPC                         = "hostIPC"
	PodSecurityPolicyTemplateFieldHostNetwork                     = "hostNetwork"
	PodSecurityPolicyTemplateFieldHostPID                         = "hostPID"
	PodSecurityPolicyTemplateFieldHostPorts                       = "hostPorts"
	PodSecurityPolicyTemplateFieldLabels                          = "labels"
	PodSecurityPolicyTemplateFieldName                            = "name"
	PodSecurityPolicyTemplateFieldOwnerReferences                 = "ownerReferences"
	PodSecurityPolicyTemplateFieldPrivileged                      = "privileged"
	PodSecurityPolicyTemplateFieldReadOnlyRootFilesystem          = "readOnlyRootFilesystem"
	PodSecurityPolicyTemplateFieldRemoved                         = "removed"
	PodSecurityPolicyTemplateFieldRequiredDropCapabilities        = "requiredDropCapabilities"
	PodSecurityPolicyTemplateFieldRunAsGroup                      = "runAsGroup"
	PodSecurityPolicyTemplateFieldRunAsUser                       = "runAsUser"
	PodSecurityPolicyTemplateFieldRuntimeClass                    = "runtimeClass"
	PodSecurityPolicyTemplateFieldSELinux                         = "seLinux"
	PodSecurityPolicyTemplateFieldSupplementalGroups              = "supplementalGroups"
	PodSecurityPolicyTemplateFieldUUID                            = "uuid"
	PodSecurityPolicyTemplateFieldVolumes                         = "volumes"
)

type PodSecurityPolicyTemplate struct {
	types.Resource
	AllowPrivilegeEscalation        *bool                              `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	AllowedCSIDrivers               []AllowedCSIDriver                 `json:"allowedCSIDrivers,omitempty" yaml:"allowedCSIDrivers,omitempty"`
	AllowedCapabilities             []string                           `json:"allowedCapabilities,omitempty" yaml:"allowedCapabilities,omitempty"`
	AllowedFlexVolumes              []AllowedFlexVolume                `json:"allowedFlexVolumes,omitempty" yaml:"allowedFlexVolumes,omitempty"`
	AllowedHostPaths                []AllowedHostPath                  `json:"allowedHostPaths,omitempty" yaml:"allowedHostPaths,omitempty"`
	AllowedProcMountTypes           []string                           `json:"allowedProcMountTypes,omitempty" yaml:"allowedProcMountTypes,omitempty"`
	AllowedUnsafeSysctls            []string                           `json:"allowedUnsafeSysctls,omitempty" yaml:"allowedUnsafeSysctls,omitempty"`
	Annotations                     map[string]string                  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                         string                             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                       string                             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultAddCapabilities          []string                           `json:"defaultAddCapabilities,omitempty" yaml:"defaultAddCapabilities,omitempty"`
	DefaultAllowPrivilegeEscalation *bool                              `json:"defaultAllowPrivilegeEscalation,omitempty" yaml:"defaultAllowPrivilegeEscalation,omitempty"`
	Description                     string                             `json:"description,omitempty" yaml:"description,omitempty"`
	FSGroup                         *FSGroupStrategyOptions            `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	ForbiddenSysctls                []string                           `json:"forbiddenSysctls,omitempty" yaml:"forbiddenSysctls,omitempty"`
	HostIPC                         bool                               `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                     bool                               `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                         bool                               `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	HostPorts                       []HostPortRange                    `json:"hostPorts,omitempty" yaml:"hostPorts,omitempty"`
	Labels                          map[string]string                  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                            string                             `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences                 []OwnerReference                   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Privileged                      bool                               `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ReadOnlyRootFilesystem          bool                               `json:"readOnlyRootFilesystem,omitempty" yaml:"readOnlyRootFilesystem,omitempty"`
	Removed                         string                             `json:"removed,omitempty" yaml:"removed,omitempty"`
	RequiredDropCapabilities        []string                           `json:"requiredDropCapabilities,omitempty" yaml:"requiredDropCapabilities,omitempty"`
	RunAsGroup                      *RunAsGroupStrategyOptions         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsUser                       *RunAsUserStrategyOptions          `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RuntimeClass                    *RuntimeClassStrategyOptions       `json:"runtimeClass,omitempty" yaml:"runtimeClass,omitempty"`
	SELinux                         *SELinuxStrategyOptions            `json:"seLinux,omitempty" yaml:"seLinux,omitempty"`
	SupplementalGroups              *SupplementalGroupsStrategyOptions `json:"supplementalGroups,omitempty" yaml:"supplementalGroups,omitempty"`
	UUID                            string                             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Volumes                         []string                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}

type PodSecurityPolicyTemplateCollection struct {
	types.Collection
	Data   []PodSecurityPolicyTemplate `json:"data,omitempty"`
	client *PodSecurityPolicyTemplateClient
}

type PodSecurityPolicyTemplateClient struct {
	apiClient *Client
}

type PodSecurityPolicyTemplateOperations interface {
	List(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error)
	ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error)
	Create(opts *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Update(existing *PodSecurityPolicyTemplate, updates interface{}) (*PodSecurityPolicyTemplate, error)
	Replace(existing *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	ByID(id string) (*PodSecurityPolicyTemplate, error)
	Delete(container *PodSecurityPolicyTemplate) error
}

func newPodSecurityPolicyTemplateClient(apiClient *Client) *PodSecurityPolicyTemplateClient {
	return &PodSecurityPolicyTemplateClient{
		apiClient: apiClient,
	}
}

func (c *PodSecurityPolicyTemplateClient) Create(container *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoCreate(PodSecurityPolicyTemplateType, container, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Update(existing *PodSecurityPolicyTemplate, updates interface{}) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoUpdate(PodSecurityPolicyTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Replace(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoReplace(PodSecurityPolicyTemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) List(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error) {
	resp := &PodSecurityPolicyTemplateCollection{}
	err := c.apiClient.Ops.DoList(PodSecurityPolicyTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) ListAll(opts *types.ListOpts) (*PodSecurityPolicyTemplateCollection, error) {
	resp := &PodSecurityPolicyTemplateCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *PodSecurityPolicyTemplateCollection) Next() (*PodSecurityPolicyTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PodSecurityPolicyTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PodSecurityPolicyTemplateClient) ByID(id string) (*PodSecurityPolicyTemplate, error) {
	resp := &PodSecurityPolicyTemplate{}
	err := c.apiClient.Ops.DoByID(PodSecurityPolicyTemplateType, id, resp)
	return resp, err
}

func (c *PodSecurityPolicyTemplateClient) Delete(container *PodSecurityPolicyTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(PodSecurityPolicyTemplateType, &container.Resource)
}
