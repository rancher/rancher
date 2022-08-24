package users

import (
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const (
	defaultPasswordLength = 12
)

func GenerateUserPassword(password string) string {
	return namegenerator.RandStringLower(defaultPasswordLength)
}
