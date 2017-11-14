package plugin

import (
	rpcDriver "github.com/rancher/kontainer-engine/driver"
	"github.com/rancher/kontainer-engine/driver/gke"
	"github.com/rancher/kontainer-engine/driver/rke"
	"github.com/sirupsen/logrus"
)

var (
	// BuiltInDrivers includes all the buildin supported drivers
	BuiltInDrivers = map[string]bool{
		"gke": true,
		"rke": true,
	}
)

// Run starts a driver plugin in a go routine, and send its listen address back to addrChan
func Run(driverName string, addrChan chan string) error {
	var driver rpcDriver.Driver
	switch driverName {
	case "gke":
		driver = gke.NewDriver()
	case "rke":
		driver = rke.NewDriver()
	default:
		addrChan <- ""
	}
	if BuiltInDrivers[driverName] {
		go startRPCServer(rpcDriver.NewServer(driver, addrChan))
		return nil
	}
	logrus.Fatal("driver not supported")
	return nil
}

func startRPCServer(server rpcDriver.RPCServer) {
	server.Serve()
}
