//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -source=manager.go -destination=zz_manager_fakes.go -package=auth
package auth
