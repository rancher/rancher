package utils

import (
	"io"
	"sync"

	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/norman/condition"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewClusterLoggingLogger(obj *v3.ClusterLogging, cl v3.ClusterLoggingInterface, cond condition.Cond) (logstream.LoggerStream, io.Closer) {
	logger := logstream.NewLogStream()
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for event := range logger.Stream() {
			if cond.GetMessage(obj) != event.Message {
				if event.Error {
					cond.False(obj)
				}
				cond.Message(obj, event.Message)
				if newObj, err := cl.Update(obj); err == nil {
					obj = newObj
				} else {
					newObj, err = cl.Get(obj.Name, metav1.GetOptions{})
					if err == nil {
						obj = newObj
					}
				}
			}
		}
	}()

	return logger, closerFunc(func() error {
		logger.Close()
		wg.Wait()
		return nil
	})
}

func NewProjectLoggingLogger(obj *v3.ProjectLogging, pl v3.ProjectLoggingInterface, cond condition.Cond) (logstream.LoggerStream, io.Closer) {
	logger := logstream.NewLogStream()
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for event := range logger.Stream() {
			if cond.GetMessage(obj) != event.Message {
				if event.Error {
					cond.False(obj)
				}
				cond.Message(obj, event.Message)
				if newObj, err := pl.Update(obj); err == nil {
					obj = newObj
				} else {
					newObj, err = pl.Controller().Lister().Get(obj.Namespace, obj.Name)
					if err == nil {
						obj = newObj
					}
				}
			}
		}
	}()

	return logger, closerFunc(func() error {
		logger.Close()
		wg.Wait()
		return nil
	})
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
