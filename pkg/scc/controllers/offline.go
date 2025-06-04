package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

type sccOfflineMode struct {
	log log.StructuredLogger
}

func (s sccOfflineMode) NeedsRegistration(registration *v1.Registration) bool {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) RegisterSystem(registration *v1.Registration) (*v1.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) NeedsActivation(registration *v1.Registration) bool {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Activate(registration *v1.Registration) (*v1.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Keepalive(registration *v1.Registration) (*v1.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (s sccOfflineMode) Deregister() error {
	//TODO implement me
	panic("implement me")
}

var _ SCCHandler = &sccOfflineMode{}
