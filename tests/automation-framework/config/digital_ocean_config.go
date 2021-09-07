package config

func (d *DigitalOceanConfig) SetDOAccessKey(accessKey string) {
	d.DOAccessKey = accessKey
}

func (d *DigitalOceanConfig) GetDOAccessKey() string {
	return d.DOAccessKey
}

func (d *DigitalOceanConfig) SetDOImage(image string) {
	d.DOImage = image
}

func (d *DigitalOceanConfig) GetDOImage() string {
	return d.DOImage
}

func (d *DigitalOceanConfig) SetDORegion(doRegion string) {
	d.DORegion = doRegion
}

func (d *DigitalOceanConfig) GetDORegion() string {
	return d.DORegion
}

func (d *DigitalOceanConfig) SetDOSize(size string) {
	d.DOSize = size
}

func (d *DigitalOceanConfig) GetDOSize() string {
	return d.DOSize
}
