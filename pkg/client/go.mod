module github.com/rancher/rancher/pkg/client

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.18.8

require (
	github.com/rancher/norman v0.0.0-20211201154850-abe17976423e
	github.com/sirupsen/logrus v1.6.0 // indirect
	k8s.io/apimachinery v0.21.0
)
