package audit

import (
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type LogWriter struct {
	Level  int
	Output *lumberjack.Logger
}

func NewLogWriter(path string, level, maxAge, maxBackup, maxSize int) *LogWriter {
	if path == "" || level == levelNull {
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
