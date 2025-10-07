//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -source=../common/provider.go -destination=./provider.go -package=mocks
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -package mocks -source=../oidc/oidc_provider.go -destination=./tokenmanager.go
package mocks
