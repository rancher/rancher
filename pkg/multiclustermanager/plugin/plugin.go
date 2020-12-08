package main

import (
	"github.com/rancher/rancher/pkg/multiclustermanager/deferred"
	"github.com/rancher/rancher/pkg/multiclustermanager/server"
)

var Factory deferred.Factory = server.NewMCM
