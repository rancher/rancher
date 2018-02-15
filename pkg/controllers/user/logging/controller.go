package logging

import (
	"github.com/rancher/types/config"
)

func Register(cluster *config.UserContext) {
	registerClusterLogging(cluster)
	registerProjectLogging(cluster)
}
