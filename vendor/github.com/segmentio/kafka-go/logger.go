package kafka

// Logger interface API for log.Logger
type Logger interface {
	Printf(string, ...interface{})
}

// LoggerFunc is a bridge between Logger and any third party logger
// Usage:
//   l := NewLogger() // some logger
//   r := kafka.NewReader(kafka.ReaderConfig{
//     Logger:      kafka.LoggerFunc(l.Infof),
//     ErrorLogger: kafka.LoggerFunc(l.Errorf),
//   })
type LoggerFunc func(string, ...interface{})

func (f LoggerFunc) Printf(msg string, args ...interface{}) { f(msg, args...) }
