package clusters

type Default struct {
	StringValue      string `json:"stringValue"`
	IntValue         int    `json:"intValue"`
	BoolValue        bool   `json:"boolValue"`
	StringSliceValue []int  `json:"stringSliceValue"`
}

type SSHUser struct {
	Type        string `json:"type"`
	Default     Default
	Create      bool   `json:"create"`
	Update      bool   `json:"update"`
	Description string `json:"description"`
}

type ResourceFields struct {
	SSHUser SSHUser
}

// DynamicSchemaSpec contains ResourceFields that contains all the data for the DynamicSchemaSpec which a type in provisioning.cattle.io.clusters this is how we get an ssh user for a node pool
type DynamicSchemaSpec struct {
	ResourceFields ResourceFields `json:"resourceFields"`
}
