package logstream

type LogEvent struct {
	Error   bool
	Message string
}

type Logger interface {
	Infof(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
}

type LoggerStream interface {
	Logger
	ID() string
	Stream() chan<- LogEvent
	Close()
}

func GetLogStream(id string) LoggerStream {
	return nil
}

func NewLogStream() LoggerStream {
	return nil

}
