//go:generate go tool -modfile ../../gotools/mockgen/go.mod mockgen -source=manager.go -destination=zz_manager_fake.go -package=user
package mocks
