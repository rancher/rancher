//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen --build_flags=--mod=mod -package rbac -destination ./v3mgmntMocks_test.go github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3 ClusterInterface
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen --build_flags=--mod=mod -package rbac -destination ./v1rbacMocks_test.go github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1 ClusterRoleBindingInterface,ClusterRoleBindingLister
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -package=rbac -destination=v1rbacMocks_test.go github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1 ClusterRoleBindingInterface,ClusterRoleBindingLister
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -package=rbac -destination=v3mgmntMocks_test.go github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3 ClusterInterface
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -source handler_base.go -destination=zz_manager_fakes.go -package=rbac
package rbac
