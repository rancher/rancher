package config

func (a *AWSConfig) SetAWSInstanceType(instanceType string) {
	a.AWSInstanceType = instanceType
}

func (a *AWSConfig) GetAWSInstanceType() string {
	return a.AWSInstanceType
}

func (a *AWSConfig) SetAWSRegion(region string) {
	a.AWSRegion = region
}

func (a *AWSConfig) GetAWSRegion() string {
	return a.AWSRegion
}

func (a *AWSConfig) SetAWSRegionAZ(regionAZ string) {
	a.AWSRegionAZ = regionAZ
}

func (a *AWSConfig) GetAWSRegionAZ() string {
	return a.AWSRegionAZ
}

func (a *AWSConfig) SetAWSAMI(ami string) {
	a.AWSAMI = ami
}

func (a *AWSConfig) GetAWSAMI() string {
	return a.AWSAMI
}

func (a *AWSConfig) SetAWSSecurityGroup(sg string) {
	a.AWSSecurityGroup = sg
}

func (a *AWSConfig) GetAWSSecurityGroup() string {
	return a.AWSSecurityGroup
}

func (a *AWSConfig) SetAWSAccessKeyID(keyID string) {
	a.AWSAccessKeyID = keyID
}

func (a *AWSConfig) GetAWSAccessKeyID() string {
	return a.AWSAccessKeyID
}

func (a *AWSConfig) SetAWSSecretAccessKey(secretKey string) {
	a.AWSSecretAccessKey = secretKey
}

func (a *AWSConfig) GetAWSSecretAccessKey() string {
	return a.AWSSecretAccessKey
}

func (a *AWSConfig) SetAWSSSHKeyName(sshKeyName string) {
	a.AWSSSHKeyName = sshKeyName
}

func (a *AWSConfig) GetAWSSSHKeyName() string {
	return a.AWSSSHKeyName
}

func (a *AWSConfig) SetAWSCICDInstanceTag(instanceTag string) {
	a.AWSCICDInstanceTag = instanceTag
}

func (a *AWSConfig) GetAWSCICDInstanceTag() string {
	return a.AWSCICDInstanceTag
}

func (a *AWSConfig) SetAWSIAMProfile(profile string) {
	a.AWSIAMProfile = profile
}

func (a *AWSConfig) GetAWSIAMProfile() string {
	return a.AWSIAMProfile
}

func (a *AWSConfig) SetAWSUser(user string) {
	a.AWSUser = user
}

func (a *AWSConfig) GetAWSUser() string {
	return a.AWSUser
}

func (a *AWSConfig) SetAWSVolumeSize(volumeSize int64) {
	a.AWSVolumeSize = volumeSize
}

func (a *AWSConfig) GetAWSVolumeSize() int64 {
	return a.AWSVolumeSize
}

func (a *AWSConfig) SetSSHPath(sshPath string) {
	a.AWSSSHKeyPath = sshPath
}

func (a *AWSConfig) GetSSHPath() string {
	return a.AWSSSHKeyPath
}
