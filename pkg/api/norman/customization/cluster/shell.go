package cluster

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/transport"
)

type ShellLinkHandler struct {
	Proxy          http.Handler
	ClusterManager *clustermanager.Manager
}

func (s *ShellLinkHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	context, err := s.ClusterManager.UserContextNoControllers(apiContext.ID)
	if err != nil {
		return err
	}
	userManager := context.Management.UserManager

	userID := userManager.GetUser(apiContext)

	var shellTTL int64
	if minutes, err := strconv.ParseInt(settings.AuthUserSessionTTLMinutes.Get(), 10, 64); err == nil {
		shellTTL = minutes * 60 * 1000 // convert minutes to milliseconds
	}
	input := user.TokenInput{
		TokenName:    "kubectl-shell-" + userID,
		Description:  "Access to kubectl shell in the browser",
		Kind:         "kubectl-shell",
		UserName:     userID,
		AuthProvider: localprovider.Name,
		TTL:          &shellTTL,
		Randomize:    true,
	}
	tokenKey, _, err := userManager.EnsureToken(input)
	if err != nil {
		return err
	}
	cacerts := base64.StdEncoding.EncodeToString([]byte(settings.CACerts.Get()))

	pods, err := context.K8sClient.CoreV1().Pods("cattle-system").List(apiContext.Request.Context(), v1.ListOptions{
		LabelSelector: "app=cattle-agent",
	})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if condition.Cond(corev1.PodReady).IsTrue(&pod) {
			vars := url.Values{}
			vars.Add("container", "agent")
			vars.Add("stdout", "1")
			vars.Add("stdin", "1")
			vars.Add("stderr", "1")
			vars.Add("tty", "1")
			vars.Add("command", "kubectl-shell.sh")
			vars.Add("command", tokenKey)
			vars.Add("command", context.ClusterName)
			vars.Add("command", cacerts)

			path := fmt.Sprintf("/k8s/clusters/%s/api/v1/namespaces/%s/pods/%s/exec", context.ClusterName, "cattle-system", pod.Name)

			req := apiContext.Request
			req.URL.Path = path
			req.URL.RawQuery = vars.Encode()
			// we want to run this as a the system user
			req.Header.Del(transport.ImpersonateUserHeader)
			req.Header.Del(transport.ImpersonateGroupHeader)
			req.Header.Del(transport.ImpersonateUserExtraHeaderPrefix)

			s.Proxy.ServeHTTP(apiContext.Response, req)
			return nil
		}
	}

	return httperror.NewAPIError(httperror.NotFound, "failed to find kubectl pod")
}
