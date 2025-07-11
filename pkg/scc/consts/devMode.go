package consts

import "os"

var devMode = os.Getenv("CATTLE_DEV_MODE")

func IsDevMode() bool {
	return devMode != ""
}

func DevModeValue() string {
	return devMode
}
