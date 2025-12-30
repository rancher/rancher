package node

import (
	"regexp"
)

var (
	RegExNodeDirEnv      = regexp.MustCompile("^" + nodeDirEnvKey + ".*")
	RegExNodePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")
	RegExNodeDriverName  = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")
)

const (
	nodeDirEnvKey = "MACHINE_STORAGE_PATH="
)
