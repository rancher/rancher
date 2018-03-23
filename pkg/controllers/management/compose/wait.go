package compose

import (
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	managementClient "github.com/rancher/types/client/management/v3"
)

var (
	// WaitCondition is a set of function that can be customized to wait for a resource
	WaitCondition = map[string]func(baseClient *clientbase.APIBaseClient, id, schemaType string) error{}
)

func WaitCluster(baseClient *clientbase.APIBaseClient, id, schemaType string) error {
	start := time.Now()
	for {
		respObj := managementClient.Cluster{}
		if err := baseClient.ByID(schemaType, id, &respObj); err != nil {
			return err
		}
		for _, cond := range respObj.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				return nil
			}
		}
		time.Sleep(time.Second * 10)
		if time.Now().After(start.Add(time.Minute * 30)) {
			return errors.Errorf("Timeout wait for cluster %s to be ready", id)
		}
	}
}
