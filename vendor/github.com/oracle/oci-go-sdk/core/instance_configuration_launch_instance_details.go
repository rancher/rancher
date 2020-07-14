// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Core Services API
//
// API covering the Networking (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/overview.htm),
// Compute (https://docs.cloud.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm), and
// Block Volume (https://docs.cloud.oracle.com/iaas/Content/Block/Concepts/overview.htm) services. Use this API
// to manage resources such as virtual cloud networks (VCNs), compute instances, and
// block storage volumes.
//

package core

import (
	"encoding/json"
	"github.com/oracle/oci-go-sdk/common"
)

// InstanceConfigurationLaunchInstanceDetails See Instance launch details - LaunchInstanceDetails
type InstanceConfigurationLaunchInstanceDetails struct {

	// The availability domain of the instance.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"false" json:"availabilityDomain"`

	// The OCID of the compartment.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// Details for the primary VNIC, which is automatically created and attached when
	// the instance is launched.
	CreateVnicDetails *InstanceConfigurationCreateVnicDetails `mandatory:"false" json:"createVnicDetails"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	// Example: `My bare metal instance`
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Additional metadata key/value pairs that you provide. They serve the same purpose and functionality as fields in the 'metadata' object.
	// They are distinguished from 'metadata' fields in that these can be nested JSON objects (whereas 'metadata' fields are string/string maps only).
	ExtendedMetadata map[string]interface{} `mandatory:"false" json:"extendedMetadata"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// This is an advanced option.
	// When a bare metal or virtual machine
	// instance boots, the iPXE firmware that runs on the instance is
	// configured to run an iPXE script to continue the boot process.
	// If you want more control over the boot process, you can provide
	// your own custom iPXE script that will run when the instance boots;
	// however, you should be aware that the same iPXE script will run
	// every time an instance boots; not only after the initial
	// LaunchInstance call.
	// The default iPXE script connects to the instance's local boot
	// volume over iSCSI and performs a network boot. If you use a custom iPXE
	// script and want to network-boot from the instance's local boot volume
	// over iSCSI the same way as the default iPXE script, you should use the
	// following iSCSI IP address: 169.254.0.2, and boot volume IQN:
	// iqn.2015-02.oracle.boot.
	// For more information about the Bring Your Own Image feature of
	// Oracle Cloud Infrastructure, see
	// Bring Your Own Image (https://docs.cloud.oracle.com/Content/Compute/References/bringyourownimage.htm).
	// For more information about iPXE, see http://ipxe.org.
	IpxeScript *string `mandatory:"false" json:"ipxeScript"`

	// Custom metadata key/value pairs that you provide, such as the SSH public key
	// required to connect to the instance.
	// A metadata service runs on every launched instance. The service is an HTTP
	// endpoint listening on 169.254.169.254. You can use the service to:
	// * Provide information to Cloud-Init (https://cloudinit.readthedocs.org/en/latest/)
	//   to be used for various system initialization tasks.
	// * Get information about the instance, including the custom metadata that you
	//   provide when you launch the instance.
	//  **Providing Cloud-Init Metadata**
	//  You can use the following metadata key names to provide information to
	//  Cloud-Init:
	//  **"ssh_authorized_keys"** - Provide one or more public SSH keys to be
	//  included in the `~/.ssh/authorized_keys` file for the default user on the
	//  instance. Use a newline character to separate multiple keys. The SSH
	//  keys must be in the format necessary for the `authorized_keys` file, as shown
	//  in the example below.
	//  **"user_data"** - Provide your own base64-encoded data to be used by
	//  Cloud-Init to run custom scripts or provide custom Cloud-Init configuration. For
	//  information about how to take advantage of user data, see the
	//  Cloud-Init Documentation (http://cloudinit.readthedocs.org/en/latest/topics/format.html).
	//  **Note:** Cloud-Init does not pull this data from the `http://169.254.169.254/opc/v1/instance/metadata/`
	//  path. When the instance launches and either of these keys are provided, the key values are formatted as
	//  OpenStack metadata and copied to the following locations, which are recognized by Cloud-Init:
	//  `http://169.254.169.254/openstack/latest/meta_data.json` - This JSON blob
	//  contains, among other things, the SSH keys that you provided for
	//   **"ssh_authorized_keys"**.
	//  `http://169.254.169.254/openstack/latest/user_data` - Contains the
	//  base64-decoded data that you provided for **"user_data"**.
	//  **Metadata Example**
	//       "metadata" : {
	//          "quake_bot_level" : "Severe",
	//          "ssh_authorized_keys" : "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQCZ06fccNTQfq+xubFlJ5ZR3kt+uzspdH9tXL+lAejSM1NXM+CFZev7MIxfEjas06y80ZBZ7DUTQO0GxJPeD8NCOb1VorF8M4xuLwrmzRtkoZzU16umt4y1W0Q4ifdp3IiiU0U8/WxczSXcUVZOLqkz5dc6oMHdMVpkimietWzGZ4LBBsH/LjEVY7E0V+a0sNchlVDIZcm7ErReBLcdTGDq0uLBiuChyl6RUkX1PNhusquTGwK7zc8OBXkRuubn5UKXhI3Ul9Nyk4XESkVWIGNKmw8mSpoJSjR8P9ZjRmcZVo8S+x4KVPMZKQEor== ryan.smith@company.com
	//          ssh-rsa AAAAB3NzaC1yc2EAAAABJQAAAQEAzJSAtwEPoB3Jmr58IXrDGzLuDYkWAYg8AsLYlo6JZvKpjY1xednIcfEVQJm4T2DhVmdWhRrwQ8DmayVZvBkLt+zs2LdoAJEVimKwXcJFD/7wtH8Lnk17HiglbbbNXsemjDY0hea4JUE5CfvkIdZBITuMrfqSmA4n3VNoorXYdvtTMoGG8fxMub46RPtuxtqi9bG9Zqenordkg5FJt2mVNfQRqf83CWojcOkklUWq4CjyxaeLf5i9gv1fRoBo4QhiA8I6NCSppO8GnoV/6Ox6TNoh9BiifqGKC9VGYuC89RvUajRBTZSK2TK4DPfaT+2R+slPsFrwiT/oPEhhEK1S5Q== rsa-key-20160227",
	//          "user_data" : "SWYgeW91IGNhbiBzZWUgdGhpcywgdGhlbiBpdCB3b3JrZWQgbWF5YmUuCg=="
	//       }
	//  **Getting Metadata on the Instance**
	//  To get information about your instance, connect to the instance using SSH and issue any of the
	//  following GET requests:
	//      curl http://169.254.169.254/opc/v1/instance/
	//      curl http://169.254.169.254/opc/v1/instance/metadata/
	//      curl http://169.254.169.254/opc/v1/instance/metadata/<any-key-name>
	//  You'll get back a response that includes all the instance information; only the metadata information; or
	//  the metadata information for the specified key name, respectively.
	Metadata map[string]string `mandatory:"false" json:"metadata"`

	// The shape of an instance. The shape determines the number of CPUs, amount of memory,
	// and other resources allocated to the instance.
	// You can enumerate all available shapes by calling ListShapes.
	Shape *string `mandatory:"false" json:"shape"`

	ShapeConfig *InstanceConfigurationLaunchInstanceShapeConfigDetails `mandatory:"false" json:"shapeConfig"`

	// Details for creating an instance.
	// Use this parameter to specify whether a boot volume or an image should be used to launch a new instance.
	SourceDetails InstanceConfigurationInstanceSourceDetails `mandatory:"false" json:"sourceDetails"`

	// A fault domain is a grouping of hardware and infrastructure within an availability domain.
	// Each availability domain contains three fault domains. Fault domains let you distribute your
	// instances so that they are not on the same physical hardware within a single availability domain.
	// A hardware failure or Compute hardware maintenance that affects one fault domain does not affect
	// instances in other fault domains.
	// If you do not specify the fault domain, the system selects one for you. To change the fault
	// domain for an instance, terminate it and launch a new instance in the preferred fault domain.
	// To get a list of fault domains, use the
	// ListFaultDomains operation in the
	// Identity and Access Management Service API.
	// Example: `FAULT-DOMAIN-1`
	FaultDomain *string `mandatory:"false" json:"faultDomain"`

	// The OCID of dedicated VM host.
	DedicatedVmHostId *string `mandatory:"false" json:"dedicatedVmHostId"`

	// Specifies the configuration mode for launching virtual machine (VM) instances. The configuration modes are:
	// * `NATIVE` - VM instances launch with iSCSI boot and VFIO devices. The default value for Oracle-provided images.
	// * `EMULATED` - VM instances launch with emulated devices, such as the E1000 network driver and emulated SCSI disk controller.
	// * `PARAVIRTUALIZED` - VM instances launch with paravirtualized devices using virtio drivers.
	// * `CUSTOM` - VM instances launch with custom configuration settings specified in the `LaunchOptions` parameter.
	LaunchMode InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum `mandatory:"false" json:"launchMode,omitempty"`

	LaunchOptions *InstanceConfigurationLaunchOptions `mandatory:"false" json:"launchOptions"`

	AgentConfig *InstanceConfigurationLaunchInstanceAgentConfigDetails `mandatory:"false" json:"agentConfig"`

	// Whether to enable in-transit encryption for the data volume's paravirtualized attachment. The default value is false.
	IsPvEncryptionInTransitEnabled *bool `mandatory:"false" json:"isPvEncryptionInTransitEnabled"`

	// The preferred maintenance action for an instance. The default is LIVE_MIGRATE, if live migration is supported.
	// * `LIVE_MIGRATE` - Run maintenance using a live migration.
	// * `REBOOT` - Run maintenance using a reboot.
	PreferredMaintenanceAction InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum `mandatory:"false" json:"preferredMaintenanceAction,omitempty"`
}

