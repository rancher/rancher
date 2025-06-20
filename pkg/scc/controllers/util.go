package controllers

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	"time"
)

func minResyncInterval() time.Time {
	now := time.Now()
	if consts.IsDevMode() {
		return now.Add(-devMinCheckin)
	}
	return now.Add(-prodMinCheckin)
}
