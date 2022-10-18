package logger

import (
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

type Logger struct {
	level int
	l     *logrus.Logger
	entry *logrus.Entry
}

func New(level int) logr.Logger {
	return logr.New(&Logger{
		level: level,
		l:     logrus.StandardLogger(),
		entry: logrus.StandardLogger().WithFields(logrus.Fields{}),
	})
}

func (l *Logger) Init(info logr.RuntimeInfo) {

}

func (l *Logger) Enabled(level int) bool {
	return true
}

func (l *Logger) Info(level int, msg string, keysAndValues ...interface{}) {
	l.withValues(keysAndValues...).Debug("[CAPI] " + msg)
}

func (l *Logger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.withValues(keysAndValues...).Errorf("[CAPI] %s: %v", msg, err)
}

func (l *Logger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &Logger{
		level: l.level,
		l:     l.l,
		entry: l.withValues(keysAndValues...),
	}
}

func (l *Logger) withValues(keysAndValues ...interface{}) *logrus.Entry {
	entry := l.entry
	for i := range keysAndValues {
		if i > 0 && i%2 == 0 {
			v := keysAndValues[i]
			k, ok := keysAndValues[i-1].(string)
			if !ok {
				continue
			}
			entry = entry.WithField(k, v)
		}
	}
	return entry
}

func (l *Logger) WithName(name string) logr.LogSink {
	return l.WithValues("name", name)
}
