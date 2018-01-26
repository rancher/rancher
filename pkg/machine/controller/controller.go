package controller

import (
	"github.com/rancher/rancher/pkg/machine/controller/machine"
	"github.com/rancher/rancher/pkg/machine/controller/machinedriver"
	"github.com/rancher/types/config"
)

func Register(management *config.ManagementContext) {
	machine.Register(management)
	machinedriver.Register(management)
}
