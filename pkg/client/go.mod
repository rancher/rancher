module github.com/rancher/rancher/pkg/client

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.18.8

require (
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/rancher/norman v0.0.0-20200820172041-261460ee9088
	github.com/rancher/wrangler v0.6.2-0.20200820173016-2068de651106 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/stretchr/testify v1.5.1 // indirect
	golang.org/x/sys v0.0.0-20200323222414-85ca7c5b95cd // indirect
	k8s.io/apimachinery v0.18.8
)
