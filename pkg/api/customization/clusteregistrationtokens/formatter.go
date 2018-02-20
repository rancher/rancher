package clusteregistrationtokens

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	commandFormat         = "kubectl apply -f %s"
	insecureCommandFormat = "curl --insecure -sfL %s | kubectl apply -f -"
	nodeCommandFormat     = "docker run -d --restart=unless-stopped --net=host -v /var/run/docker.sock:/var/run/docker.sock %s --server %s --token %s%s"
)

func CAChecksum() string {
	ca := settings.CACerts.Get()
	if ca != "" {
		if !strings.HasSuffix(ca, "\n") {
			ca += "\n"
		}
		digest := sha256.Sum256([]byte(ca))
		return hex.EncodeToString(digest[:])
	}
	return ""
}

func Formatter(request *types.APIContext, resource *types.RawResource) {
	ca := CAChecksum()
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
