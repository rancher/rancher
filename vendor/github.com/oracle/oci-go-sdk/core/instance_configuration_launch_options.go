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
	"github.com/oracle/oci-go-sdk/common"
)

// InstanceConfigurationLaunchOptions Options for tuning compatibility and performance of VM shapes.
type InstanceConfigurationLaunchOptions struct {

	// Emulation type for volume.
	// * `ISCSI` - ISCSI attached block storage device. This is the default for Boot Volumes and Remote Block
	// Storage volumes on Oracle provided images.
	// * `SCSI` - Emulated SCSI disk.
	// * `IDE` - Emulated IDE disk.
	// * `VFIO` - Direct attached Virtual Function storage.  This is the default option for Local data
	// volumes on Oracle provided images.
	// * `PARAVIRTUALIZED` - Paravirtualized disk.
	BootVolumeType InstanceConfigurationLaunchOptionsBootVolumeTypeEnum `mandatory:"false" json:"bootVolumeType,omitempty"`

	// Firmware used to boot VM.  Select the option that matches your operating system.
	// * `BIOS` - Boot VM using BIOS style firmware.  This is compatible with both 32 bit and 64 bit operating
	// systems that boot using MBR style bootloaders.
	// * `UEFI_64` - Boot VM using UEFI style firmware compatible with 64 bit operating systems.  This is the
	// default for Oracle provided images.
	Firmware InstanceConfigurationLaunchOptionsFirmwareEnum `mandatory:"false" json:"firmware,omitempty"`

	// Emulation type for the physical network interface card (NIC).
	// * `E1000` - Emulated Gigabit ethernet controller.  Compatible with Linux e1000 network driver.
	// * `VFIO` - Direct attached Virtual Function network controller. This is the networking type
	// when you launch an instance using hardware-assisted (SR-IOV) networking.
	// * `PARAVIRTUALIZED` - VM instances launch with paravirtualized devices using virtio drivers.
	NetworkType InstanceConfigurationLaunchOptionsNetworkTypeEnum `mandatory:"false" json:"networkType,omitempty"`

	// Emulation type for volume.
	// * `ISCSI` - ISCSI attached block storage device. This is the default for Boot Volumes and Remote Block
	// Storage volumes on Oracle provided images.
	// * `SCSI` - Emulated SCSI disk.
	// * `IDE` - Emulated IDE disk.
	// * `VFIO` - Direct attached Virtual Function storage.  This is the default option for Local data
	// volumes on Oracle provided images.
	// * `PARAVIRTUALIZED` - Paravirtualized disk.
	RemoteDataVolumeType InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum `mandatory:"false" json:"remoteDataVolumeType,omitempty"`

	// Whether to enable in-transit encryption for the boot volume's paravirtualized attachment. The default value is false.
	IsPvEncryptionInTransitEnabled *bool `mandatory:"false" json:"isPvEncryptionInTransitEnabled"`

	// Whether to enable consistent volume naming feature. Defaults to false.
	IsConsistentVolumeNamingEnabled *bool `mandatory:"false" json:"isConsistentVolumeNamingEnabled"`
}

func (m InstanceConfigurationLaunchOptions) String() string {
	return common.PointerString(m)
}

// InstanceConfigurationLaunchOptionsBootVolumeTypeEnum Enum with underlying type: string
type InstanceConfigurationLaunchOptionsBootVolumeTypeEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchOptionsBootVolumeTypeEnum
const (
	InstanceConfigurationLaunchOptionsBootVolumeTypeIscsi           InstanceConfigurationLaunchOptionsBootVolumeTypeEnum = "ISCSI"
	InstanceConfigurationLaunchOptionsBootVolumeTypeScsi            InstanceConfigurationLaunchOptionsBootVolumeTypeEnum = "SCSI"
	InstanceConfigurationLaunchOptionsBootVolumeTypeIde             InstanceConfigurationLaunchOptionsBootVolumeTypeEnum = "IDE"
	InstanceConfigurationLaunchOptionsBootVolumeTypeVfio            InstanceConfigurationLaunchOptionsBootVolumeTypeEnum = "VFIO"
	InstanceConfigurationLaunchOptionsBootVolumeTypeParavirtualized InstanceConfigurationLaunchOptionsBootVolumeTypeEnum = "PARAVIRTUALIZED"
)

var mappingInstanceConfigurationLaunchOptionsBootVolumeType = map[string]InstanceConfigurationLaunchOptionsBootVolumeTypeEnum{
	"ISCSI":           InstanceConfigurationLaunchOptionsBootVolumeTypeIscsi,
	"SCSI":            InstanceConfigurationLaunchOptionsBootVolumeTypeScsi,
	"IDE":             InstanceConfigurationLaunchOptionsBootVolumeTypeIde,
	"VFIO":            InstanceConfigurationLaunchOptionsBootVolumeTypeVfio,
	"PARAVIRTUALIZED": InstanceConfigurationLaunchOptionsBootVolumeTypeParavirtualized,
}

