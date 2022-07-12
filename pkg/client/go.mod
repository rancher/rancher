module github.com/rancher/rancher/pkg/client

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.18.8

require (
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/rancher/norman v0.0.0-20220712163932-620fef760449
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/stretchr/testify v1.5.1 // indirect
	golang.org/x/sys v0.0.0-20200323222414-85ca7c5b95cd // indirect
	k8s.io/apimachinery v0.18.8
)
