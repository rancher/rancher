package image

import "encoding/json"

type KDMData struct {
	// K3S specific data, opaque and defined by the config file in kdm
	K3S map[string]interface{} `json:"k3s,omitempty"`
	// Rke2 specific data, defined by the config file in kdm
	RKE2 map[string]interface{} `json:"rke2,omitempty"`
}

func KDMFromData(b []byte) (KDMData, error) {
	d := &KDMData{}

	if err := json.Unmarshal(b, d); err != nil {
		return KDMData{}, err
	}
	return *d, nil
}
