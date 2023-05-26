package mocks

//go:generate mockgen --build_flags=--mod=mod -package mocks -destination ./clustercontroller.go github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1 ClusterController
//go:generate mockgen --build_flags=--mod=mod -package mocks -destination ./capi_clustercache.go github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1 ClusterCache
//go:generate mockgen --build_flags=--mod=mod -package mocks -destination ./capi_clusterclient.go github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1 ClusterClient
//go:generate mockgen --build_flags=--mod=mod -package mocks -destination ./rke2_controlplanecache.go github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1 RKEControlPlaneCache
