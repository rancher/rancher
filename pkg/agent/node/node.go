package node

import (
	"os"
)

func TokenAndURL() (string, string, error) {
	return os.Getenv("CATTLE_TOKEN"), os.Getenv("CATTLE_SERVER"), nil
}
