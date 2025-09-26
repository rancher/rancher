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

var defaultKEv2Operators = []KEv2OperatorInfo{
	{Name: "aks", OldDriverName: "azureKubernetesService", Active: true},
	{Name: "eks", OldDriverName: "amazonElasticContainerService", Active: true},
	{Name: "gke", OldDriverName: "googleElasticContainerService", Active: true},
	{Name: "alibabacloud", OldDriverName: "", Active: false},
}

func syncOperatorDriverActiveState(management *config.ManagementContext) error {
	creator := driverCreator{
		driversLister: management.Management.KontainerDrivers("").Controller().Lister(),
		drivers:       management.Management.KontainerDrivers(""),
	}
	return creator.syncKEv2OperatorsSetting()
}

func (c *driverCreator) syncKEv2OperatorsSetting() error {
	existingSettingJSON := settings.KEv2Operators.Get()

	existingOperators := []KEv2OperatorInfo{}
	if existingSettingJSON != "{}" && existingSettingJSON != "" {
		if err := json.Unmarshal([]byte(existingSettingJSON), &existingOperators); err != nil {
			existingOperators = []KEv2OperatorInfo{}
		}
	}

	settingChanged := false

	// Add any missing default operators
	for _, operator := range defaultKEv2Operators {
		found := false
		for _, existing := range existingOperators {
			if existing.Name == operator.Name {
				found = true
				break
			}
		}
		if !found {
			existingOperators = append(existingOperators, operator)
			settingChanged = true
		}
	}

	// Update Active state based on old drivers
	for i, operatorInfo := range existingOperators {
		if operatorInfo.OldDriverName != "" {
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
				existingOperators[i].Active = isActive
				settingChanged = true
			}
		}
	}

	updatedSettingBytes, err := json.Marshal(existingOperators)
	if err != nil {
		return fmt.Errorf("error marshaling KEv2Operators: %w", err)
	}
	updatedSettingJSON := string(updatedSettingBytes)

	if existingSettingJSON == "{}" || settingChanged || !jsonEqual(existingSettingJSON, updatedSettingJSON) {
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
