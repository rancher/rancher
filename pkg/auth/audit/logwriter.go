package audit

import (
	"context"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type LogWriter struct {
	Level  Level
	Output *lumberjack.Logger
}

func (l *LogWriter) Start(ctx context.Context) {
	if l == nil {
		return
	}
	go func() {
		<-ctx.Done()
		l.Output.Close()
	}()
}

func NewLogWriter(path string, level Level, maxAge, maxBackup, maxSize int) *LogWriter {
	if path == "" || level == LevelNull {
		return nil
	}

	return &LogWriter{
		Level: level,
		Output: &lumberjack.Logger{
			Filename:   path,
			MaxAge:     maxAge,
			MaxBackups: maxBackup,
			MaxSize:    maxSize,
		},
	}
}
