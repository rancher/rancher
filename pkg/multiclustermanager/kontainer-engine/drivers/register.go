package drivers

import (
	"github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/drivers/aks"
	"github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/drivers/eks"
	"github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/drivers/gke"
	kubeimport "github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/drivers/import"
	"github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/drivers/rke"
	"github.com/rancher/rancher/pkg/multiclustermanager/kontainer-engine/types"
)

var Drivers map[string]types.Driver

func init() {
	Drivers = map[string]types.Driver{
		"googlekubernetesengine":        gke.NewDriver(),
		"azurekubernetesservice":        aks.NewDriver(),
		"amazonelasticcontainerservice": eks.NewDriver(),
		"import":                        kubeimport.NewDriver(),
		"rke":                           rke.NewDriver(),
	}
}
