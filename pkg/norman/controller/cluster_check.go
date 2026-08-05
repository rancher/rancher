package controller

import (
	"os"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type ObjectClusterName interface {
	ObjClusterName() string
}

func ObjectInCluster(cluster string, obj interface{}) bool {
	// Check if the object implements the interface, this is best case and
	// what objects should strive to be
	if o, ok := obj.(ObjectClusterName); ok {
		clusterName := o.ObjClusterName()
		debugLogsClusterExtraction(clusterName, obj)
		return clusterName == cluster
	}

	// For types outside of rancher, attempt to check the anno, then use the namespace
	// This is much better than using the reflect hole below
	switch v := obj.(type) {
	case *corev1.Secret:
		if c, ok := v.Annotations["field.cattle.io/projectId"]; ok {
			if parts := strings.SplitN(c, ":", 2); len(parts) == 2 {
				return cluster == parts[0]
			}
		}
		return v.Namespace == cluster
	case *corev1.Namespace:
		if c, ok := v.Annotations["field.cattle.io/projectId"]; ok {
			if parts := strings.SplitN(c, ":", 2); len(parts) == 2 {
				return cluster == parts[0]
			}
		}
		return v.Namespace == cluster
	case *corev1.Node:
		if c, ok := v.Annotations["field.cattle.io/projectId"]; ok {
			if parts := strings.SplitN(c, ":", 2); len(parts) == 2 {
				return cluster == parts[0]
			}
		}
		return v.Namespace == cluster
	}

	// Seeing this message means something needs to be done with the type, see comments above
	if dm := os.Getenv("CATTLE_DEV_MODE"); dm != "" {
		logrus.Errorf("ObjectClusterName not implemented by type %T", obj)
	}

	var clusterName string

	if c := getValue(obj, "ClusterName"); c.IsValid() {
		clusterName = c.String()
	}
	if clusterName == "" {
		if c := getValue(obj, "Spec", "ClusterName"); c.IsValid() {
			clusterName = c.String()
		}

	}
	if clusterName == "" {
		if c := getValue(obj, "ProjectName"); c.IsValid() {
			if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
				clusterName = parts[0]
			}
		}
	}
	if clusterName == "" {
		if c := getValue(obj, "Spec", "ProjectName"); c.IsValid() {
			if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
				clusterName = parts[0]
			}
		}
	}
	if clusterName == "" {
		if a := getValue(obj, "Annotations"); a.IsValid() {
			if c := a.MapIndex(reflect.ValueOf("field.cattle.io/projectId")); c.IsValid() {
				if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
					clusterName = parts[0]
				}
			}
		}
	}
	if clusterName == "" {
		if c := getValue(obj, "Namespace"); c.IsValid() {
			clusterName = c.String()
		}
	}

	debugLogsClusterExtraction(clusterName, obj)
	return clusterName == cluster
}

func getValue(obj interface{}, name ...string) reflect.Value {
	v := reflect.ValueOf(obj)
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(name[0])
	if !field.IsValid() || len(name) == 1 {
		return field
	}

	return getFieldValue(field, name[1:]...)
}

func getFieldValue(v reflect.Value, name ...string) reflect.Value {
	field := v.FieldByName(name[0])
	if len(name) == 1 {
		return field
	}
	return getFieldValue(field, name[1:]...)
}

func debugLogsClusterExtraction(clusterName string, obj interface{}) {
	if clusterName != "" {
		return
	}
	ns := getValue(obj, "Namespace")
	name := getValue(obj, "Name")
	kind := getValue(obj, "Kind")
	logrus.Debugf("unable to extract cluster that the object [%s:%s] of kind [%s] belongs to", ns, name, kind)
}
