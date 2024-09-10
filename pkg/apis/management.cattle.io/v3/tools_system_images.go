package v3

var (
	ToolsSystemImages = struct {
		AuthSystemImages AuthSystemImages
	}{
		AuthSystemImages: AuthSystemImages{
			KubeAPIAuth: "rancher/kube-api-auth:v0.2.1",
		},
	}
)
