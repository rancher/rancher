package plugin

import (
	"github.com/rancher/kontainer-engine/drivers/aks"
	"github.com/rancher/kontainer-engine/drivers/gke"
	"github.com/rancher/kontainer-engine/drivers/import"
	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/types"
	"github.com/sirupsen/logrus"
)

var (
	// BuiltInDrivers includes all the buildin supported drivers
	BuiltInDrivers = map[string]bool{
		"gke":    true,
		"rke":    true,
		"aks":    true,
		"import": true,
	}
)

// Run starts a driver plugin in a go routine, and send its listen address back to addrChan
func Run(driverName string, addrChan chan string) (types.Driver, error) {
	var driver types.Driver
	switch driverName {
	case "gke":
		driver = gke.NewDriver()
	case "rke":
		driver = rke.NewDriver()
	case "aks":
		driver = aks.NewDriver()
	case "import":
		driver = kubeimport.NewDriver()
	default:
		addrChan <- ""
	}
	if BuiltInDrivers[driverName] {
		go types.NewServer(driver, addrChan).Serve()
		return driver, nil
	}
	logrus.Fatal("driver not supported")
	return driver, nil
}
