package log

import (
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/scc/consts"
)

type StructuredLogger = *logrus.Entry

var rootLog *logrus.Logger

type Builder struct {
	Controller   string
	SubComponent string
}

func NewLog() StructuredLogger {
	if rootLog == nil {
		rootLog = logrus.StandardLogger()
	}

	baseLogger := rootLog.
		WithField("component", "scc-operator-deployer")

	if consts.IsDevMode() {
		return baseLogger.WithField("devMode", true)
	}

	return baseLogger
}

func NewControllerLogger(controllerName string) StructuredLogger {
	builder := &Builder{
		Controller: controllerName,
	}

	return builder.ToLogger()
}

func (lb *Builder) ToLogger() StructuredLogger {
	baseLogEntry := NewLog()

	if lb.Controller != "" {
		baseLogEntry = baseLogEntry.WithField("controller", lb.Controller)
	}

	if lb.SubComponent != "" {
		baseLogEntry = baseLogEntry.WithField("subcomponent", lb.SubComponent)
	}

	return baseLogEntry
}
