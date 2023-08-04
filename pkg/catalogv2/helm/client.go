package helm

import (
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Client struct {
	restClientGetter genericclioptions.RESTClientGetter
}

func NewClient(restClientGetter genericclioptions.RESTClientGetter) *Client {
	return &Client{restClientGetter: restClientGetter}
}

func (c *Client) ListReleases(namespace, name string, stateMask action.ListStates) ([]*release.Release, error) {
	helmcfg := &action.Configuration{}
	if err := helmcfg.Init(c.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return nil, err
	}

	l := action.NewList(helmcfg)
	l.Filter = "^" + name + "$"
	l.StateMask = stateMask

	return l.Run()
}
