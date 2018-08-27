package systemimage

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"

	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

var systemServices = make(map[string]SystemService)

type SystemService interface {
	Init(ctx context.Context, cluster *config.UserContext)
	Upgrade(currentVersion string) (newVersion string, err error)
	Version() (string, error)
}

func RegisterSystemService(name string, systemService SystemService) {
	if _, exists := systemServices[name]; exists {
		logrus.Errorf("system service '%s' tried to register twice", name)
	}
	systemServices[name] = systemService
}

func DefaultGetVersion(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal obj failed when get system image version: %v", err)
	}

	return fmt.Sprintf("%x", sha1.Sum(b))[:7], nil
}
