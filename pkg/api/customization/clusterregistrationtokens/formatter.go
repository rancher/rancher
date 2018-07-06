package clusterregistrationtokens

import (
	"fmt"
	"net/url"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rke/hosts"
)

const (
	commandFormat         = "kubectl apply -f %s"
	insecureCommandFormat = "curl --insecure -sfL %s | kubectl apply -f -"
	nodeCommandFormat     = "sudo docker run -d --privileged --restart=unless-stopped --net=host -v %s/etc/kubernetes:/etc/kubernetes -v /var/run:/var/run %s --server %s --token %s%s"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	ca := systemtemplate.CAChecksum()
	if ca != "" {
		ca = " --ca-checksum " + ca
	}

	token, _ := resource.Values["token"].(string)
	if token != "" {
		url := getURL(request, token)
		resource.Values["insecureCommand"] = fmt.Sprintf(insecureCommandFormat, url)
		resource.Values["command"] = fmt.Sprintf(commandFormat, url)
		resource.Values["nodeCommand"] = fmt.Sprintf(nodeCommandFormat,
			"",
			image.Resolve(settings.AgentImage.Get()),
			getRootURL(request),
			token,
			ca)
		resource.Values["token"] = token
		resource.Values["manifestUrl"] = url
	}
}

func NodeCommand(token, pathPrefix, os string) string {
	ca := systemtemplate.CAChecksum()
	if ca != "" {
		ca = " --ca-checksum " + ca
	}
	prefix := hosts.GetPrefixPath(os, pathPrefix)

	return fmt.Sprintf(nodeCommandFormat,
		prefix,
		image.Resolve(settings.AgentImage.Get()),
		getRootURL(nil),
		token,
		ca)
}

func getRootURL(request *types.APIContext) string {
	serverURL := settings.ServerURL.Get()
	if serverURL == "" {
		if request != nil {
			serverURL = request.URLBuilder.RelativeToRoot("")
		}
	} else {
		u, err := url.Parse(serverURL)
		if err != nil {
			if request != nil {
				serverURL = request.URLBuilder.RelativeToRoot("")
			}
		} else {
			u.Path = ""
			serverURL = u.String()
		}
	}

	return serverURL
}

func getURL(request *types.APIContext, token string) string {
	path := "/v3/import/" + token + ".yaml"
	serverURL := settings.ServerURL.Get()
	if serverURL == "" {
		serverURL = request.URLBuilder.RelativeToRoot(path)
	} else {
		u, err := url.Parse(serverURL)
		if err != nil {
			serverURL = request.URLBuilder.RelativeToRoot(path)
		} else {
			u.Path = path
			serverURL = u.String()
		}
	}

	return serverURL
}
