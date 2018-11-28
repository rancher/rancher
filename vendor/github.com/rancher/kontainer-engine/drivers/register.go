package drivers

import (
	"github.com/rancher/kontainer-engine/drivers/aks"
	"github.com/rancher/kontainer-engine/drivers/eks"
	"github.com/rancher/kontainer-engine/drivers/gke"
	"github.com/rancher/kontainer-engine/drivers/import"
	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/types"
)

var Drivers map[string]types.Driver

func init() {
	Drivers = map[string]types.Driver{
		"gke":    gke.NewDriver(),
		"aks":    aks.NewDriver(),
		"eks":    eks.NewDriver(),
		"import": kubeimport.NewDriver(),
		"rke":    rke.NewDriver(),
	}
}
