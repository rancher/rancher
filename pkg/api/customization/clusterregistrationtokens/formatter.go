package clusterregistrationtokens

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

const (
	commandFormat         = "kubectl apply -f %s"
	insecureCommandFormat = "curl --insecure -sfL %s | kubectl apply -f -"
	nodeCommandFormat     = "docker run -d --restart=unless-stopped --net=host -v /etc/kubernetes/ssl:/etc/kubernetes/ssl -v /var/run:/var/run %s --server %s --token %s%s"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	ca := systemtemplate.CAChecksum()
	if ca != "" {
		ca = " --ca-checksum " + ca
	}

	token, _ := resource.Values["token"].(string)
	if token != "" {
		url := request.URLBuilder.RelativeToRoot("/v3/import/" + token + ".yaml")
		resource.Values["insecureCommand"] = fmt.Sprintf(insecureCommandFormat, url)
		resource.Values["command"] = fmt.Sprintf(commandFormat, url)
		resource.Values["nodeCommand"] = fmt.Sprintf(nodeCommandFormat,
			settings.AgentImage.Get(),
			request.URLBuilder.RelativeToRoot(""),
			token,
			ca)
		resource.Values["token"] = token
		resource.Values["manifestUrl"] = url
	}

}
