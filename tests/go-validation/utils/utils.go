package utils

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/client-go/rest"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

var Host string = os.Getenv("CATTLE_TEST_URL")
var Token string = os.Getenv("ADMIN_TOKEN")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewRestConfig() *rest.Config {
	return &rest.Config{
		Host:        Host,
		BearerToken: Token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func RancherCleanup() bool {
	rancherCleanupString := os.Getenv("RANCHER_CLEANUP")
	if rancherCleanupString == "false" || rancherCleanupString == "" {
		return false
	}

	return true
}
