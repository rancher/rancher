package charts

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Project that charts are installed in
	gatekeeperProjectName = "gatekeeper-project"
	// namespace that is created without a label
	RancherDisallowedNamespace = "no-label"
)

var Constraint = schema.GroupVersionResource{
	Group:    "constraints.gatekeeper.sh",
	Version:  "v1beta1",
	Resource: "k8srequiredlabels",
}

var Namespaces = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "namespaces",
}

type Status struct {
	AuditTimestamp  string
	ByPod           interface{}
	TotalViolations int64
	Violations      []interface{}
}
type Items struct {
	ApiVersion string
	Kind       string
	Metadata   interface{}
	Spec       interface{}
	Status     Status
}

// ConstraintResponse is the data structure that is used to extract data about the gatekeeper constraint created in the test
// It contains the Items and Status structs, which are used for the same purpose
// anything that isn't being used in the test is declared as a string or an interface
type ConstraintResponse struct {
	ApiVersion string
	Items      []Items
	Kind       string
	Metadata   interface{}
}

// GetUnstructuredList helper function that returns an unstructured list of data from a cluster resource
func getUnstructuredList(client *rancher.Client, project *management.Project, schema schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(project.ClusterID)
	if err != nil {
		return nil, err
	}

	unstructured := dynamicClient.Resource(schema).Namespace("")

	unstructuredList, err := unstructured.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return unstructuredList, nil
}

// parseConstraintList converts an Unstructed List into a Constraint response by marshaling it into json, then unmarshaling the json into the Constraint respionse struct, allowing access to deeply nested fields
func parseConstraintList(list *unstructured.UnstructuredList) (*ConstraintResponse, error) {
	jsonConstraint, err := list.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var parsedConstraint ConstraintResponse
	err = json.Unmarshal([]byte(jsonConstraint), &parsedConstraint)
	if err != nil {
		return nil, err
	}

	return &parsedConstraint, nil

}

func getAuditTimestamp(client *rancher.Client, project *management.Project) {
	// wait until the first audit finishes running.
	// AuditTimestamp will be empty string until first audit finishes
	wait.Poll(1*time.Second, 5*time.Minute, func() (done bool, err error) {

		// get list of constraints
		auditList, err := getUnstructuredList(client, project, Constraint)
		if err != nil {
			return false, nil
		}

		// parse it so that we can extract individual values
		parsedAuditList, err := parseConstraintList(auditList)
		if err != nil {
			return false, nil
		}

		// extract the timestamp of the last constraint audit
		auditTime := parsedAuditList.Items[0].Status.AuditTimestamp
		if auditTime == "" {
			return false, nil
		}
		return true, nil
	})

}
