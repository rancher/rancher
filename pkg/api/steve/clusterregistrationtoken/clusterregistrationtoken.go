package clusterregistrationtoken

import (
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/wrangler"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

func Register(server *steve.Server, clients *wrangler.Context) {
	secretCache := clients.Core.Secret().Cache()

	server.SchemaFactory.AddTemplate(schema2.Template{
		Group:     "management.cattle.io",
		Kind:      "ClusterRegistrationToken",
		Formatter: formatter(secretCache), // Formatter applies to all operations: GET, List, Watch
	})
}

var commandFields = []string{
	"command",
	"insecureCommand",
	"manifestUrl",
	"nodeCommand",
	"insecureNodeCommand",
	"windowsNodeCommand",
	"insecureWindowsNodeCommand",
}

func formatter(secretCache corecontrollers.SecretCache) types.Formatter {
	return func(request *types.APIRequest, resource *types.RawResource) {
		tokenSecretName := resource.APIObject.Data().String("status", "tokenSecretName")
		if tokenSecretName == "" {
			logrus.Tracef("[CRT steve] no tokenSecretName, clearing commands")
			clearCommands(resource)
			return
		}

		ns := resource.APIObject.Namespace()
		secret, err := secretCache.Get(ns, tokenSecretName)
		if err != nil {
			logrus.Tracef("[CRT steve] failed to get token secret %s: %v", tokenSecretName, err)
			clearCommands(resource)
			return
		}

		token := string(secret.Data["token"])
		if token == "" {
			logrus.Tracef("[CRT steve] empty token in secret %s, clearing commands", tokenSecretName)
			clearCommands(resource)
			return
		}

		resource.APIObject.Data().SetNested(token, "status", "token")

		for _, field := range commandFields {
			replaceTokenInField(resource, "status", field, token)
		}
	}
}

func replaceTokenInField(resource *types.RawResource, path1, path2, token string) {
	val := resource.APIObject.Data().String(path1, path2)
	if val != "" {
		replaced := strings.ReplaceAll(val, "{token}", token)
		resource.APIObject.Data().SetNested(replaced, path1, path2)
	}
}

func clearCommands(resource *types.RawResource) {
	for _, field := range commandFields {
		if resource.APIObject.Data().String("status", field) != "" {
			resource.APIObject.Data().SetNested("", "status", field)
		}
	}
}
