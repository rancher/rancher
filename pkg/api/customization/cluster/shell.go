package clusteregistrationtokens

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ShellLinkHandler struct {
	Proxy          http.Handler
	ClusterManager *clustermanager.Manager
}

func (s *ShellLinkHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	context, err := s.ClusterManager.UserContext(apiContext.ID)
	if err != nil {
		return err
	}

	userManager := context.Management.UserManager

	userID := userManager.GetUser(apiContext)
	token, err := userManager.EnsureToken("kubectl-shell", "Access to kubectl shell in the browser", userID)
	if err != nil {
		return err
	}

	pods, err := context.K8sClient.CoreV1().Pods("cattle-system").List(v1.ListOptions{
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
			vars.Add("command", token)

			path := fmt.Sprintf("/k8s/clusters/%s/api/v1/namespaces/%s/pods/%s/exec", context.ClusterName, "cattle-system", pod.Name)

			req := apiContext.Request
			req.URL.Path = path
			req.URL.RawQuery = vars.Encode()

			s.Proxy.ServeHTTP(apiContext.Response, req)
			return nil
		}
	}

	return httperror.NewAPIError(httperror.NotFound, "failed to find kubectl pod")
}
