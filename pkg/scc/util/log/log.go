package log

import "github.com/sirupsen/logrus"

type StructuredLogger = *logrus.Entry

var rootLog *logrus.Logger

func NewLog() StructuredLogger {
	if rootLog == nil {
		rootLog = logrus.StandardLogger()
	}

	return rootLog.
		WithField("component", "scc-operator")
}

func NewControllerLogger(controllerName string) StructuredLogger {
	baseLogEntry := NewLog().
		WithField("controller", controllerName)

	return baseLogEntry
}
