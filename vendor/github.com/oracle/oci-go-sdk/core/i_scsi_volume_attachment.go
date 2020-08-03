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

// IScsiVolumeAttachment An ISCSI volume attachment.
type IScsiVolumeAttachment struct {

	// The availability domain of an instance.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"true" json:"availabilityDomain"`

	// The OCID of the compartment.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The OCID of the volume attachment.
	Id *string `mandatory:"true" json:"id"`

	// The OCID of the instance the volume is attached to.
	InstanceId *string `mandatory:"true" json:"instanceId"`

	// The date and time the volume was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The OCID of the volume.
	VolumeId *string `mandatory:"true" json:"volumeId"`

	// The volume's iSCSI IP address.
	// Example: `169.254.0.2`
	Ipv4 *string `mandatory:"true" json:"ipv4"`

	// The target volume's iSCSI Qualified Name in the format defined by RFC 3720.
	// Example: `iqn.2015-12.us.oracle.com:456b0391-17b8-4122-bbf1-f85fc0bb97d9`
	Iqn *string `mandatory:"true" json:"iqn"`

	// The volume's iSCSI port.
	// Example: `3260`
	Port *int `mandatory:"true" json:"port"`

	// The device name.
	Device *string `mandatory:"false" json:"device"`

	// A user-friendly name. Does not have to be unique, and it cannot be changed.
	// Avoid entering confidential information.
	// Example: `My volume attachment`
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Whether the attachment was created in read-only mode.
	IsReadOnly *bool `mandatory:"false" json:"isReadOnly"`

	// Whether the attachment should be created in shareable mode. If an attachment is created in shareable mode, then other instances can attach the same volume, provided that they also create their attachments in shareable mode. Only certain volume types can be attached in shareable mode. Defaults to false if not specified.
	IsShareable *bool `mandatory:"false" json:"isShareable"`

	// Whether in-transit encryption for the data volume's paravirtualized attachment is enabled or not.
	IsPvEncryptionInTransitEnabled *bool `mandatory:"false" json:"isPvEncryptionInTransitEnabled"`

	// The Challenge-Handshake-Authentication-Protocol (CHAP) secret valid for the associated CHAP user name.
	// (Also called the "CHAP password".)
	// Example: `d6866c0d-298b-48ba-95af-309b4faux45e`
	ChapSecret *string `mandatory:"false" json:"chapSecret"`

	// The volume's system-generated Challenge-Handshake-Authentication-Protocol (CHAP) user name.
	// Example: `ocid1.volume.oc1.phx.abyhqljrgvttnlx73nmrwfaux7kcvzfs3s66izvxf2h4lgvyndsdsnoiwr5q`
	ChapUsername *string `mandatory:"false" json:"chapUsername"`

	// The current state of the volume attachment.
	LifecycleState VolumeAttachmentLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`
}

//GetAvailabilityDomain returns AvailabilityDomain
func (m IScsiVolumeAttachment) GetAvailabilityDomain() *string {
	return m.AvailabilityDomain
}

//GetCompartmentId returns CompartmentId
func (m IScsiVolumeAttachment) GetCompartmentId() *string {
	return m.CompartmentId
}

//GetDevice returns Device
func (m IScsiVolumeAttachment) GetDevice() *string {
	return m.Device
}

//GetDisplayName returns DisplayName
func (m IScsiVolumeAttachment) GetDisplayName() *string {
	return m.DisplayName
}

//GetId returns Id
func (m IScsiVolumeAttachment) GetId() *string {
	return m.Id
}

//GetInstanceId returns InstanceId
func (m IScsiVolumeAttachment) GetInstanceId() *string {
	return m.InstanceId
}

//GetIsReadOnly returns IsReadOnly
func (m IScsiVolumeAttachment) GetIsReadOnly() *bool {
	return m.IsReadOnly
}

//GetIsShareable returns IsShareable
func (m IScsiVolumeAttachment) GetIsShareable() *bool {
	return m.IsShareable
}

//GetLifecycleState returns LifecycleState
func (m IScsiVolumeAttachment) GetLifecycleState() VolumeAttachmentLifecycleStateEnum {
	return m.LifecycleState
}

//GetTimeCreated returns TimeCreated
func (m IScsiVolumeAttachment) GetTimeCreated() *common.SDKTime {
	return m.TimeCreated
}

//GetVolumeId returns VolumeId
func (m IScsiVolumeAttachment) GetVolumeId() *string {
	return m.VolumeId
}

//GetIsPvEncryptionInTransitEnabled returns IsPvEncryptionInTransitEnabled
func (m IScsiVolumeAttachment) GetIsPvEncryptionInTransitEnabled() *bool {
	return m.IsPvEncryptionInTransitEnabled
}

func (m IScsiVolumeAttachment) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m IScsiVolumeAttachment) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeIScsiVolumeAttachment IScsiVolumeAttachment
	s := struct {
		DiscriminatorParam string `json:"attachmentType"`
		MarshalTypeIScsiVolumeAttachment
	}{
		"iscsi",
		(MarshalTypeIScsiVolumeAttachment)(m),
	}

	return json.Marshal(&s)
}
