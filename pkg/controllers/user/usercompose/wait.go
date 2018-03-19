package usercompose

import "github.com/rancher/norman/clientbase"

var (
	// WaitCondition is a set of function that can be customized to wait for a resource
	WaitCondition = map[string]func(baseClient *clientbase.APIBaseClient, id, schemaType string) error{}
)
