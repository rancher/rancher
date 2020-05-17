package controller

import (
	"context"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/controller"

	errors2 "github.com/pkg/errors"
	"github.com/rancher/norman/metrics"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type HandlerFunc func(key string, obj interface{}) (interface{}, error)

type GenericController interface {
	Informer() cache.SharedIndexInformer
	AddHandler(ctx context.Context, name string, handler HandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type genericController struct {
	controller controller.SharedController
	informer   cache.SharedIndexInformer
	name       string
}

func NewGenericController(name string, controller controller.SharedController) GenericController {
	return &genericController{
		controller: controller,
		informer:   controller.Informer(),
		name:       name,
	}
}

func (g *genericController) Informer() cache.SharedIndexInformer {
	return g.informer
}

func (g *genericController) Enqueue(namespace, name string) {
	g.controller.Enqueue(namespace, name)
}

func (g *genericController) EnqueueAfter(namespace, name string, after time.Duration) {
	g.controller.EnqueueAfter(namespace, name, after)
}

func (g *genericController) AddHandler(ctx context.Context, name string, handler HandlerFunc) {
	g.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		logrus.Tracef("%s calling handler %s %s", g.name, name, key)
		metrics.IncTotalHandlerExecution(g.name, name)
		result, err := handler(key, obj)
		runtimeObject, _ := result.(runtime.Object)
		if err != nil && !ignoreError(err, false) {
			metrics.IncTotalHandlerFailure(g.name, name, key)
		}
		if _, ok := err.(*ForgetError); ok {
			logrus.Tracef("%v %v completed with dropped err: %v", g.name, key, err)
			return runtimeObject, controller.ErrIgnore
		}
		return runtimeObject, err
	}))
}

func ignoreError(err error, checkString bool) bool {
	err = errors2.Cause(err)
	if errors.IsConflict(err) {
		return true
	}
	if err == controller.ErrIgnore {
		return true
	}
	if _, ok := err.(*ForgetError); ok {
		return true
	}
	if checkString {
		return strings.HasSuffix(err.Error(), "please apply your changes to the latest version and try again")
	}
	return false
}
