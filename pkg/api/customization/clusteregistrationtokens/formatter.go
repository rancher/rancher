package clusteregistrationtokens

import (
	"fmt"

	"strings"

	"encoding/base64"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	commandFormat     = "kubectl apply -f %s"
	nodeCommandFormat = "docker run -d --restart=unless-stopped --net=host %s --server %s --token %s hecksum \"%s\""
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	ca := settings.CACerts.Get()
	if ca != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		ca = base64.StdEncoding.EncodeToString([]byte(ca))
		ca = " --ca-checkum " + ca
	}

	token, _ := resource.Values["token"].(string)
	if token != "" {
		url := request.URLBuilder.RelativeToRoot("/" + token + ".yaml")
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
