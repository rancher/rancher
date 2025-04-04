// This program generates the code for the Rancher types and clients.
package main

import (
	"os"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/codegen/generator"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	"github.com/rancher/rancher/pkg/schemas/factory"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	appsv1 "k8s.io/api/apps/v1"
	scalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func main() {
	os.Unsetenv("GOPATH")

	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/rancher/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"management.cattle.io": {
				PackageName: "management.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./pkg/apis/management.cattle.io/v3",
				},
				GenerateTypes: true,
			},
			"ui.cattle.io": {
				PackageName: "ui.cattle.io",
				Types: []interface{}{
					"./pkg/apis/ui.cattle.io/v1",
				},
				GenerateTypes: true,
			},
			"cluster.cattle.io": {
				PackageName: "cluster.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./pkg/apis/cluster.cattle.io/v3",
				},
				GenerateTypes: true,
			},
			"project.cattle.io": {
				PackageName: "project.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./pkg/apis/project.cattle.io/v3",
				},
				GenerateTypes: true,
			},
			"catalog.cattle.io": {
				PackageName: "catalog.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./pkg/apis/catalog.cattle.io/v1",
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"upgrade.cattle.io": {
				PackageName: "upgrade.cattle.io",
				Types: []interface{}{
					planv1.Plan{},
				},
				GenerateClients: true,
			},
			"provisioning.cattle.io": {
				Types: []interface{}{
					"./pkg/apis/provisioning.cattle.io/v1",
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"fleet.cattle.io": {
				Types: []interface{}{
					fleet.Bundle{},
					fleet.Cluster{},
					fleet.ClusterGroup{},
				},
			},
			"rke.cattle.io": {
				Types: []interface{}{
					"./pkg/apis/rke.cattle.io/v1",
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"cluster.x-k8s.io": {
				Types: []interface{}{
					capi.Machine{},
					capi.MachineSet{},
					capi.MachineDeployment{},
					capi.MachineHealthCheck{},
					capi.Cluster{},
				},
			},
			"auditlog.cattle.io": {
				PackageName: "auditlog.cattle.io",
				Types: []interface{}{
					"./pkg/apis/auditlog.cattle.io/v1",
				},
				GenerateTypes: true,
			},
			// This package is not a CRD but types from the extension API server.
			"ext.cattle.io": {
				PackageName: "ext.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./pkg/apis/ext.cattle.io/v1",
				},
				GenerateTypes:   true,
				GenerateOpenAPI: true,
				OpenAPIDependencies: []string{
					"k8s.io/apimachinery/pkg/apis/meta/v1",
					"k8s.io/apimachinery/pkg/runtime",
					"k8s.io/apimachinery/pkg/version",
				},
			},
		},
	})

	clusterAPIVersion := &types.APIVersion{Group: capi.GroupVersion.Group, Version: capi.GroupVersion.Version, Path: "/v1"}
	generator.GenerateClient(factory.Schemas(clusterAPIVersion).Init(func(schemas *types.Schemas) *types.Schemas {
		return schemas.MustImportAndCustomize(clusterAPIVersion, capi.Machine{}, func(schema *types.Schema) {
			schema.ID = "cluster.x-k8s.io.machine"
		})
	}), nil)

	generator.GenerateComposeType(projectSchema.Schemas, managementSchema.Schemas, clusterSchema.Schemas)
	generator.Generate(managementSchema.Schemas, map[string]bool{
		"userAttribute": true,
	})
	generator.GenerateClient(publicSchema.PublicSchemas, nil)
	generator.Generate(clusterSchema.Schemas, map[string]bool{
		"clusterUserAttribute": true,
		"clusterAuthToken":     true,
	})
	generator.Generate(projectSchema.Schemas, nil)
	generator.GenerateNativeTypes(v1.SchemeGroupVersion, []interface{}{
		v1.Endpoints{},
		v1.PersistentVolumeClaim{},
		v1.Pod{},
		v1.Service{},
		v1.Secret{},
		v1.ConfigMap{},
		v1.ServiceAccount{},
		v1.ReplicationController{},
		v1.ResourceQuota{},
		v1.LimitRange{},
	}, []interface{}{
		v1.Node{},
		v1.ComponentStatus{},
		v1.Namespace{},
		v1.Event{},
	})
	generator.GenerateNativeTypes(appsv1.SchemeGroupVersion, []interface{}{
		appsv1.Deployment{},
		appsv1.DaemonSet{},
		appsv1.StatefulSet{},
		appsv1.ReplicaSet{},
	}, nil)
	generator.GenerateNativeTypes(rbacv1.SchemeGroupVersion, []interface{}{
		rbacv1.RoleBinding{},
		rbacv1.Role{},
	}, []interface{}{
		rbacv1.ClusterRoleBinding{},
		rbacv1.ClusterRole{},
	})
	generator.GenerateNativeTypes(knetworkingv1.SchemeGroupVersion, []interface{}{
		knetworkingv1.NetworkPolicy{},
		knetworkingv1.Ingress{},
	}, nil)
	generator.GenerateNativeTypes(batchv1.SchemeGroupVersion, []interface{}{
		batchv1.Job{},
		batchv1.CronJob{},
	}, nil)
	generator.GenerateNativeTypes(extv1beta1.SchemeGroupVersion,
		[]interface{}{
			extv1beta1.Ingress{},
		},
		nil,
	)
	generator.GenerateNativeTypes(storagev1.SchemeGroupVersion,
		nil,
		[]interface{}{
			storagev1.StorageClass{},
		},
	)
	generator.GenerateNativeTypes(scalingv2.SchemeGroupVersion,
		[]interface{}{
			scalingv2.HorizontalPodAutoscaler{},
		},
		nil,
	)
	generator.GenerateNativeTypes(apiregistrationv1.SchemeGroupVersion,
		nil,
		[]interface{}{
			apiregistrationv1.APIService{},
		},
	)
}
