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

func formatter(secretCache corecontrollers.SecretCache) types.Formatter {
	return func(request *types.APIRequest, resource *types.RawResource) {
		tokenSecretName := resource.APIObject.Data().String("status", "tokenSecretName")
		if tokenSecretName == "" {
			return
		}

		ns := resource.APIObject.Namespace()
		secret, err := secretCache.Get(ns, tokenSecretName)
		if err != nil {
			logrus.Warnf("[CRT formatter] failed to get token secret %s/%s: %v", ns, tokenSecretName, err)
			return
		}

		token := string(secret.Data["token"])
		if token == "" {
			return
		}

		resource.APIObject.Data().SetNested(token, "status", "token")

		replaceTokenInField(resource, "status", "command", token)
		replaceTokenInField(resource, "status", "insecureCommand", token)
		replaceTokenInField(resource, "status", "manifestUrl", token)
		replaceTokenInField(resource, "status", "nodeCommand", token)
		replaceTokenInField(resource, "status", "insecureNodeCommand", token)
		replaceTokenInField(resource, "status", "windowsNodeCommand", token)
		replaceTokenInField(resource, "status", "insecureWindowsNodeCommand", token)
	}
}

func replaceTokenInField(resource *types.RawResource, path1, path2, token string) {
	val := resource.APIObject.Data().String(path1, path2)
	if val != "" {
		replaced := strings.ReplaceAll(val, "{token}", token)
		resource.APIObject.Data().SetNested(replaced, path1, path2)
	}
}
