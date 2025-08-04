//go:generate go tool -modfile ../../../gotools/mockgen/go.mod mockgen -source=../stores/passwordchangerequest/store.go -destination=./passwordupdater.go -package=mocks

package mocks
