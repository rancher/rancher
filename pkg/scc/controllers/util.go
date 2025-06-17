package controllers

import (
	"github.com/rancher/rancher/pkg/scc/util"
	"time"
)

func minResyncInterval() time.Time {
	now := time.Now()
	if util.VersionIsDevBuild() {
		return now.Add(-devMinCheckin)
	}
	return now.Add(-prodMinCheckin)
}
