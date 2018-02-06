package event

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type Logger interface {
	Info(obj runtime.Object, message string)
	Infof(obj runtime.Object, messagefmt string, args ...interface{})
	Error(obj runtime.Object, message string)
	Errorf(obj runtime.Object, messagefmt string, args ...interface{})
}

type logger struct {
	recorder record.EventRecorder
}

func (l *logger) Info(obj runtime.Object, message string) {
	//l.recorder.Event(obj, "Normal", "Message", message)
}

func (l *logger) Infof(obj runtime.Object, messagefmt string, args ...interface{}) {
	//l.recorder.Eventf(obj, "Normal", "Message", messagefmt, args...)
}

func (l *logger) Error(obj runtime.Object, message string) {
	//l.recorder.Event(obj, "Warning", "Message", message)
}

func (l *logger) Errorf(obj runtime.Object, messagefmt string, args ...interface{}) {
	//l.recorder.Eventf(obj, "Warning", "Message", messagefmt, args...)
}

func NewLogger(recorder record.EventRecorder) Logger {
	return &logger{
		recorder: recorder,
	}
}
