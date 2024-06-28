//go:generate go run pkg/codegen/buildconfig/writer.go pkg/codegen/buildconfig/main.go
//go:generate scripts/configure-drone
//go:generate go run pkg/codegen/generator/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate scripts/build-crds --quiet
package main
