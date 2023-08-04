package helm

import (
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Client struct {
	actRun           func(*action.List) ([]*release.Release, error)
	newList          func(*action.Configuration) *action.List
	restClientGetter genericclioptions.RESTClientGetter
}

func NewClient(restClientGetter genericclioptions.RESTClientGetter) *Client {
	return &Client{restClientGetter: restClientGetter, actRun: runAction, newList: action.NewList}
}

func (c *Client) ListReleases(namespace, name string, stateMask action.ListStates) ([]*release.Release, error) {
	helmCfg := &action.Configuration{}
	if err := helmCfg.Init(c.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return nil, err
	}
	l := c.newList(helmCfg)
	l.Filter = "^" + name + "$"
	l.StateMask = stateMask
	return c.actRun(l)
}

func runAction(l *action.List) ([]*release.Release, error) {
	return l.Run()
}
