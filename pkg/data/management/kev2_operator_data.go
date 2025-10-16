package management

import (
	"encoding/json"
	"fmt"
	"sort"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	EKSOperator     = "eks"
	GKEOperator     = "gke"
	AKSOperator     = "aks"
	AlibabaOperator = "alibaba"
)

var Kev2ToKontainerDriver = map[string]string{
	EKSOperator: "amazonelasticcontainerservice",
	AKSOperator: "azurekubernetesservice",
	GKEOperator: "googlekubernetesengine",
}

type KEv2OperatorInfo struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

var defaultKEv2Operators = []KEv2OperatorInfo{
	{Name: AKSOperator, Active: true},
	{Name: EKSOperator, Active: true},
	{Name: GKEOperator, Active: true},
	{Name: AlibabaOperator, Active: false},
}

// GetDefaultKEv2Operators returns a copy of the default KEv2 operators.
func GetDefaultKEv2Operators() []KEv2OperatorInfo {
	// return a copy to prevent modification
	defaults := make([]KEv2OperatorInfo, len(defaultKEv2Operators))
	copy(defaults, defaultKEv2Operators)
	return defaults
}

func syncKEv2Operators(management *config.ManagementContext) error {
	return syncKEv2OperatorsSetting(management.Management.KontainerDrivers("").Controller().Lister())
}

func syncKEv2OperatorsSetting(driversLister v3.KontainerDriverLister) error {
	existingVal := settings.KEv2Operators.Get()
	settingsChanged := false

	updatedValueData := []KEv2OperatorInfo{}
	if existingVal != "{}" && existingVal != "" {
		if err := json.Unmarshal([]byte(existingVal), &updatedValueData); err != nil {
			// if the setting is corrupt, log it and reset to defaults
			logrus.Warnf("failed to unmarshal KEv2Operators setting, will reset to default: %v", err)
		}
	}

	// Add any missing default operators
	for _, operator := range defaultKEv2Operators {
		found := false
		for _, existing := range updatedValueData {
			if existing.Name == operator.Name {
				found = true
				break
			}
		}
		if !found {
			updatedValueData = append(updatedValueData, operator)
			settingsChanged = true
		}
	}

	// Update Active state based on old drivers
	for i, operatorInfo := range updatedValueData {
		if oldDriverName, ok := Kev2ToKontainerDriver[operatorInfo.Name]; ok {
			driver, err := driversLister.Get("", oldDriverName)
			if err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return err
			}
			if driver != nil && updatedValueData[i].Active != driver.Spec.Active {
				updatedValueData[i].Active = driver.Spec.Active
				settingsChanged = true
			}
		}
	}

	if settingsChanged {
		if len(updatedValueData) > 0 {
			// sorting updatedValueData
			sort.Slice(updatedValueData, func(first, second int) bool {
				return updatedValueData[first].Name < updatedValueData[second].Name
			})
		}

		updatedSettingBytes, err := json.Marshal(updatedValueData)
		if err != nil {
			return fmt.Errorf("error marshaling KEv2Operators setting value: %w", err)
		}

		return settings.KEv2Operators.Set(string(updatedSettingBytes))
	}

	return nil
}

// GetDefaultKEv2Operators returns a copy of the default KEv2 operators.
func GetKEv2OperatorsSettingData() []KEv2OperatorInfo {
	existingVal := settings.KEv2Operators.Get()
	data := []KEv2OperatorInfo{}
	if existingVal != "{}" && existingVal != "" {
		if err := json.Unmarshal([]byte(existingVal), &data); err != nil {
			// setting is corrupt so returning default kev2 operators data to not block other operations.
			return GetDefaultKEv2Operators()
		}
	}
	return data
}
