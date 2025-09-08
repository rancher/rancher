package management

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
)

type KEv2OperatorInfo struct {
	Name          string `json:"name"`
	OldDriverName string `json:"oldDriverName"`
	Active        bool   `json:"active"`
}

var defaultKEv2Operators = map[string]KEv2OperatorInfo{
	"aks": {Name: "aks", OldDriverName: "azureKubernetesService", Active: true},
	"eks": {Name: "eks", OldDriverName: "amazonElasticContainerService", Active: true},
	"gke": {Name: "gke", OldDriverName: "googleElasticContainerService", Active: true},
}

func addKEv2OperatorSchemas(management *config.ManagementContext) error {
	creator := driverCreator{
		driversLister: management.Management.KontainerDrivers("").Controller().Lister(),
		drivers:       management.Management.KontainerDrivers(""),
	}
	return creator.syncKEv2OperatorsSetting()
}

func (c *driverCreator) syncKEv2OperatorsSetting() error {
	existingSettingJSON := settings.KEv2Operators.Get()

	existingOperators := map[string]KEv2OperatorInfo{}
	if existingSettingJSON != "" {
		if err := json.Unmarshal([]byte(existingSettingJSON), &existingOperators); err != nil {
			existingOperators = map[string]KEv2OperatorInfo{}
		}
	}

	settingChanged := false
	for key, operator := range defaultKEv2Operators {
		if _, found := existingOperators[key]; !found {
			existingOperators[key] = operator
			settingChanged = true
		}
	}

	for operatorKey, operatorInfo := range existingOperators {
		driver, err := c.driversLister.Get("", operatorInfo.OldDriverName)
		isActive := true
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		} else if driver != nil && !driver.Spec.Active {
			isActive = false
		}

		if operatorInfo.Active != isActive {
			operatorInfo.Active = isActive
			existingOperators[operatorKey] = operatorInfo
			settingChanged = true
		}
	}

	updatedSettingBytes, err := json.Marshal(existingOperators)
	if err != nil {
		return fmt.Errorf("error marshaling KEv2Operators: %w", err)
	}
	updatedSettingJSON := string(updatedSettingBytes)

	if existingSettingJSON == "" || settingChanged || !jsonEqual(existingSettingJSON, updatedSettingJSON) {
		settings.KEv2Operators.Set(updatedSettingJSON)
	}

	return nil
}

func jsonEqual(a, b string) bool {
	var objA, objB any
	if err := json.Unmarshal([]byte(a), &objA); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &objB); err != nil {
		return false
	}
	return reflect.DeepEqual(objA, objB)
}
