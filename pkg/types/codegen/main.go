package main

import (
	"os"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"

	"github.com/coreos/prometheus-operator/pkg/apis/monitoring"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/generator"

	"github.com/rancher/wrangler/pkg/controller-gen/args"
	appsv1 "k8s.io/api/apps/v1"
	scalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
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
					v3.ProjectCatalog{},
					v3.ClusterCatalog{},
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
		},
	})

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
	}, nil)
	generator.GenerateNativeTypes(batchv1.SchemeGroupVersion, []interface{}{
		batchv1.Job{},
	}, nil)
	generator.GenerateNativeTypes(batchv1beta1.SchemeGroupVersion, []interface{}{
		batchv1beta1.CronJob{},
	}, nil)
	generator.GenerateNativeTypes(extv1beta1.SchemeGroupVersion,
		[]interface{}{
			extv1beta1.Ingress{},
		},
		nil,
	)
	generator.GenerateNativeTypes(policyv1beta1.SchemeGroupVersion,
		nil,
		[]interface{}{
			policyv1beta1.PodSecurityPolicy{},
		},
	)
	generator.GenerateNativeTypes(storagev1.SchemeGroupVersion,
		nil,
		[]interface{}{
			storagev1.StorageClass{},
		},
	)
	generator.GenerateNativeTypes(
		k8sschema.GroupVersion{Group: monitoring.GroupName, Version: monitoringv1.Version},
		[]interface{}{
			monitoringv1.Prometheus{},
			monitoringv1.Alertmanager{},
			monitoringv1.PrometheusRule{},
			monitoringv1.ServiceMonitor{},
		},
		nil,
	)
	generator.GenerateNativeTypes(scalingv2beta2.SchemeGroupVersion,
		[]interface{}{
			scalingv2beta2.HorizontalPodAutoscaler{},
		},
		nil,
	)
	generator.GenerateNativeTypes(istiov1alpha3.SchemeGroupVersion,
		[]interface{}{
			istiov1alpha3.VirtualService{},
			istiov1alpha3.DestinationRule{},
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
