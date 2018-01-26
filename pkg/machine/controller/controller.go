package controller

import (
	"github.com/rancher/machine-controller/controller/machine"
	"github.com/rancher/machine-controller/controller/machinedriver"
	"github.com/rancher/types/config"
)

func Register(management *config.ManagementContext) {
	machine.Register(management)
	machinedriver.Register(management)
}
