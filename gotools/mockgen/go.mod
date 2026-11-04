module github.com/rancher/rancher/gotools/mockgen

go 1.27.0

toolchain go1.27.3

tool go.uber.org/mock/mockgen

require (
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
)

// gopkg.in/yaml.v3 is deprecated in favour of the maintained
// go.yaml.in/yaml successor modules.
replace gopkg.in/yaml.v3 => go.yaml.in/yaml/v3 v3.0.5