// GetInstanceConfigurationLaunchOptionsBootVolumeTypeEnumValues Enumerates the set of values for InstanceConfigurationLaunchOptionsBootVolumeTypeEnum
func GetInstanceConfigurationLaunchOptionsBootVolumeTypeEnumValues() []InstanceConfigurationLaunchOptionsBootVolumeTypeEnum {
	values := make([]InstanceConfigurationLaunchOptionsBootVolumeTypeEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchOptionsBootVolumeType {
		values = append(values, v)
	}
	return values
}

// InstanceConfigurationLaunchOptionsFirmwareEnum Enum with underlying type: string
type InstanceConfigurationLaunchOptionsFirmwareEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchOptionsFirmwareEnum
const (
	InstanceConfigurationLaunchOptionsFirmwareBios   InstanceConfigurationLaunchOptionsFirmwareEnum = "BIOS"
	InstanceConfigurationLaunchOptionsFirmwareUefi64 InstanceConfigurationLaunchOptionsFirmwareEnum = "UEFI_64"
)

var mappingInstanceConfigurationLaunchOptionsFirmware = map[string]InstanceConfigurationLaunchOptionsFirmwareEnum{
	"BIOS":    InstanceConfigurationLaunchOptionsFirmwareBios,
	"UEFI_64": InstanceConfigurationLaunchOptionsFirmwareUefi64,
}

// GetInstanceConfigurationLaunchOptionsFirmwareEnumValues Enumerates the set of values for InstanceConfigurationLaunchOptionsFirmwareEnum
func GetInstanceConfigurationLaunchOptionsFirmwareEnumValues() []InstanceConfigurationLaunchOptionsFirmwareEnum {
	values := make([]InstanceConfigurationLaunchOptionsFirmwareEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchOptionsFirmware {
		values = append(values, v)
	}
	return values
}

// InstanceConfigurationLaunchOptionsNetworkTypeEnum Enum with underlying type: string
type InstanceConfigurationLaunchOptionsNetworkTypeEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchOptionsNetworkTypeEnum
const (
	InstanceConfigurationLaunchOptionsNetworkTypeE1000           InstanceConfigurationLaunchOptionsNetworkTypeEnum = "E1000"
	InstanceConfigurationLaunchOptionsNetworkTypeVfio            InstanceConfigurationLaunchOptionsNetworkTypeEnum = "VFIO"
	InstanceConfigurationLaunchOptionsNetworkTypeParavirtualized InstanceConfigurationLaunchOptionsNetworkTypeEnum = "PARAVIRTUALIZED"
)

var mappingInstanceConfigurationLaunchOptionsNetworkType = map[string]InstanceConfigurationLaunchOptionsNetworkTypeEnum{
	"E1000":           InstanceConfigurationLaunchOptionsNetworkTypeE1000,
	"VFIO":            InstanceConfigurationLaunchOptionsNetworkTypeVfio,
	"PARAVIRTUALIZED": InstanceConfigurationLaunchOptionsNetworkTypeParavirtualized,
}

// GetInstanceConfigurationLaunchOptionsNetworkTypeEnumValues Enumerates the set of values for InstanceConfigurationLaunchOptionsNetworkTypeEnum
func GetInstanceConfigurationLaunchOptionsNetworkTypeEnumValues() []InstanceConfigurationLaunchOptionsNetworkTypeEnum {
	values := make([]InstanceConfigurationLaunchOptionsNetworkTypeEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchOptionsNetworkType {
		values = append(values, v)
	}
	return values
}

// InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum Enum with underlying type: string
type InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum string

// Set of constants representing the allowable values for InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum
const (
	InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeIscsi           InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum = "ISCSI"
	InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeScsi            InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum = "SCSI"
	InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeIde             InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum = "IDE"
	InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeVfio            InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum = "VFIO"
	InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeParavirtualized InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum = "PARAVIRTUALIZED"
)

var mappingInstanceConfigurationLaunchOptionsRemoteDataVolumeType = map[string]InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum{
	"ISCSI":           InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeIscsi,
	"SCSI":            InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeScsi,
	"IDE":             InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeIde,
	"VFIO":            InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeVfio,
	"PARAVIRTUALIZED": InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeParavirtualized,
}

// GetInstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnumValues Enumerates the set of values for InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum
func GetInstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnumValues() []InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum {
	values := make([]InstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum, 0)
	for _, v := range mappingInstanceConfigurationLaunchOptionsRemoteDataVolumeType {
		values = append(values, v)
	}
	return values
}