func (m InstanceConfigurationLaunchInstanceDetails) String() string {
	return common.PointerString(m)
}

// UnmarshalJSON unmarshals from json
func (m *InstanceConfigurationLaunchInstanceDetails) UnmarshalJSON(data []byte) (e error) {
	model := struct {
		AvailabilityDomain             *string                                                                  `json:"availabilityDomain"`
		CompartmentId                  *string                                                                  `json:"compartmentId"`
		CreateVnicDetails              *InstanceConfigurationCreateVnicDetails                                  `json:"createVnicDetails"`
		DefinedTags                    map[string]map[string]interface{}                                        `json:"definedTags"`
		DisplayName                    *string                                                                  `json:"displayName"`
		ExtendedMetadata               map[string]interface{}                                                   `json:"extendedMetadata"`
		FreeformTags                   map[string]string                                                        `json:"freeformTags"`
		IpxeScript                     *string                                                                  `json:"ipxeScript"`
		Metadata                       map[string]string                                                        `json:"metadata"`
		Shape                          *string                                                                  `json:"shape"`
		ShapeConfig                    *InstanceConfigurationLaunchInstanceShapeConfigDetails                   `json:"shapeConfig"`
		SourceDetails                  instanceconfigurationinstancesourcedetails                               `json:"sourceDetails"`
		FaultDomain                    *string                                                                  `json:"faultDomain"`
		DedicatedVmHostId              *string                                                                  `json:"dedicatedVmHostId"`
		LaunchMode                     InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum                 `json:"launchMode"`
		LaunchOptions                  *InstanceConfigurationLaunchOptions                                      `json:"launchOptions"`
		AgentConfig                    *InstanceConfigurationLaunchInstanceAgentConfigDetails                   `json:"agentConfig"`
		IsPvEncryptionInTransitEnabled *bool                                                                    `json:"isPvEncryptionInTransitEnabled"`
		PreferredMaintenanceAction     InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum `json:"preferredMaintenanceAction"`
	}{}

	e = json.Unmarshal(data, &model)
	if e != nil {
		return
	}
	var nn interface{}
	m.AvailabilityDomain = model.AvailabilityDomain

	m.CompartmentId = model.CompartmentId

	m.CreateVnicDetails = model.CreateVnicDetails

	m.DefinedTags = model.DefinedTags

	m.DisplayName = model.DisplayName

	m.ExtendedMetadata = model.ExtendedMetadata

	m.FreeformTags = model.FreeformTags

	m.IpxeScript = model.IpxeScript

	m.Metadata = model.Metadata

	m.Shape = model.Shape

	m.ShapeConfig = model.ShapeConfig

	nn, e = model.SourceDetails.UnmarshalPolymorphicJSON(model.SourceDetails.JsonData)
	if e != nil {
		return
	}
	if nn != nil {
		m.SourceDetails = nn.(InstanceConfigurationInstanceSourceDetails)
	} else {
		m.SourceDetails = nil
	}

	m.FaultDomain = model.FaultDomain

	m.DedicatedVmHostId = model.DedicatedVmHostId

	m.LaunchMode = model.LaunchMode

	m.LaunchOptions = model.LaunchOptions

	m.AgentConfig = model.AgentConfig

	m.IsPvEncryptionInTransitEnabled = model.IsPvEncryptionInTransitEnabled

	m.PreferredMaintenanceAction = model.PreferredMaintenanceAction
	return
}

// InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum Enum with underlying type: string
type InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum
const (
	InstanceConfigurationLaunchInstanceDetailsLaunchModeNative          InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum = "NATIVE"
	InstanceConfigurationLaunchInstanceDetailsLaunchModeEmulated        InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum = "EMULATED"
	InstanceConfigurationLaunchInstanceDetailsLaunchModeParavirtualized InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum = "PARAVIRTUALIZED"
	InstanceConfigurationLaunchInstanceDetailsLaunchModeCustom          InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum = "CUSTOM"
)

var mappingInstanceConfigurationLaunchInstanceDetailsLaunchMode = map[string]InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum{
	"NATIVE":          InstanceConfigurationLaunchInstanceDetailsLaunchModeNative,
	"EMULATED":        InstanceConfigurationLaunchInstanceDetailsLaunchModeEmulated,
	"PARAVIRTUALIZED": InstanceConfigurationLaunchInstanceDetailsLaunchModeParavirtualized,
	"CUSTOM":          InstanceConfigurationLaunchInstanceDetailsLaunchModeCustom,
}

// GetInstanceConfigurationLaunchInstanceDetailsLaunchModeEnumValues Enumerates the set of values for InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum
func GetInstanceConfigurationLaunchInstanceDetailsLaunchModeEnumValues() []InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum {
	values := make([]InstanceConfigurationLaunchInstanceDetailsLaunchModeEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchInstanceDetailsLaunchMode {
		values = append(values, v)
	}
	return values
}

// InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum Enum with underlying type: string
type InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum
const (
	InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionLiveMigrate InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum = "LIVE_MIGRATE"
	InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot      InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum = "REBOOT"
)

var mappingInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceAction = map[string]InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum{
	"LIVE_MIGRATE": InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionLiveMigrate,
	"REBOOT":       InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot,
}

// GetInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnumValues Enumerates the set of values for InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum
func GetInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnumValues() []InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum {
	values := make([]InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceAction {
		values = append(values, v)
	}
	return values
}
