package helm

import (
	"helm.sh/helm/v4/pkg/action"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Client struct {
	actRun           func(*action.List) ([]*releasev1.Release, error)
	newList          func(*action.Configuration) *action.List
	restClientGetter genericclioptions.RESTClientGetter
}

func NewClient(restClientGetter genericclioptions.RESTClientGetter) *Client {
	return &Client{restClientGetter: restClientGetter, actRun: runAction, newList: action.NewList}
}

func (c *Client) ListReleases(namespace, name string, stateMask action.ListStates) ([]*releasev1.Release, error) {
	helmCfg := &action.Configuration{}
	if err := helmCfg.Init(c.restClientGetter, namespace, ""); err != nil {
		return nil, err
	}
	l := c.newList(helmCfg)
	l.Filter = "^" + name + "$"
	l.StateMask = stateMask
	return c.actRun(l)
}

func runAction(l *action.List) ([]*releasev1.Release, error) {
	results, err := l.Run()
	if err != nil {
		return nil, err
	}

	var rels []*releasev1.Release
	for _, r := range results {
		if rel, ok := r.(*releasev1.Release); ok {
			rels = append(rels, rel)
		}
	}
	return rels, nil
}
