module github.com/rancher/rancher/pkg/client

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.18.8

require (
	github.com/rancher/norman v0.0.0-20211201154850-abe17976423e
	github.com/sirupsen/logrus v1.6.0 // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	gopkg.in/yaml.v3 v3.0.0 // indirect
	k8s.io/apimachinery v0.21.0
)
