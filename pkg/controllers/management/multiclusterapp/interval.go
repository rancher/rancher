package multiclusterapp

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/ticker"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/namespace"
	"github.com/sirupsen/logrus"
)

type IntervalData struct {
	interval   int
	cancelFunc context.CancelFunc
}

var mcAppTickerData map[string]*IntervalData
var mcAppDataLock = sync.Mutex{}

func storeContext(cctx context.Context, mcapp *v3.MultiClusterApp, mcApps v3.MultiClusterAppInterface) {
	if mcapp.Spec.UpgradeStrategy.RollingUpdate == nil {
		return
	}
	set := mcapp.Spec.UpgradeStrategy.RollingUpdate.Interval
	if set == 0 {
		return
	}
	defer mcAppDataLock.Unlock()
	mcAppDataLock.Lock()
	if data, ok := mcAppTickerData[mcapp.Name]; ok {
		if data.interval == set {
			return
		}
		data.interval = set
		data.cancelFunc()
		delete(mcAppTickerData, mcapp.Name)
	}
	ctx, cancel := context.WithCancel(cctx)
	go startTicker(ctx, set, mcApps, mcapp.Name)
	mcAppTickerData[mcapp.Name] = &IntervalData{
		interval:   set,
		cancelFunc: cancel,
	}
}

func deleteContext(mcappName string) {
	mcAppDataLock.Lock()
	if data, ok := mcAppTickerData[mcappName]; ok {
		data.cancelFunc()
		delete(mcAppTickerData, mcappName)
	}
	mcAppDataLock.Unlock()
}

func startTicker(ctx context.Context, set int, mcApps v3.MultiClusterAppInterface, name string) {
	interval := time.Duration(set) * time.Second
	for range ticker.Context(ctx, interval) {
		logrus.Debugf("mcappTicker: interval %v enqueue %s", set, name)
		mcApps.Controller().Enqueue(namespace.GlobalNamespace, name)
	}
}
