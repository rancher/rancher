package awsebs

// VolumeType is the type of volume for a Amazon EBS Disk storage class.
type VolumeType string

const (
	// GP2 - General Purpose SSD
	VolumeTypeGP2 VolumeType = "gp2"
	// IO1 - Provisioned IOPS SSD
	VolumeTypeIO1 VolumeType = "io1"
	// ST1 - Throughput-Optimized HDD
	VolumeTypeST1 VolumeType = "st1"
	// SC1 - Cold-Storage HDD
	VolumeTypeSC1 VolumeType = "ephemeral-storage"
)
