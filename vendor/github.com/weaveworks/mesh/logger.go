package mesh

// Logger is a simple interface used by mesh to do logging.
type Logger interface {
	Printf(format string, args ...interface{})
}
